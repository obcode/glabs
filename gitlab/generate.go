package gitlab

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/git"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) Generate(assignmentCfg *config.AssignmentConfig) {
	assignmentGitLabGroupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		os.Exit(1)
	}

	var starterrepo *git.Starterrepo

	if assignmentCfg.Startercode != nil {
		starterrepo, err = git.PrepareStartercodeRepo(assignmentCfg)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.generatePerGroup(assignmentCfg, assignmentGitLabGroupID, starterrepo)
	case config.PerStudent:
		c.generatePerStudent(assignmentCfg, assignmentGitLabGroupID, starterrepo)
	default:
		fmt.Printf("it is only possible to generate for students oder groups, not for %v", per)
		os.Exit(1)
	}
}

func (c *Client) generate(assignmentCfg *config.AssignmentConfig, assignmentGroupID int,
	projectname string, members []string, starterrepo *git.Starterrepo) {

	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" generating project %s at %s"),
			aurora.Yellow(projectname),
			aurora.Magenta(assignmentCfg.URL+"/"+projectname),
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

	spinner.Message("generating project on host")
	project, generated, err := c.generateProject(assignmentCfg, projectname, assignmentGroupID)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return
	} else {
		if !generated {
			spinner.StopMessage(aurora.Sprintf(aurora.Red("project already exists")))
		}

		err = spinner.Stop()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
	}

	if starterrepo != nil {
		if !generated {
			fmt.Println(aurora.Red("    ↪ not trying to push startercode to existing project"))
		} else {
			cfg.Suffix = aurora.Sprintf(aurora.Cyan(" ↪ pushing startercode"))
			spinner, err := yacspin.New(cfg)
			if err != nil {
				log.Debug().Err(err).Msg("cannot create spinner")
			}
			err = spinner.Start()
			if err != nil {
				log.Debug().Err(err).Msg("cannot start spinner")
			}

			err = c.pushStartercode(assignmentCfg, starterrepo, project)
			if err != nil {
				spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

				err := spinner.StopFail()
				if err != nil {
					log.Debug().Err(err).Msg("cannot stop spinner")
				}
				return
			}

			err = spinner.Stop()
			if err != nil {
				log.Debug().Err(err).Msg("cannot stop spinner")
			}
		}
	} else if assignmentCfg.Seeder != nil {
		if !generated {
			fmt.Println(aurora.Red("    ↪ not running seeder for existing project"))
		} else {
			cfg.Suffix = aurora.Sprintf(aurora.Cyan(" ↪ seeding project %s using %s"),
				aurora.Magenta(projectname),
				aurora.Magenta(assignmentCfg.Seeder.Command),
			)
			spinner, err := yacspin.New(cfg)
			if err != nil {
				log.Debug().Err(err).Msg("cannot create spinner")
			}
			err = spinner.Start()
			if err != nil {
				log.Debug().Err(err).Msg("cannot start spinner")
			}

			err = c.runSeeder(assignmentCfg, project)
			if err != nil {
				spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

				err := spinner.StopFail()
				if err != nil {
					log.Debug().Err(err).Msg("cannot stop spinner")
				}
				return
			}

			err = spinner.Stop()
			if err != nil {
				log.Debug().Err(err).Msg("cannot stop spinner")
			}
		}
	}

	for _, student := range members {
		cfg.Suffix = aurora.Sprintf(aurora.Cyan(" ↪ adding member %s to %s as %s"),
			aurora.Yellow(student),
			aurora.Magenta(projectname),
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

		userID, err := c.getUserID(student)
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

func (c *Client) generatePerStudent(assignmentCfg *config.AssignmentConfig, assignmentGroupID int,
	starterrepo *git.Starterrepo) {
	if len(assignmentCfg.Students) == 0 {
		fmt.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		name := assignmentCfg.Name + "-" + assignmentCfg.EscapeUserName(student)
		c.generate(assignmentCfg, assignmentGroupID, name, []string{student}, starterrepo)
	}
}

func (c *Client) generatePerGroup(assignmentCfg *config.AssignmentConfig, assignmentGroupID int,
	starterrepo *git.Starterrepo) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		c.generate(assignmentCfg, assignmentGroupID, assignmentCfg.Name+"-"+grp.Name, grp.Members, starterrepo)
	}
}
