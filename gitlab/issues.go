package gitlab

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/obcode/glabs/v3/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

type issueReplicationPayload struct {
	Number       int
	Title        string
	Description  string
	WorkItemType string
	ChildIIDs    []int
	// Labels are the source label names. They are recreated in the target project
	// (with the source colour) before the issue is created — see ensureLabels.
	Labels []string
	// AssigneeIDs are the SOURCE assignees. Only those who can actually be assigned
	// in the target are kept; see assignableIn.
	AssigneeIDs []int64
	// Closed replicates the source state. The REST create call has no state field,
	// so a closed issue is created and then closed.
	Closed bool
}

type issueReplicationPlan struct {
	OrderedIssues []int
	ParentByChild map[int]int
}

const issueChildrenGraphQLQuery = `
query IssueChildren($fullPath: ID!, $iid: String!) {
  project(fullPath: $fullPath) {
    issue(iid: $iid) {
      workItem {
        widgets {
          ... on WorkItemWidgetHierarchy {
            children(first: 100) {
              nodes {
                iid
              }
            }
          }
        }
      }
    }
  }
}
`

type issueChildrenGraphQLResponse struct {
	Data struct {
		Project *struct {
			Issue *struct {
				WorkItem *struct {
					Widgets []struct {
						Children struct {
							Nodes []struct {
								IID string `json:"iid"`
							} `json:"nodes"`
						} `json:"children"`
					} `json:"widgets"`
				} `json:"workItem"`
			} `json:"issue"`
		} `json:"project"`
	} `json:"data"`
}

const issueChildrenByParentGraphQLQuery = `
query IssueChildrenByParent($fullPath: ID!, $parentIds: [WorkItemID!], $after: String) {
  namespace(fullPath: $fullPath) {
    workItems(parentIds: $parentIds, first: 100, after: $after) {
      pageInfo {
        endCursor
        hasNextPage
      }
      nodes {
        iid
      }
    }
  }
}
`

type issueChildrenByParentGraphQLResponse struct {
	Data struct {
		Namespace *struct {
			WorkItems struct {
				PageInfo struct {
					EndCursor   string `json:"endCursor"`
					HasNextPage bool   `json:"hasNextPage"`
				} `json:"pageInfo"`
				Nodes []struct {
					IID string `json:"iid"`
				} `json:"nodes"`
			} `json:"workItems"`
		} `json:"namespace"`
	} `json:"data"`
}

// getStartercodeProject extracts the project path from the startercode URL and returns the GitLab project
func (c *Client) getStartercodeProject(assignmentCfg *config.AssignmentConfig) (*gitlab.Project, error) {
	// Parse project path from URL
	// Expected formats:
	// git@gitlab.lrz.de:mpd/startercode/blatt-01.git
	// https://gitlab.lrz.de/mpd/startercode/blatt-01.git
	// https://gitlab.lrz.de/mpd/startercode/blatt-01

	url := assignmentCfg.Startercode.URL

	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	var projectPath string

	// Handle SSH URLs (git@host:path/to/project)
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			projectPath = parts[1]
		}
	} else {
		// Handle HTTPS URLs (https://host/path/to/project)
		re := regexp.MustCompile(`https?://[^/]+/(.+)`)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			projectPath = matches[1]
		}
	}

	if projectPath == "" {
		return nil, fmt.Errorf("could not parse project path from URL: %s", assignmentCfg.Startercode.URL)
	}

	log.Debug().Str("projectPath", projectPath).Msg("loading startercode project for issue replication")

	project, _, err := c.Projects.GetProject(projectPath, nil)
	if err != nil {
		return nil, fmt.Errorf("could not get startercode project: %w", err)
	}

	return project, nil
}

// replicateIssue loads a single issue from source project and creates it in target project
func (c *Client) replicateIssue(sourceProject *gitlab.Project, targetProject *gitlab.Project, issueNumber int, asTask bool) (int, error) {
	issue, err := c.loadIssueForReplication(sourceProject, issueNumber, false)
	if err != nil {
		return 0, err
	}

	// Metadata is best-effort: a label that cannot be created or an assignee who is not a
	// member of the target must not cost us the issue itself.
	assignees, assigneeErr := c.assignableIn(targetProject, issue.AssigneeIDs)
	if assigneeErr != nil {
		log.Debug().Err(assigneeErr).
			Str("targetProject", targetProject.PathWithNamespace).
			Msg("could not determine assignable members; replicating without assignees")
		assignees = nil
	}

	labelIDs, labelErr := c.ensureLabels(sourceProject, targetProject, issue.Labels)
	if labelErr != nil {
		log.Debug().Err(labelErr).
			Str("targetProject", targetProject.PathWithNamespace).
			Msg("could not prepare labels in target project; replicating without them")
		labelIDs = nil
	}

	if asTask {
		targetProjectPath, pathErr := c.getProjectPathForGraphQL(targetProject)
		if pathErr != nil {
			return 0, pathErr
		}

		workItemTypeID, ok := workItemTypeIDForName("task")
		if !ok {
			return 0, fmt.Errorf("task work item type is not available")
		}

		createWorkItemOpts := &gitlab.CreateWorkItemOptions{Title: issue.Title}
		if issue.Description != "" {
			desc := issue.Description
			createWorkItemOpts.Description = &desc
		}
		// Work items want label and assignee IDs, not names — the client turns these into
		// the gid:// forms the GraphQL API expects.
		createWorkItemOpts.LabelIDs = labelIDs
		createWorkItemOpts.AssigneeIDs = assignees

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		createdWI, _, createErr := c.WorkItems.CreateWorkItem(targetProjectPath, workItemTypeID, createWorkItemOpts, gitlab.WithContext(ctx))
		if createErr == nil {
			if issue.Closed {
				closeEvent := gitlab.WorkItemStateEventClose
				if _, _, closeErr := c.WorkItems.UpdateWorkItem(targetProjectPath, createdWI.IID,
					&gitlab.UpdateWorkItemOptions{StateEvent: &closeEvent}, gitlab.WithContext(ctx)); closeErr != nil {
					log.Debug().Err(closeErr).
						Int64("taskIID", createdWI.IID).
						Str("targetProject", targetProject.PathWithNamespace).
						Msg("replicated task could not be closed; it stays open")
				}
			}

			log.Debug().
				Str("issueTitle", issue.Title).
				Str("issueType", "Task").
				Strs("labels", issue.Labels).
				Int("assignees", len(assignees)).
				Bool("closed", issue.Closed).
				Str("targetProject", targetProject.PathWithNamespace).
				Msg("successfully replicated issue via work items GraphQL")

			return int(createdWI.IID), nil
		}

		return 0, fmt.Errorf("could not create task work item in target project %d: %w", targetProject.ID, createErr)
	}

	// Create issue in target project
	createIssueOpts := &gitlab.CreateIssueOptions{
		Title:       gitlab.Ptr(issue.Title),
		Description: gitlab.Ptr(issue.Description),
	}
	// By name here, unlike the work-item path. ensureLabels has already created them with
	// the source colour, so GitLab binds to those rather than inventing new ones.
	// The client sends these as one comma-joined string (LabelOptions.MarshalJSON), which is
	// what the REST API wants — and which is also why a label name containing a comma cannot
	// survive this route. The work-item path above passes ids and is unaffected.
	if len(issue.Labels) > 0 {
		createIssueOpts.Labels = gitlab.Ptr(gitlab.LabelOptions(issue.Labels))
	}
	if len(assignees) > 0 {
		createIssueOpts.AssigneeIDs = &assignees
	}

	created, _, err := c.Issues.CreateIssue(targetProject.ID, createIssueOpts)
	if err != nil {
		return 0, fmt.Errorf("could not create issue %q in target project %d: %w", issue.Title, targetProject.ID, err)
	}

	// The REST create call has no state field, so a closed source issue is created open and
	// closed right after. A failure here is not worth discarding the issue for.
	if issue.Closed {
		if _, _, closeErr := c.Issues.UpdateIssue(targetProject.ID, created.IID,
			&gitlab.UpdateIssueOptions{StateEvent: gitlab.Ptr("close")}); closeErr != nil {
			log.Debug().Err(closeErr).
				Int64("issueIID", created.IID).
				Str("targetProject", targetProject.PathWithNamespace).
				Msg("replicated issue could not be closed; it stays open")
		}
	}

	log.Debug().
		Str("issueTitle", issue.Title).
		Strs("labels", issue.Labels).
		Int("assignees", len(assignees)).
		Bool("closed", issue.Closed).
		Str("targetProject", targetProject.PathWithNamespace).
		Msg("successfully replicated issue")

	return int(created.IID), nil
}

func (c *Client) resolveIssuePlanForReplication(sourceProject *gitlab.Project, issueNumbers []int, includeChildTasks bool) (*issueReplicationPlan, error) {
	ordered := make([]int, 0, len(issueNumbers))
	seen := make(map[int]struct{}, len(issueNumbers))
	parentByChild := make(map[int]int)

	queue := make([]int, 0, len(issueNumbers))
	queue = append(queue, issueNumbers...)

	for len(queue) > 0 {
		issueNumber := queue[0]
		queue = queue[1:]

		if _, exists := seen[issueNumber]; exists {
			continue
		}

		seen[issueNumber] = struct{}{}
		ordered = append(ordered, issueNumber)

		if !includeChildTasks {
			continue
		}

		issue, err := c.loadIssueForReplication(sourceProject, issueNumber, true)
		if err != nil {
			return nil, err
		}

		for _, child := range issue.ChildIIDs {
			if _, exists := parentByChild[child]; !exists {
				parentByChild[child] = issueNumber
			}

			if _, exists := seen[child]; exists {
				continue
			}
			queue = append(queue, child)
		}
	}

	return &issueReplicationPlan{OrderedIssues: ordered, ParentByChild: parentByChild}, nil
}

func (c *Client) resolveIssueNumbersForReplication(sourceProject *gitlab.Project, issueNumbers []int, includeChildTasks bool) ([]int, error) {
	plan, err := c.resolveIssuePlanForReplication(sourceProject, issueNumbers, includeChildTasks)
	if err != nil {
		return nil, err
	}

	return plan.OrderedIssues, nil
}

func (c *Client) loadIssueForReplication(sourceProject *gitlab.Project, issueNumber int, includeChildTasks bool) (*issueReplicationPayload, error) {
	projectPath, err := c.getProjectPathForGraphQL(sourceProject)
	if err != nil {
		return nil, err
	}

	issue, _, err := c.Issues.GetIssue(sourceProject.ID, int64(issueNumber), nil)
	if err != nil {
		return nil, fmt.Errorf("could not get issue from startercode project %s with number %d: %w", projectPath, issueNumber, err)
	}

	result := &issueReplicationPayload{
		Number:       issueNumber,
		Title:        issue.Title,
		Description:  issue.Description,
		WorkItemType: "Issue",
		Labels:       append([]string(nil), issue.Labels...),
		Closed:       issue.State == "closed",
	}

	for _, assignee := range issue.Assignees {
		if assignee != nil {
			result.AssigneeIDs = append(result.AssigneeIDs, assignee.ID)
		}
	}

	if !includeChildTasks {
		return result, nil
	}

	childIIDs, childErr := c.listChildIIDsByParentLookup(projectPath, issue.ID)
	if childErr == nil {
		result.ChildIIDs = append(result.ChildIIDs, childIIDs...)
		if len(childIIDs) > 0 {
			log.Debug().Int("issue", issueNumber).Ints("childIIDs", childIIDs).Msg("resolved child tasks via parent lookup")
			return result, nil
		}
	}

	childIIDs, childErr = c.listChildIIDsByIssueGraphQL(projectPath, issueNumber)
	if childErr != nil {
		log.Debug().Err(childErr).Int("issue", issueNumber).Str("project", projectPath).Msg("could not resolve child tasks from fallback issue query; continuing without children")
		return result, nil
	}
	result.ChildIIDs = append(result.ChildIIDs, childIIDs...)
	if len(childIIDs) > 0 {
		log.Debug().Int("issue", issueNumber).Ints("childIIDs", childIIDs).Msg("resolved child tasks via hierarchy widget query")
	}

	return result, nil
}

func (c *Client) listChildIIDsByParentLookup(projectPath string, parentIssueID int64) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	parentGID := fmt.Sprintf("gid://gitlab/WorkItem/%d", parentIssueID)
	after := ""
	childIIDs := make([]int, 0)

	for {
		var response issueChildrenByParentGraphQLResponse
		variables := map[string]any{
			"fullPath":  projectPath,
			"parentIds": []string{parentGID},
		}
		if after != "" {
			variables["after"] = after
		}

		_, err := c.GraphQL.Do(gitlab.GraphQLQuery{Query: issueChildrenByParentGraphQLQuery, Variables: variables}, &response, gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}

		if response.Data.Namespace == nil {
			return childIIDs, nil
		}

		for _, node := range response.Data.Namespace.WorkItems.Nodes {
			var iid int
			if _, err := fmt.Sscanf(node.IID, "%d", &iid); err != nil {
				return nil, fmt.Errorf("invalid child iid %q", node.IID)
			}
			childIIDs = append(childIIDs, iid)
		}

		if !response.Data.Namespace.WorkItems.PageInfo.HasNextPage {
			break
		}

		after = response.Data.Namespace.WorkItems.PageInfo.EndCursor
		if after == "" {
			break
		}
	}

	return childIIDs, nil
}

func (c *Client) listChildIIDsByIssueGraphQL(projectPath string, issueNumber int) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var response issueChildrenGraphQLResponse
	_, err := c.GraphQL.Do(gitlab.GraphQLQuery{
		Query: issueChildrenGraphQLQuery,
		Variables: map[string]any{
			"fullPath": projectPath,
			"iid":      fmt.Sprintf("%d", issueNumber),
		},
	}, &response, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	if response.Data.Project == nil || response.Data.Project.Issue == nil || response.Data.Project.Issue.WorkItem == nil {
		return nil, nil
	}

	childIIDs := make([]int, 0)
	for _, widget := range response.Data.Project.Issue.WorkItem.Widgets {
		for _, node := range widget.Children.Nodes {
			var iid int
			_, scanErr := fmt.Sscanf(node.IID, "%d", &iid)
			if scanErr != nil {
				return nil, fmt.Errorf("invalid child iid %q", node.IID)
			}
			childIIDs = append(childIIDs, iid)
		}
	}

	return childIIDs, nil
}

func workItemTypeIDForName(typeName string) (gitlab.WorkItemTypeID, bool) {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "issue":
		return gitlab.WorkItemTypeIssue, true
	case "task":
		return gitlab.WorkItemTypeTask, true
	case "incident":
		return gitlab.WorkItemTypeIncident, true
	case "test case", "testcase":
		return gitlab.WorkItemTypeTestCase, true
	case "requirement":
		return gitlab.WorkItemTypeRequirement, true
	case "objective":
		return gitlab.WorkItemTypeObjective, true
	case "key result", "keyresult":
		return gitlab.WorkItemTypeKeyResult, true
	case "epic":
		return gitlab.WorkItemTypeEpic, true
	case "ticket":
		return gitlab.WorkItemTypeTicket, true
	default:
		return "", false
	}
}

func (c *Client) attachChildTaskToParent(targetProject *gitlab.Project, parentIssueIID int, childIssueIID int) error {
	projectPath, err := c.getProjectPathForGraphQL(targetProject)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	parentWI, _, err := c.WorkItems.GetWorkItem(projectPath, int64(parentIssueIID), gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("could not load target parent issue #%d as work item: %w", parentIssueIID, err)
	}

	parentID := parentWI.ID
	_, _, err = c.WorkItems.UpdateWorkItem(projectPath, int64(childIssueIID), &gitlab.UpdateWorkItemOptions{ParentID: &parentID}, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("could not attach child issue #%d to parent issue #%d in %s: %w", childIssueIID, parentIssueIID, projectPath, err)
	}

	return nil
}

func (c *Client) getProjectPathForGraphQL(project *gitlab.Project) (string, error) {
	if project == nil {
		return "", fmt.Errorf("source project is nil")
	}

	if project.PathWithNamespace != "" {
		return project.PathWithNamespace, nil
	}

	if project.ID == 0 {
		return "", fmt.Errorf("source project has no path and no id")
	}

	reloaded, _, err := c.Projects.GetProject(project.ID, nil)
	if err != nil {
		return "", fmt.Errorf("could not load source project path for GraphQL: %w", err)
	}

	if reloaded.PathWithNamespace == "" {
		return "", fmt.Errorf("source project path is empty for project %d", project.ID)
	}

	return reloaded.PathWithNamespace, nil
}

// --- metadata carried along with a replicated issue -------------------------------------
//
// Labels, assignees and the open/closed state are replicated "where possible" (issue #151).
// Two of the four fields asked for there are deliberately NOT replicated, because they
// cannot be: both iterations and milestones belong to a group's own cadence, and a generated
// student project lives under a different group, where the source id does not exist.
//
// The caches below live on the Client and carry no lock. A Client is never shared across
// goroutines — the web server builds one per request (see the `rep` field) and the CLI runs
// generate sequentially.

// projectMemberIDs returns everyone who can be assigned in the project, inherited group
// members included. Cached, because issue replication asks once per issue but the answer only
// changes per project.
func (c *Client) projectMemberIDs(project *gitlab.Project) (map[int64]struct{}, error) {
	if ids, ok := c.memberIDs[project.ID]; ok {
		return ids, nil
	}

	ids := make(map[int64]struct{})
	opts := &gitlab.ListProjectMembersOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	for {
		members, resp, err := c.ProjectMembers.ListAllProjectMembers(project.ID, opts)
		if err != nil {
			return nil, fmt.Errorf("could not list members of project %d: %w", project.ID, err)
		}
		for _, member := range members {
			if member != nil {
				ids[member.ID] = struct{}{}
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if c.memberIDs == nil {
		c.memberIDs = make(map[int64]map[int64]struct{})
	}
	c.memberIDs[project.ID] = ids

	return ids, nil
}

// assignableIn keeps only those source assignees who are members of the target project.
// GitLab rejects the whole create call for a non-member assignee, and the usual case here is
// the lecturer who owns the startercode issue and has no business being assigned in every
// student's repository.
func (c *Client) assignableIn(project *gitlab.Project, sourceIDs []int64) ([]int64, error) {
	if len(sourceIDs) == 0 {
		return nil, nil
	}

	members, err := c.projectMemberIDs(project)
	if err != nil {
		return nil, err
	}

	kept := make([]int64, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		if _, ok := members[id]; ok {
			kept = append(kept, id)
		}
	}

	return kept, nil
}

// projectLabels indexes a project's labels by name. Cached per project.
func (c *Client) projectLabels(project *gitlab.Project) (map[string]*gitlab.Label, error) {
	if labels, ok := c.labelsByName[project.ID]; ok {
		return labels, nil
	}

	labels := make(map[string]*gitlab.Label)
	opts := &gitlab.ListLabelsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	for {
		page, resp, err := c.Labels.ListLabels(project.ID, opts)
		if err != nil {
			return nil, fmt.Errorf("could not list labels of project %d: %w", project.ID, err)
		}
		for _, label := range page {
			if label != nil {
				labels[label.Name] = label
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if c.labelsByName == nil {
		c.labelsByName = make(map[int64]map[string]*gitlab.Label)
	}
	c.labelsByName[project.ID] = labels

	return labels, nil
}

// ensureLabels makes sure every name exists as a label in the target project and returns the
// resulting ids. Creating them up front rather than letting GitLab create them implicitly
// while creating the issue is what preserves colour and description: the implicit path invents
// a colour, so the same label would look different in every student repository.
//
// A label that cannot be created is skipped rather than failing the replication — losing a
// label is a smaller loss than losing the issue.
func (c *Client) ensureLabels(sourceProject, targetProject *gitlab.Project, names []string) ([]int64, error) {
	if len(names) == 0 {
		return nil, nil
	}

	targetLabels, err := c.projectLabels(targetProject)
	if err != nil {
		return nil, err
	}

	// Only needed when something is actually missing, so it stays lazy.
	var sourceLabels map[string]*gitlab.Label

	ids := make([]int64, 0, len(names))
	for _, name := range names {
		if existing, ok := targetLabels[name]; ok {
			ids = append(ids, existing.ID)
			continue
		}

		if sourceLabels == nil {
			sourceLabels, err = c.projectLabels(sourceProject)
			if err != nil {
				return nil, err
			}
		}

		opts := &gitlab.CreateLabelOptions{Name: gitlab.Ptr(name)}
		if source, ok := sourceLabels[name]; ok {
			opts.Color = gitlab.Ptr(source.Color)
			if source.Description != "" {
				opts.Description = gitlab.Ptr(source.Description)
			}
		} else {
			// The source label is gone (or lives on an ancestor group we cannot read).
			// GitLab requires a colour, so pick a neutral one rather than giving up.
			opts.Color = gitlab.Ptr("#6699cc")
		}

		created, _, createErr := c.Labels.CreateLabel(targetProject.ID, opts)
		if createErr != nil {
			log.Debug().Err(createErr).
				Str("label", name).
				Str("targetProject", targetProject.PathWithNamespace).
				Msg("could not create label in target project; replicating the issue without it")
			continue
		}

		targetLabels[name] = created
		ids = append(ids, created.ID)
	}

	return ids, nil
}
