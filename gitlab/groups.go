package gitlab

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func (c *Client) getGroupID(course, assignmentKey string) (int, string, error) {
	path := viper.GetString(course + ".coursepath")
	if semesterpath := viper.GetString(course + ".semesterpath"); len(semesterpath) > 0 {
		path += "/" + semesterpath
	}

	assignmentpath := path
	if group := viper.GetString(assignmentKey + ".assignmentpath"); len(group) > 0 {
		assignmentpath += "/" + group
	}

	pathParts := strings.Split(assignmentpath, "/")
	groups, _, err := c.Groups.SearchGroup(pathParts[len(pathParts)-1])

	if err != nil {
		log.Error().Err(err).
			Str("course", course).
			Str("assignmentpath", assignmentpath).
			Msg("error while searching id of assignmentPath")
		return 0, "", err
	}

	if len(groups) == 0 {
		log.Debug().Str("group", course).
			Str("assignmentpath", assignmentpath).
			Msg("no group found")
		return 0, "", fmt.Errorf("no gitlab group found for assignmentpath %s", assignmentpath)
	}

	log.Debug().Str("assignmentpath", assignmentpath).Msg("searching id of gitlab group")

	// semesterpathID := 0
	assignmentGroupID := 0

	for _, group := range groups {
		// if group.Path == semesterpath {
		// 	log.Debug().Str("group.Path", group.Path).Msg("found semester group")
		// 	semesterpathID = group.ID
		// }
		if group.FullPath == assignmentpath {
			log.Debug().Str("group.FullPath", group.FullPath).Msg("found assignment group")
			assignmentGroupID = group.ID
		}
	}

	if assignmentGroupID == 0 {
		log.Info().Msg("creating assignment group")
		log.Error().
			Str("course", course).
			Str("assignmentpath", assignmentpath).
			Msg("please go to the gitlab website and create the subgroup with the assignment patz")
	}

	return assignmentGroupID, assignmentpath, nil
}
