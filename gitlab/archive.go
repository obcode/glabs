package gitlab

import (
	"fmt"
	"os"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) Archive(assignmentCfg *config.AssignmentConfig, unarchive bool) {
	_, err := c.getGroupID(assignmentCfg)
	if err != nil {
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		os.Exit(1)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.archivePerGroup(assignmentCfg, unarchive)
	case config.PerStudent:
		c.archivePerStudent(assignmentCfg, unarchive)
	default:
		fmt.Printf("it is only possible to set access levels for students oder groups, not for %v", per)
		os.Exit(1)
	}
}

func (c *Client) archivePerStudent(assignmentCfg *config.AssignmentConfig, unarchive bool) {
	if len(assignmentCfg.Students) == 0 {
		fmt.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		name := assignmentCfg.Name + "-" + assignmentCfg.RepoSuffix(student)
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, name)
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			fmt.Printf("cannot archive project %s failed with %s", projectname, err)
			return
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
		projectname := fmt.Sprintf("%s/%s-%s", assignmentCfg.Path, assignmentCfg.Name, grp.Name)
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			fmt.Printf("cannot archive project %s failed with %s", projectname, err)
			return
		}
		if err := c.archive(assignmentCfg, project, true, unarchive); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot archive project")
		}
	}
}

func (c *Client) archive(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, spin bool, unarchive bool) error {
	// var cfg yacspin.Config
	var spinner *yacspin.Spinner
	if spin {
		un := ""
		if unarchive {
			un = "un"
		}
		cfg := yacspin.Config{
			Frequency: 100 * time.Millisecond,
			CharSet:   yacspin.CharSets[69],
			Suffix: aurora.Sprintf(aurora.Cyan(" %sarchiving project %s at %s"),
				un,
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

	var err error
	if unarchive {
		_, _, err = c.Projects.UnarchiveProject(project.ID)
		if err != nil {
			log.Debug().Err(err).
				Str("name", project.Name).
				Str("toURL", project.SSHURLToRepo).
				Msg("cannot unarchive project")

			if spin {
				err := spinner.StopFail()
				if err != nil {
					log.Debug().Err(err).Msg("cannot stop spinner")
				}
			}
			return fmt.Errorf("error while trying to unarchive project: %w", err)
		}
	} else {
		_, _, err = c.Projects.ArchiveProject(project.ID)
		if err != nil {
			log.Debug().Err(err).
				Str("name", project.Name).
				Str("toURL", project.SSHURLToRepo).
				Msg("cannot archive project")

			if spin {
				err := spinner.StopFail()
				if err != nil {
					log.Debug().Err(err).Msg("cannot stop spinner")
				}
			}
			return fmt.Errorf("error while trying to archive project: %w", err)
		}
	}

	if spin {
		spinner.StopMessage(aurora.Sprintf(aurora.Green("ok")))
		err = spinner.Stop()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
	}

	return nil
}
