package gitlab

import (
	"fmt"

	"github.com/obcode/glabs/v3/config"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) Delete(assignmentCfg *config.AssignmentConfig) error {
	assignmentGitLabGroupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		return fmt.Errorf("GitLab group for assignment does not exist, please create the group %s", assignmentCfg.URL)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.deletePerGroup(assignmentCfg, assignmentGitLabGroupID)
	case config.PerStudent:
		c.deletePerStudent(assignmentCfg, assignmentGitLabGroupID)
	default:
		return fmt.Errorf("it is only possible to delete projects for students or groups, not for %v", per)
	}
	return nil
}

func (c *Client) deletePerStudent(assignmentCfg *config.AssignmentConfig, assignmentGroupID int64) {
	if len(assignmentCfg.Students) == 0 {
		c.rep.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		c.delete(assignmentGroupID, assignmentCfg.RepoNameForStudent(student))
	}
}

func (c *Client) deletePerGroup(assignmentCfg *config.AssignmentConfig, assignmentGroupID int64) {
	if len(assignmentCfg.Groups) == 0 {
		c.rep.Println("no groups in config for assignment found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		c.delete(assignmentGroupID, assignmentCfg.RepoNameForGroup(grp))
	}
}

func (c *Client) delete(gid int64, name string) {
	projects, _, err := c.Search.ProjectsByGroup(gid, name, &gitlab.SearchOptions{})
	if err != nil {
		c.rep.Printf("searching for project %s failed with %s", name, err)
		return
	}
	if len(projects) == 0 {
		c.rep.Printf("no project %s to delete (skipped)", name)
		return
	}
	for _, project := range projects {
		if project.Name == name {
			task := c.rep.Task(fmt.Sprintf(" deleting project %s", project.Name))
			removedTags, err := c.deleteRegistryTags(project.ID)
			if err != nil {
				task.Fail(fmt.Sprintf("deleting container registry tags of project %s failed: %v", name, err))
				return
			}
			_, err = c.Projects.DeleteProject(project.ID, &gitlab.DeleteProjectOptions{})
			if err != nil {
				task.Fail(fmt.Sprintf("deleting project %s failed: %v", name, err))
				return
			}
			if removedTags > 0 {
				task.Done(fmt.Sprintf("%d container registry tag(s) removed", removedTags))
			} else {
				task.Done("")
			}
			break
		}
	}
}

// deleteRegistryTags removes every container registry tag of a project and
// returns how many it removed. GitLab refuses to delete a project that still
// has tags ("Cannot rename or delete project because it contains container
// registry tags"). Tags are deleted one by one because that endpoint works
// synchronously; the bulk and repository endpoints only schedule a background
// job, so the project delete right after would still be refused.
//
// If the registry repositories cannot be listed (registry disabled for the
// project or on the instance), there is nothing we can clean up; the project
// delete is attempted anyway and reports its own error if tags remain.
func (c *Client) deleteRegistryTags(pid int64) (int, error) {
	var repos []*gitlab.RegistryRepository
	repoOpts := &gitlab.ListProjectRegistryRepositoriesOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	for {
		page, resp, err := c.ContainerRegistry.ListProjectRegistryRepositories(pid, repoOpts)
		if err != nil {
			return 0, nil //nolint:nilerr // no registry to clean up, see above
		}
		repos = append(repos, page...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		repoOpts.Page = resp.NextPage
	}

	removed := 0
	for _, repo := range repos {
		// Collect all tags before deleting any, so deletions don't shift the pages.
		var tags []*gitlab.RegistryRepositoryTag
		tagOpts := &gitlab.ListRegistryRepositoryTagsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
		for {
			page, resp, err := c.ContainerRegistry.ListRegistryRepositoryTags(pid, repo.ID, tagOpts)
			if err != nil {
				return removed, fmt.Errorf("listing tags of registry repository %s: %w", repo.Path, err)
			}
			tags = append(tags, page...)
			if resp == nil || resp.NextPage == 0 {
				break
			}
			tagOpts.Page = resp.NextPage
		}

		for _, tag := range tags {
			if _, err := c.ContainerRegistry.DeleteRegistryRepositoryTag(pid, repo.ID, tag.Name); err != nil {
				return removed, fmt.Errorf("deleting tag %s:%s: %w", repo.Path, tag.Name, err)
			}
			removed++
		}
	}
	return removed, nil
}
