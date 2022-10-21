package gitlab

import (
	"fmt"
	"os"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/git"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
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
	projectname string, members []*config.Student, starterrepo *git.Starterrepo) {

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
	c.setaccess(assignmentCfg, project, members, &cfg)
}

func (c *Client) generatePerStudent(assignmentCfg *config.AssignmentConfig, assignmentGroupID int,
	starterrepo *git.Starterrepo) {
	if len(assignmentCfg.Students) == 0 {
		fmt.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		name := assignmentCfg.Name + "-" + assignmentCfg.RepoSuffix(student)
		c.generate(assignmentCfg, assignmentGroupID, name, []*config.Student{student}, starterrepo)
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
