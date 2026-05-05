package gitlab

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/obcode/glabs/v2/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

type issueReplicationPayload struct {
	Number       int
	Title        string
	Description  string
	WorkItemType string
	ChildIIDs    []int
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

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		createdWI, _, createErr := c.WorkItems.CreateWorkItem(targetProjectPath, workItemTypeID, createWorkItemOpts, gitlab.WithContext(ctx))
		if createErr == nil {
			log.Debug().
				Str("issueTitle", issue.Title).
				Str("issueType", "Task").
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

	created, _, err := c.Issues.CreateIssue(targetProject.ID, createIssueOpts)
	if err != nil {
		return 0, fmt.Errorf("could not create issue %q in target project %d: %w", issue.Title, targetProject.ID, err)
	}

	log.Debug().
		Str("issueTitle", issue.Title).
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
