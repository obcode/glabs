package gitlab

import (
	"errors"
	"fmt"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) getUser(searchPattern string) (*gitlab.User, error) {
	u := &gitlab.ListUsersOptions{
		Search: gitlab.String(searchPattern),
	}
	users, _, err := c.Users.ListUsers(u)
	if err != nil {
		log.Fatal().Err(err)
	}

	if len(users) == 0 {
		log.Debug().Str("searchPattern", searchPattern).Msg("user not found")
		return nil, errors.New("user not found")
	} else if len(users) > 1 {
		log.Debug().Str("searchPattern", searchPattern).Msg("more than one user found")
		return nil, errors.New("more than one user found")
	}

	return users[0], nil
}

func (c *Client) getUserID(searchPattern string) (int, error) {
	user, err := c.getUser(searchPattern)

	if err != nil {
		log.Debug().Err(err).Str("searchPattern", searchPattern).Msg("cannot get User")
		return 0, fmt.Errorf("cannot get user: %w", err)
	}

	userID := user.ID
	log.Debug().Str("searchPattern", searchPattern).Int("userID", userID).Msg("found user with id")

	return userID, nil
}

func (c *Client) addMember(assignmentConfig *config.AssignmentConfig, projectID, userID int) (string, error) {
	member, _, _ := c.ProjectMembers.GetInheritedProjectMember(projectID, userID)
	if member != nil {
		if member.AccessLevel == gitlab.OwnerPermissions {
			return "already owner", nil
		}

		if member.AccessLevel != gitlab.AccessLevelValue(assignmentConfig.AccessLevel) {
			e := &gitlab.EditProjectMemberOptions{
				AccessLevel: gitlab.AccessLevel(gitlab.AccessLevelValue(assignmentConfig.AccessLevel)),
			}
			_, _, err := c.ProjectMembers.EditProjectMember(projectID, userID, e)
			if err != nil {
				return "", fmt.Errorf("error while trying to change access level: %w", err)
			}

			return fmt.Sprintf("set accesslevel from %s to %s", config.AccessLevel(member.AccessLevel).String(), assignmentConfig.AccessLevel.String()), nil
		}

		return fmt.Sprintf("already member with accesslevel %s", config.AccessLevel(member.AccessLevel).String()), nil
	}

	m := &gitlab.AddProjectMemberOptions{
		UserID:      gitlab.Int(userID),
		AccessLevel: gitlab.AccessLevel(gitlab.AccessLevelValue(assignmentConfig.AccessLevel)),
	}
	member, _, err := c.ProjectMembers.AddProjectMember(projectID, m)
	if err != nil {
		return "", fmt.Errorf("problem while adding member with id... %d: %w", userID, err)
	}

	log.Debug().Int("projectID", projectID).Msg("granted access to repo")
	return fmt.Sprintf("added successfully with accesslevel %s", config.AccessLevel(member.AccessLevel).String()), nil
}
