package gitlab

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func (c *Client) Generate(group, assignment string) {
	if groupInfo := viper.GetStringMap(group); len(groupInfo) == 0 {
		log.Info().Str("group", group).Msg("goup not found")
		return
	}

	assignmentKey := group + "." + assignment

	if assignmentConfig := viper.GetStringMap(assignmentKey); len(assignmentConfig) == 0 {
		log.Info().Str("assignment", assignment).Msg("no configuration for assignment found")
		return
	}

	assignmentGroupID, assignmentPath, err := c.getGroupID(group, assignmentKey)
	if err != nil {
		log.Error().Err(err).Str("assignment", assignment).Msg("gitlab group for assignment does not exist")
		return
	}

	starterrepo := prepareStartercodeRepo(group, assignment)

	switch viper.GetString(assignmentKey + ".per") {
	case "group":
		log.Info().Msg("generating for groups")
		fmt.Print("Press 'Enter' to continue or `Ctrl-C` to stop ...")
		fmt.Scanln()
		c.generatePerGroup(group, assignment, assignmentPath, assignmentGroupID, starterrepo)
	case "student", "":
		log.Info().Msg("generating per student")
		fmt.Print("Press 'Enter' to continue or `Ctrl-C` to stop ...")
		fmt.Scanln()
		c.generatePerStudent(group, assignment, assignmentPath, assignmentGroupID, starterrepo)
	default:
		log.Info().Msg("generating per unknown")
		return
	}
}

func (c *Client) generatePerStudent(group, assignment, assignmentPath string, assignmentGroupID int,
	starterrepo *starterrepo) {
	students := viper.GetStringSlice(group + ".students")
	if len(students) == 0 {
		log.Info().Str("group", group).Msg("no students found")
		return
	}

	for _, student := range students {
		project, generated, err := c.generateProject(student, group, assignment, assignmentPath, assignmentGroupID)
		if err != nil {
			log.Error().Err(err).
				Str("student", student).
				Str("group", group).
				Str("assignment", assignment).
				Msg("error while generating project")
			break
		}

		if generated && starterrepo != nil {
			pushStartercode(starterrepo, project.Name, project.SSHURLToRepo)
		}

		userID, err := c.getUserID(student)
		if err != nil {
			log.Error().Err(err).
				Str("student", student).
				Msg("error while trying to get student id")
			break
		}

		err = c.addMember(project.ID, userID, group+"."+assignment)
		if err != nil {
			log.Error().Err(err).
				Int("projectID", project.ID).
				Int("userID", userID).
				Str("student", student).
				Str("group", group).
				Str("assignment", assignment).
				Msg("error while adding member")
			break
		}
	}
}

func (c *Client) generatePerGroup(group, assignment, assignmentPath string, assignmentGroupID int,
	starterrepo *starterrepo) {
	groups := viper.GetStringMapStringSlice(group + ".groups")
	if len(groups) == 0 {
		log.Info().Str("group", group).Msg("no groups found")
		return
	}

	for grp, students := range groups {
		project, generated, err := c.generateProject(grp, group, assignment, assignmentPath, assignmentGroupID)
		if err != nil {
			log.Error().Err(err).
				Str("studentgroup", grp).
				Str("group", group).
				Str("assignment", assignment).
				Msg("error while generating project")
			break
		}

		if generated && starterrepo != nil {
			pushStartercode(starterrepo, project.Name, project.SSHURLToRepo)
		}

		for _, student := range students {
			userID, err := c.getUserID(student)
			if err != nil {
				log.Error().Err(err).
					Str("student", student).
					Msg("error while trying to get student id")
				break
			}

			err = c.addMember(project.ID, userID, group+"."+assignment)
			if err != nil {
				log.Error().Err(err).
					Int("projectID", project.ID).
					Int("userID", userID).
					Str("student", student).
					Str("group", group).
					Str("assignment", assignment).
					Msg("error while adding member")
				break
			}
		}
	}
}
