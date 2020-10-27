package gitlab

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) getUser(username string) (*gitlab.User, error) {
	u := &gitlab.ListUsersOptions{
		Username: gitlab.String(username),
	}
	users, _, err := c.Users.ListUsers(u)
	if err != nil {
		log.Fatal().Err(err)
	}

	if len(users) == 0 {
		log.Debug().Str("username", username).Msg("user not found")
		return nil, errors.New("user not found")
	} else if len(users) > 1 {
		log.Debug().Str("username", username).Msg("more than one user found")
		return nil, errors.New("more than one user found")
	}

	return users[0], nil
}

func (c *Client) getUserID(username string) (int, error) {
	user, err := c.getUser(username)

	if err != nil {
		log.Debug().Err(err).Str("username", username).Msg("cannot get User")
		return 0, fmt.Errorf("cannot get user: %w", err)
	}

	userID := user.ID
	log.Debug().Str("username", username).Int("userID", userID).Msg("found user with id")

	return userID, nil
}

func (c *Client) addMember(assignmentConfig *config.AssignmentConfig, projectID, userID int) error {
	m := &gitlab.AddProjectMemberOptions{
		UserID:      gitlab.Int(userID),
		AccessLevel: gitlab.AccessLevel(gitlab.AccessLevelValue(assignmentConfig.AccessLevel)),
	}
	_, resp, err := c.ProjectMembers.AddProjectMember(projectID, m)
	if err != nil {
		if resp.StatusCode == http.StatusConflict {
			log.Debug().Int("projectID", projectID).Msg("user should have already access to repo")
			return nil
		}
		log.Debug().Err(err).Msg("error while adding member")
		return fmt.Errorf("problem while adding member with id %d: %w", userID, err)
	}

	log.Debug().Int("projectID", projectID).Msg("granted access to repo")
	return nil
}
