package gitlab

import (
	"fmt"
	"os"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) Delete(assignmentCfg *config.AssignmentConfig) {
	assignmentGitLabGroupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		os.Exit(1)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.deletePerGroup(assignmentCfg, assignmentGitLabGroupID)
	case config.PerStudent:
		c.deletePerStudent(assignmentCfg, assignmentGitLabGroupID)
	default:
		fmt.Printf("it is only possible to delete projects for students or groups, not for %v", per)
		os.Exit(1)
	}
}

func (c *Client) deletePerStudent(assignmentCfg *config.AssignmentConfig, assignmentGroupID int64) {
	if len(assignmentCfg.Students) == 0 {
		fmt.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		c.delete(assignmentGroupID, assignmentCfg.RepoNameForStudent(student))
	}
}

func (c *Client) deletePerGroup(assignmentCfg *config.AssignmentConfig, assignmentGroupID int64) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		c.delete(assignmentGroupID, assignmentCfg.RepoNameForGroup(grp))
	}
}

func (c *Client) delete(gid int64, name string) {
	projects, _, err := c.Search.ProjectsByGroup(gid, name, &gitlab.SearchOptions{})
	if err != nil {
		log.Error().Str("project", name).Msg("searching for projects failed")
		return
	}
	if len(projects) == 0 {
		return
	}
	for _, project := range projects {
		if project.Name == name {
			log.Info().Str("project", project.Name).Msg("deleting project")
			_, err = c.Projects.DeleteProject(project.ID, &gitlab.DeleteProjectOptions{})
			if err != nil {
				log.Error().Str("project", name).Msg("deleting project failed")
				return
			}
			break
		}
	}

}
