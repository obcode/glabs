package gitlab

import (
	"strconv"

	"github.com/obcode/glabs/v3/config"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// ExistingRepoNames lists the names of the projects that actually exist in an
// assignment's GitLab group — one paginated group listing rather than a GetProject
// per student, so a whole-course overview stays cheap (one call per assignment, not
// per repository). The caller matches these against RepoTargets to see which repos
// have been generated. A missing group (assignment never generated) surfaces as an
// error the caller can treat as "nothing generated".
func (c *Client) ExistingRepoNames(assignmentCfg *config.AssignmentConfig) (map[string]bool, error) {
	groupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		return nil, err
	}

	names := make(map[string]bool)
	opts := &gitlab.ListGroupProjectsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	for {
		projects, resp, err := c.Groups.ListGroupProjects(groupID, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			names[p.Name] = true
		}
		next := nextPageFromHeader(resp)
		if next == 0 {
			break
		}
		opts.Page = int64(next)
	}
	return names, nil
}

// nextPageFromHeader reads GitLab's X-Next-Page pagination header (0 = last page).
func nextPageFromHeader(resp *gitlab.Response) int {
	if resp == nil {
		return 0
	}
	vals := resp.Header["X-Next-Page"]
	if len(vals) == 0 || vals[0] == "" {
		return 0
	}
	n, err := strconv.Atoi(vals[0])
	if err != nil {
		return 0
	}
	return n
}
