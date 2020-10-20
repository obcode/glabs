package gitlab

import (
	"fmt"
	"strings"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
)

func (c *Client) getGroupID(assignmentCfg *config.AssignmentConfig) (int, error) {
	pathParts := strings.Split(assignmentCfg.Path, "/")
	groups, _, err := c.Groups.SearchGroup(pathParts[len(pathParts)-1])

	if err != nil {
		log.Error().Err(err).
			Str("course", assignmentCfg.Course).
			Str("assignmentpath", assignmentCfg.Path).
			Msg("error while searching id of assignmentPath")
		return 0, err
	}

	if len(groups) == 0 {
		log.Debug().Str("group", assignmentCfg.Course).
			Str("assignmentpath", assignmentCfg.Path).
			Msg("no group found")
		return 0, fmt.Errorf("no gitlab group found for assignmentpath %s", assignmentCfg.Path)
	}

	log.Debug().Str("assignmentpath", assignmentCfg.Path).Msg("searching id of gitlab group")

	// semesterpathID := 0
	assignmentGroupID := 0

	for _, group := range groups {
		// if group.Path == semesterpath {
		// 	log.Debug().Str("group.Path", group.Path).Msg("found semester group")
		// 	semesterpathID = group.ID
		// }
		if group.FullPath == assignmentCfg.Path {
			log.Debug().Str("group.FullPath", group.FullPath).Msg("found assignment group")
			assignmentGroupID = group.ID
		}
	}

	if assignmentGroupID == 0 {
		log.Info().Msg("creating assignment group")
		log.Error().
			Str("course", assignmentCfg.Course).
			Str("assignmentpath", assignmentCfg.Path).
			Msg("please go to the gitlab website and create the subgroup with the assignment patz")
	}

	return assignmentGroupID, nil
}
