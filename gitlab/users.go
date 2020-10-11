package gitlab

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) getUserID(username string) (int, error) {
	u := &gitlab.ListUsersOptions{
		Username: gitlab.String(username),
	}
	users, _, err := c.Users.ListUsers(u)
	if err != nil {
		log.Fatal().Err(err)
	}

	if len(users) == 0 {
		log.Debug().Str("username", username).Msg("user not found")
		return 0, errors.New("user not found")
	} else if len(users) > 1 {
		log.Debug().Str("username", username).Msg("more than one user found")
		return 0, errors.New("more than one user found")
	}

	userID := users[0].ID
	log.Debug().Str("username", username).Int("userID", userID).Msg("found user with id")

	return userID, nil
}

func (c *Client) addMember(projectID, userID int, assignmentKey string) error {
	accesslevel := 30 // Developer is default

	if accesslevelIdentifier := viper.GetString(assignmentKey + ".accesslevel"); accesslevelIdentifier != "" {
		switch accesslevelIdentifier {
		case "guest":
			accesslevel = 10
		case "reporter":
			accesslevel = 20
		case "maintainer":
			accesslevel = 40
		}
	}

	m := &gitlab.AddProjectMemberOptions{
		UserID:      gitlab.Int(userID),
		AccessLevel: gitlab.AccessLevel(gitlab.AccessLevelValue(accesslevel)),
	}
	_, resp, err := c.ProjectMembers.AddProjectMember(projectID, m)
	if err != nil {
		if resp.StatusCode == http.StatusConflict {
			log.Debug().Int("projectID", projectID).Msg("user should have already access to repo")
			return nil
		}
		log.Error().Err(err).Msg("error while adding member")
		return err
	}

	log.Debug().Int("projectID", projectID).Msg("granted access to repo")
	return nil
}
