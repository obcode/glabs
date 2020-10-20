package gitlab

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func (c *Client) Generate(course, assignment string, onlyForStudentsOrGroups ...string) {
	if courseConf := viper.GetStringMap(course); len(courseConf) == 0 {
		log.Info().Str("course", course).Msg("configuration for course not found")
		return
	}

	assignmentKey := course + "." + assignment

	if assignmentConfig := viper.GetStringMap(assignmentKey); len(assignmentConfig) == 0 {
		log.Info().Str("assignment", assignment).Msg("no configuration for assignment found")
		return
	}

	assignmentGitLabGroupID, assignmentPath, err := c.getGroupID(course, assignmentKey)
	if err != nil {
		log.Error().Err(err).
			Str("assignment", assignment).
			Str("course", course).
			Msg("gitlab group for assignment does not exist")
		return
	}

	starterrepo := prepareStartercodeRepo(course, assignment)

	if len(onlyForStudentsOrGroups) > 0 {
		log.Info().Interface("only for", onlyForStudentsOrGroups).
			Msg("generating only for...")
	}

	switch viper.GetString(assignmentKey + ".per") {
	case "group":
		log.Info().Msg("generating for groups")
		fmt.Print("Press 'Enter' to continue or `Ctrl-C` to stop ...")
		fmt.Scanln()
		c.generatePerGroup(course, assignment, assignmentPath, assignmentGitLabGroupID, starterrepo,
			onlyForStudentsOrGroups...)
	case "student", "":
		log.Info().Msg("generating per student")
		fmt.Print("Press 'Enter' to continue or `Ctrl-C` to stop ...")
		fmt.Scanln()
		c.generatePerStudent(course, assignment, assignmentPath, assignmentGitLabGroupID, starterrepo,
			onlyForStudentsOrGroups...)
	default:
		log.Info().Msg("generating per unknown")
		return
	}
}

func (c *Client) generatePerStudent(course, assignment, assignmentPath string, assignmentGroupID int,
	starterrepo *starterrepo, onlyForStudents ...string) {
	students := viper.GetStringSlice(course + ".students")
	if len(onlyForStudents) > 0 {
		students = onlyForStudents
	}

	if len(students) == 0 {
		log.Info().Str("course", course).Msg("no students found")
		return
	}

	for _, student := range students {
		project, generated, err := c.generateProject(student, course, assignment, assignmentPath, assignmentGroupID)
		if err != nil {
			log.Error().Err(err).
				Str("student", student).
				Str("course", course).
				Str("assignment", assignment).
				Msg("error while generating project")
			break
		}

		if generated && starterrepo != nil {
			c.pushStartercode(course, assignment, starterrepo, project)
		}

		userID, err := c.getUserID(student)
		if err != nil {
			log.Error().Err(err).
				Str("student", student).
				Msg("error while trying to get student id")
			break
		}

		err = c.addMember(project.ID, userID, course+"."+assignment)
		if err != nil {
			log.Error().Err(err).
				Int("projectID", project.ID).
				Int("userID", userID).
				Str("student", student).
				Str("course", course).
				Str("assignment", assignment).
				Msg("error while adding member")
			break
		}
	}
}

func (c *Client) generatePerGroup(course, assignment, assignmentPath string, assignmentGroupID int,
	starterrepo *starterrepo, onlyForGroups ...string) {
	groups := viper.GetStringMapStringSlice(course + ".groups")
	if len(onlyForGroups) > 0 {
		onlyTheseGroups := make(map[string][]string)
		for _, onlyGroup := range onlyForGroups {
			for groupname, students := range groups {
				if onlyGroup == groupname {
					onlyTheseGroups[groupname] = students
				}
			}
		}
		groups = onlyTheseGroups
	}

	if len(groups) == 0 {
		log.Info().Str("group", course).Msg("no groups found")
		return
	}

	for grp, students := range groups {
		project, generated, err := c.generateProject(grp, course, assignment, assignmentPath, assignmentGroupID)
		if err != nil {
			log.Error().Err(err).
				Str("group", grp).
				Str("course", course).
				Str("assignment", assignment).
				Msg("error while generating project")
			break
		}

		if generated && starterrepo != nil {
			c.pushStartercode(course, assignment, starterrepo, project)
		}

		for _, student := range students {
			userID, err := c.getUserID(student)
			if err != nil {
				log.Error().Err(err).
					Str("student", student).
					Msg("error while trying to get student id")
				break
			}

			err = c.addMember(project.ID, userID, course+"."+assignment)
			if err != nil {
				log.Error().Err(err).
					Int("projectID", project.ID).
					Int("userID", userID).
					Str("student", student).
					Str("course", course).
					Str("assignment", assignment).
					Msg("error while adding member")
				break
			}
		}
	}
}
