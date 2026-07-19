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
			_, err = c.Projects.DeleteProject(project.ID, &gitlab.DeleteProjectOptions{})
			if err != nil {
				task.Fail(fmt.Sprintf("deleting project %s failed: %v", name, err))
				return
			}
			task.Done("")
			break
		}
	}
}
