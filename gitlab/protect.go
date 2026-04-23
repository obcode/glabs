package gitlab

import (
	"fmt"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) ProtectToBranch(assignmentCfg *config.AssignmentConfig) {
	_, err := c.getGroupID(assignmentCfg)
	if err != nil {
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		exitFunc(1)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.protectToBranchPerGroup(assignmentCfg)
	case config.PerStudent:
		c.protectToBranchPerStudent(assignmentCfg)
	default:
		fmt.Printf("it is only possible to protect the branch for students oder groups, not for %v", per)
		exitFunc(1)
	}
}

func (c *Client) protectBranch(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, spin bool) error {
	if assignmentCfg.Startercode.ProtectToBranch || assignmentCfg.Startercode.ProtectDevBranchMergeOnly {
		// var cfg yacspin.Config
		var spinner *yacspin.Spinner
		if spin {
			cfg := yacspin.Config{
				Frequency: 100 * time.Millisecond,
				CharSet:   yacspin.CharSets[69],
				Suffix: aurora.Sprintf(aurora.Cyan(" protect branch for project %s at %s"),
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
			var err error
			spinner, err = yacspin.New(cfg)
			if err != nil {
				log.Debug().Err(err).Msg("cannot create spinner")
			}
			err = spinner.Start()
			if err != nil {
				log.Debug().Err(err).Msg("cannot start spinner")
			}
		}

		log.Debug().
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", assignmentCfg.Startercode.ToBranch).
			Msg("protecting branch")

		if assignmentCfg.Startercode.ProtectDevBranchMergeOnly &&
			assignmentCfg.Startercode.DevBranch == assignmentCfg.Startercode.ToBranch {
			err := c.protectSingleBranch(
				project,
				assignmentCfg.Startercode.ToBranch,
				gitlab.NoPermissions,
				gitlab.DeveloperPermissions,
			)
			if err != nil {
				if spin {
					err := spinner.StopFail()
					if err != nil {
						log.Debug().Err(err).Msg("cannot stop spinner")
					}
				}
				return err
			}
		} else {
			if assignmentCfg.Startercode.ProtectToBranch {
				err := c.protectSingleBranch(
					project,
					assignmentCfg.Startercode.ToBranch,
					gitlab.MaintainerPermissions,
					gitlab.MaintainerPermissions,
				)
				if err != nil {
					if spin {
						err := spinner.StopFail()
						if err != nil {
							log.Debug().Err(err).Msg("cannot stop spinner")
						}
					}
					return err
				}
			}

			if assignmentCfg.Startercode.ProtectDevBranchMergeOnly {
				err := c.protectSingleBranch(
					project,
					assignmentCfg.Startercode.DevBranch,
					gitlab.NoPermissions,
					gitlab.DeveloperPermissions,
				)
				if err != nil {
					if spin {
						err := spinner.StopFail()
						if err != nil {
							log.Debug().Err(err).Msg("cannot stop spinner")
						}
					}
					return err
				}
			}
		}

		if spin {
			spinner.StopMessage(aurora.Sprintf(aurora.Green("ok")))
			if err := spinner.Stop(); err != nil {
				log.Debug().Err(err).Msg("cannot stop spinner")
			}
		}
	}

	return nil
}

func (c *Client) protectSingleBranch(
	project *gitlab.Project,
	branch string,
	pushAccessLevel gitlab.AccessLevelValue,
	mergeAccessLevel gitlab.AccessLevelValue,
) error {
	_, err := c.ProtectedBranches.UnprotectRepositoryBranches(project.ID, branch)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", branch).
			Msg("cannot unprotect branch, but that is okay")
	}

	opts := &gitlab.ProtectRepositoryBranchesOptions{
		Name:                 gitlab.Ptr(branch),
		PushAccessLevel:      gitlab.Ptr(pushAccessLevel),
		MergeAccessLevel:     gitlab.Ptr(mergeAccessLevel),
		UnprotectAccessLevel: gitlab.Ptr(gitlab.MaintainerPermissions),
	}

	_, _, err = c.ProtectedBranches.ProtectRepositoryBranches(project.ID, opts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", branch).
			Msg("error while protecting branch")
		return fmt.Errorf("error while trying to protect branch %s: %w", branch, err)
	}

	return nil
}

func (c *Client) protectToBranchPerStudent(assignmentCfg *config.AssignmentConfig) {
	if len(assignmentCfg.Students) == 0 {
		fmt.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForStudent(student))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			fmt.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		if err := c.protectBranch(assignmentCfg, project, true); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot protect the branch")
		}
	}
}

func (c *Client) protectToBranchPerGroup(assignmentCfg *config.AssignmentConfig) {
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
			fmt.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		if err := c.protectBranch(assignmentCfg, project, true); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot protect the branch")
		}
	}
}
