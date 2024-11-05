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
	"github.com/xanzy/go-gitlab"
)

func (c *Client) Update(assignmentCfg *config.AssignmentConfig) {
	_, err := c.getGroupID(assignmentCfg)
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
		c.updatePerGroup(assignmentCfg, starterrepo)
	case config.PerStudent:
		c.updatePerStudent(assignmentCfg, starterrepo)
	default:
		fmt.Printf("it is only possible to update for students oder groups, not for %v", per)
		os.Exit(1)
	}
}

func (c *Client) update(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, starterrepo *git.Starterrepo) {

	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" updating project %s at %s"),
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

	if starterrepo != nil {
		cfg.Suffix = aurora.Sprintf(aurora.Cyan(" ↪ pushing updates from startercode"))

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
}

func (c *Client) updatePerStudent(assignmentCfg *config.AssignmentConfig, starterrepo *git.Starterrepo) {
	if len(assignmentCfg.Students) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no students found")
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
			fmt.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		c.update(assignmentCfg, project, starterrepo)
	}
}

func (c *Client) updatePerGroup(assignmentCfg *config.AssignmentConfig, starterrepo *git.Starterrepo) {
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
		c.update(assignmentCfg, project, starterrepo)
	}
}
