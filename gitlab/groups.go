package gitlab

import (
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func (c *Client) getGroupID(group, assignmentKey string) (int, string, error) {
	semesterGroup := group
	if semestergroup := viper.GetString(group + ".semestergroup"); len(semestergroup) > 0 {
		semesterGroup += "/" + semestergroup
	}

	assignmentGroup := semesterGroup
	if group := viper.GetString(assignmentKey + ".group"); len(group) > 0 {
		assignmentGroup += "/" + group
	}

	groupnames := strings.Split(assignmentGroup, "/")
	groups, _, err := c.Groups.SearchGroup(groupnames[len(groupnames)-1])

	if err != nil {
		log.Error().Err(err).Str("group", group).Msg("error while searching id of group")
		return 0, "", err
	}

	if len(groups) == 0 {
		log.Debug().Str("group", group).Msg("no group found")
		return 0, "", errors.New("no group found")
	}

	log.Debug().Str("assignmentGroup", assignmentGroup).Msg("searching id of group")

	// semesterGroupID := 0
	assignmentGroupID := 0

	for _, group := range groups {
		// if group.Path == semesterGroup {
		// 	log.Debug().Str("group.Path", group.Path).Msg("found semester group")
		// 	semesterGroupID = group.ID
		// }
		if group.FullPath == assignmentGroup {
			log.Debug().Str("group.FullPath", group.FullPath).Msg("found assignment group")
			assignmentGroupID = group.ID
		}
	}

	if assignmentGroupID == 0 {
		log.Info().Msg("creating assignment group")
		panic("implement me") // TODO: implement me
	}

	return assignmentGroupID, assignmentGroup, nil
}
