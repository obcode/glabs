package gitlab

import (
	"fmt"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
)

func (c *Client) Generate(assignmentCfg *config.AssignmentConfig) {
	assignmentGitLabGroupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		log.Debug().Err(err).
			Str("assignment", assignmentCfg.Name).
			Str("course", assignmentCfg.Course).
			Msg("gitlab group for assignment does not exist")
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		return
	}

	starterrepo := prepareStartercodeRepo(assignmentCfg)

	switch assignmentCfg.Per {
	case config.PerGroup:
		log.Info().Msg("generating for groups")
		fmt.Print("Press 'Enter' to continue or `Ctrl-C` to stop ...")
		fmt.Scanln()
		c.generatePerGroup(assignmentCfg, assignmentGitLabGroupID, starterrepo)
	case config.PerStudent:
		log.Debug().
			Interface("students", assignmentCfg.Students).
			Msg("generating per student")
		fmt.Print("Press 'Enter' to continue or `Ctrl-C` to stop ...")
		fmt.Scanln()
		c.generatePerStudent(assignmentCfg, assignmentGitLabGroupID, starterrepo)
	default:
		log.Info().Msg("generating per unknown")
		return
	}
}

func (c *Client) generatePerStudent(assignmentCfg *config.AssignmentConfig, assignmentGroupID int,
	starterrepo *starterrepo) {
	if len(assignmentCfg.Students) == 0 {
		log.Info().Str("course", assignmentCfg.Course).Msg("no students found")
		return
	}

	for _, student := range assignmentCfg.Students {
		log.Debug().Str("student", student).Msg("generating for...")

		project, generated, err := c.generateProject(assignmentCfg, student, assignmentGroupID)
		if err != nil {
			log.Error().Err(err).
				Str("student", student).
				Str("course", assignmentCfg.Course).
				Str("assignment", assignmentCfg.Name).
				Msg("error while generating project")
			break
		}

		if generated && starterrepo != nil {
			c.pushStartercode(assignmentCfg, starterrepo, project)
		}

		userID, err := c.getUserID(student)
		if err != nil {
			log.Error().Err(err).
				Str("student", student).
				Msg("error while trying to get student id")
			break
		}

		err = c.addMember(assignmentCfg, project.ID, userID)
		if err != nil {
			log.Error().Err(err).
				Int("projectID", project.ID).
				Int("userID", userID).
				Str("student", student).
				Str("course", assignmentCfg.Course).
				Str("assignment", assignmentCfg.Name).
				Msg("error while adding member")
		}
	}
}

func (c *Client) generatePerGroup(assignmentCfg *config.AssignmentConfig, assignmentGroupID int,
	starterrepo *starterrepo) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		project, generated, err := c.generateProject(assignmentCfg, grp.Name, assignmentGroupID)
		if err != nil {
			log.Error().Err(err).
				Str("group", grp.Name).
				Str("course", assignmentCfg.Course).
				Str("assignment", assignmentCfg.Name).
				Msg("error while generating project")
			break
		}

		if generated && starterrepo != nil {
			c.pushStartercode(assignmentCfg, starterrepo, project)
		}

		for _, student := range grp.Members {
			userID, err := c.getUserID(student)
			if err != nil {
				log.Error().Err(err).
					Str("student", student).
					Msg("error while trying to get student id")
				break
			}

			err = c.addMember(assignmentCfg, project.ID, userID)
			if err != nil {
				log.Error().Err(err).
					Int("projectID", project.ID).
					Int("userID", userID).
					Str("student", student).
					Str("course", assignmentCfg.Course).
					Str("assignment", assignmentCfg.Name).
					Msg("error while adding member")
				break
			}
		}
	}
}
