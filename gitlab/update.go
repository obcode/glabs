package gitlab

import (
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/git"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) Update(assignmentCfg *config.AssignmentConfig) error {
	_, err := c.getGroupID(assignmentCfg)
	if err != nil {
		return fmt.Errorf("GitLab group for assignment does not exist, please create the group %s", assignmentCfg.URL)
	}

	var starterrepo *git.SourceRepo

	if assignmentCfg.Startercode != nil {
		starterrepo, err = git.PrepareSourceRepo(c.rep, c.gitAuth(), c.committer,
			assignmentCfg.Startercode.URL,
			assignmentCfg.Startercode.FromBranch,
			assignmentCfg.Startercode.Template,
			assignmentCfg.Startercode.TemplateMessage,
		)
		if err != nil {
			return err
		}
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.updatePerGroup(assignmentCfg, starterrepo)
	case config.PerStudent:
		c.updatePerStudent(assignmentCfg, starterrepo)
	default:
		return fmt.Errorf("it is only possible to update for students or groups, not for %v", per)
	}
	return nil
}

func (c *Client) update(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, starterrepo *git.SourceRepo) {
	if starterrepo == nil {
		return
	}

	task := c.rep.Task(aurora.Sprintf(aurora.Cyan(" updating project %s at %s"),
		aurora.Yellow(project.Name),
		aurora.Magenta(assignmentCfg.URL+"/"+project.Name),
	))

	if err := c.pushStartercode(assignmentCfg, starterrepo, project); err != nil {
		task.Fail(fmt.Sprintf("problem: %v", err))
		return
	}
	task.Done("")
}

func (c *Client) updatePerStudent(assignmentCfg *config.AssignmentConfig, starterrepo *git.SourceRepo) {
	if len(assignmentCfg.Students) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no students found")
		return
	}

	for _, student := range assignmentCfg.Students {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForStudent(student))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			// Skip a missing repo (e.g. one never generated) instead of aborting the
			// whole run — one 404 must not stop every remaining repo (cf. #109).
			c.rep.Printf("cannot update project %s failed with %s", projectname, err)
			continue
		}
		c.update(assignmentCfg, project, starterrepo)
	}
}

func (c *Client) updatePerGroup(assignmentCfg *config.AssignmentConfig, starterrepo *git.SourceRepo) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForGroup(grp))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			// Skip a missing repo (e.g. one never generated) instead of aborting the
			// whole run — one 404 must not stop every remaining repo (cf. #109).
			c.rep.Printf("cannot update project %s failed with %s", projectname, err)
			continue
		}
		c.update(assignmentCfg, project, starterrepo)
	}
}
