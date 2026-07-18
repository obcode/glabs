package gitlab

import (
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/reporter"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) Archive(assignmentCfg *config.AssignmentConfig, unarchive bool) error {
	_, err := c.getGroupID(assignmentCfg)
	if err != nil {
		return fmt.Errorf("GitLab group for assignment does not exist, please create the group %s", assignmentCfg.URL)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.archivePerGroup(assignmentCfg, unarchive)
	case config.PerStudent:
		c.archivePerStudent(assignmentCfg, unarchive)
	default:
		return fmt.Errorf("it is only possible to archive projects for students or groups, not for %v", per)
	}
	return nil
}

func (c *Client) archivePerStudent(assignmentCfg *config.AssignmentConfig, unarchive bool) {
	if len(assignmentCfg.Students) == 0 {
		c.rep.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForStudent(student))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			c.rep.Printf("cannot archive project %s failed with %s", projectname, err)
			continue
		}
		if err := c.archive(assignmentCfg, project, true, unarchive); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot archive project")
		}
	}
}

func (c *Client) archivePerGroup(assignmentCfg *config.AssignmentConfig, unarchive bool) {
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
			c.rep.Printf("cannot archive project %s failed with %s", projectname, err)
			continue
		}
		if err := c.archive(assignmentCfg, project, true, unarchive); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot archive project")
		}
	}
}

// archive (un)archives a project. spin reports its own progress; callers that
// already run a task pass false to avoid a nested spinner.
func (c *Client) archive(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, spin bool, unarchive bool) error {
	task := reporter.NopTask()
	if spin {
		un := ""
		if unarchive {
			un = "un"
		}
		task = c.rep.Task(aurora.Sprintf(aurora.Cyan(" %sarchiving project %s at %s"),
			un,
			aurora.Yellow(project.Name),
			aurora.Magenta(assignmentCfg.URL+"/"+project.Name),
		))
	}

	var err error
	if unarchive {
		_, _, err = c.Projects.UnarchiveProject(project.ID)
	} else {
		_, _, err = c.Projects.ArchiveProject(project.ID)
	}
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.HTTPURLToRepo).
			Msg("cannot archive project")
		task.Fail("")
		verb := "archive"
		if unarchive {
			verb = "unarchive"
		}
		return fmt.Errorf("error while trying to %s project: %w", verb, err)
	}

	task.Done(aurora.Sprintf(aurora.Green("ok")))
	return nil
}
