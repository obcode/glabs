package gitlab

import (
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v3/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) Setaccess(assignmentCfg *config.AssignmentConfig) error {
	_, err := c.getGroupID(assignmentCfg)
	if err != nil {
		return fmt.Errorf("GitLab group for assignment does not exist, please create the group %s", assignmentCfg.URL)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.setaccessPerGroup(assignmentCfg)
	case config.PerStudent:
		c.setaccessPerStudent(assignmentCfg)
	default:
		return fmt.Errorf("it is only possible to set access levels for students or groups, not for %v", per)
	}
	return nil
}

func (c *Client) setaccess(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, members []*config.Student) {
	for _, student := range members {
		task := c.rep.Task(aurora.Sprintf(aurora.Cyan(" ↪ adding member %s to %s as %s"),
			aurora.Yellow(student.Raw),
			aurora.Magenta(project.Name),
			aurora.Magenta(assignmentCfg.AccessLevel.String()),
		))

		userID, err := c.getUserID(student)
		if err != nil {
			if student.Email != nil {
				log.Debug().Str("email", *student.Email).Msg("inviting via email")
				info, err := c.inviteByEmail(assignmentCfg, project.ID, *student.Email)
				if err != nil {
					task.Fail(fmt.Sprintf("%v", err))
				} else {
					task.Done(aurora.Sprintf(aurora.Green(info)))
				}
				continue
			}
			task.Fail(fmt.Sprintf("cannot get user id: %v", err))
			continue
		}

		info, err := c.addMember(assignmentCfg, project.ID, userID)
		if err != nil {
			log.Debug().Err(err).
				Int64("projectID", project.ID).
				Int64("userID", userID).
				Str("student", student.Raw).
				Str("course", assignmentCfg.Course).
				Str("assignment", assignmentCfg.Name).
				Msg("error while adding member")

			task.Fail(fmt.Sprintf("cannot add user %v: %v", student, err))
			continue
		}

		task.Done(aurora.Sprintf(aurora.Green(info)))
	}
}

func (c *Client) inviteByEmail(cfg *config.AssignmentConfig, projectID int64, email string) (string, error) {
	m := &gitlab.InvitesOptions{
		Email:       &email,
		AccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(cfg.AccessLevel)),
	}
	resp, _, err := c.Invites.ProjectInvites(projectID, m)
	if err != nil {
		return "", err
	}
	if resp.Status != "success" {
		return "", fmt.Errorf("inviting user %s failed with %s", email, resp.Message[email])
	}
	return fmt.Sprintf("successfully invited user %s", email), nil
}

func (c *Client) setaccessPerStudent(assignmentCfg *config.AssignmentConfig) {
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
			c.rep.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		c.setaccess(assignmentCfg, project, []*config.Student{student})
	}
}

func (c *Client) setaccessPerGroup(assignmentCfg *config.AssignmentConfig) {
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
			c.rep.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		c.setaccess(assignmentCfg, project, grp.Members)
	}
}
