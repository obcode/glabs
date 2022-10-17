package gitlab

import (
	"fmt"
	"os"
	"strings"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) Setaccess(assignmentCfg *config.AssignmentConfig) {
	assignmentGitLabGroupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		os.Exit(1)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.setaccessPerGroup(assignmentCfg, assignmentGitLabGroupID)
	case config.PerStudent:
		c.setaccessPerStudent(assignmentCfg, assignmentGitLabGroupID)
	default:
		fmt.Printf("it is only possible to set access levels for students oder groups, not for %v", per)
		os.Exit(1)
	}
}

func (c *Client) setaccess(assignmentCfg *config.AssignmentConfig,
	project *gitlab.Project, members []string, cfgP *yacspin.Config) {
	var cfg yacspin.Config
	if cfgP == nil {
		cfg = yacspin.Config{
			Frequency: 100 * time.Millisecond,
			CharSet:   yacspin.CharSets[69],
			Suffix: aurora.Sprintf(aurora.Cyan(" setting access for project %s at %s"),
				aurora.Yellow(project.Name),
				aurora.Magenta(assignmentCfg.URL+"/"+project.Name),
			),
			SuffixAutoColon:   true,
			StopCharacter:     "✓",
			StopColors:        []string{"fgGreen"},
			StopFailMessage:   "error",
			StopFailCharacter: "✗",
			StopFailColors:    []string{"fgRed"},
		}
		spinner, err := yacspin.New(cfg)
		if err != nil {
			log.Debug().Err(err).Msg("cannot create spinner")
		}
		err = spinner.Start()
		if err != nil {
			log.Debug().Err(err).Msg("cannot start spinner")
		}
		err = spinner.Stop()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
	} else {
		cfg = *cfgP
	}

	for _, student := range members {
		cfg.Suffix = aurora.Sprintf(aurora.Cyan(" ↪ adding member %s to %s as %s"),
			aurora.Yellow(student),
			aurora.Magenta(project.Name),
			aurora.Magenta(assignmentCfg.AccessLevel.String()),
		)
		spinner, err := yacspin.New(cfg)
		if err != nil {
			log.Debug().Err(err).Msg("cannot create spinner")
		}
		err = spinner.Start()
		if err != nil {
			log.Debug().Err(err).Msg("cannot start spinner")
		}

		var userID int

		if strings.HasPrefix(student, "id:") {
			asRunes := []rune(student)
			userIDStr := string(asRunes[3:])
			userID, err = strconv.Atoi(userIDStr)
			if err != nil {
				log.Debug().Err(err).Msg("cannot interpret specified user id as numeric id")
			}
		} else {
			userID, err = c.getUserID(student)
		}

		if err != nil {
			if strings.Contains(student, "@") {
				info, err := c.inviteByEmail(assignmentCfg, project.ID, student)
				if err != nil {
					spinner.StopFailMessage(fmt.Sprintf("%v", err))

					err := spinner.StopFail()
					if err != nil {
						log.Debug().Err(err).Msg("cannot stop spinner")
					}
				} else {

					spinner.StopMessage(aurora.Sprintf(aurora.Green(info)))
					err = spinner.Stop()
					if err != nil {
						log.Debug().Err(err).Msg("cannot stop spinner")
					}
				}
				continue
			} else {
				spinner.StopFailMessage(fmt.Sprintf("cannot get user id: %v", err))

				err := spinner.StopFail()
				if err != nil {
					log.Debug().Err(err).Msg("cannot stop spinner")
				}
			}
			continue
		}

		info, err := c.addMember(assignmentCfg, project.ID, userID)
		if err != nil {
			log.Debug().Err(err).
				Int("projectID", project.ID).
				Int("userID", userID).
				Str("student", student).
				Str("course", assignmentCfg.Course).
				Str("assignment", assignmentCfg.Name).
				Msg("error while adding member")

			spinner.StopFailMessage(fmt.Sprintf("cannot add user %s: %v", student, err))

			err := spinner.StopFail()
			if err != nil {
				log.Debug().Err(err).Msg("cannot stop spinner")
			}
			continue
		}

		spinner.StopMessage(aurora.Sprintf(aurora.Green(info)))
		err = spinner.Stop()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
	}
}

func (c *Client) inviteByEmail(cfg *config.AssignmentConfig, projectID int, email string) (string, error) {
	m := &gitlab.InvitesOptions{
		Email:       &email,
		AccessLevel: gitlab.AccessLevel(gitlab.AccessLevelValue(cfg.AccessLevel)),
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

func (c *Client) setaccessPerStudent(assignmentCfg *config.AssignmentConfig, assignmentGroupID int) {
	if len(assignmentCfg.Students) == 0 {
		fmt.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		name := assignmentCfg.Name + "-" + assignmentCfg.EscapeUserName(student)
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, name)
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			fmt.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		c.setaccess(assignmentCfg, project, []string{student}, nil)
	}
}

func (c *Client) setaccessPerGroup(assignmentCfg *config.AssignmentConfig, assignmentGroupID int) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		projectname := fmt.Sprintf("%s/%s-%s", assignmentCfg.Path, assignmentCfg.Name, grp.Name)
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			fmt.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		c.setaccess(assignmentCfg, project, grp.Members, nil)
	}
}
