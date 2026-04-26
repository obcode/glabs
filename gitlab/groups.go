package gitlab

import (
	"fmt"
	"strings"

	"github.com/obcode/glabs/v2/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) getGroupIDByFullPath(fullPath string) (int64, error) {
	pathParts := strings.Split(fullPath, "/")
	searchTerm := pathParts[len(pathParts)-1]

	groups, _, err := c.Groups.SearchGroup(searchTerm)
	if err != nil {
		log.Error().Err(err).
			Str("grouppath", fullPath).
			Msg("error while searching id of group path")
		return 0, err
	}

	for _, group := range groups {
		if group.FullPath == fullPath {
			return group.ID, nil
		}
	}

	return 0, fmt.Errorf("no gitlab group found for path %s", fullPath)
}

func (c *Client) getGroupID(assignmentCfg *config.AssignmentConfig) (int64, error) {
	assignmentGroupID, err := c.getGroupIDByFullPath(assignmentCfg.Path)
	if err != nil {
		log.Debug().Err(err).
			Str("course", assignmentCfg.Course).
			Str("assignmentpath", assignmentCfg.Path).
			Msg("error while searching id of assignment path")
		return 0, err
	}

	return assignmentGroupID, nil
}

func (c *Client) createGroup(assignmentCfg *config.AssignmentConfig) (int64, error) {
	pathParts := strings.Split(assignmentCfg.Path, "/")
	path := pathParts[len(pathParts)-1]
	name := pathParts[len(pathParts)-1]

	var parentID *int64
	if len(pathParts) > 1 {
		parentPath := strings.Join(pathParts[:len(pathParts)-1], "/")
		resolvedParentID, err := c.getGroupIDByFullPath(parentPath)
		if err != nil {
			log.Error().Err(err).
				Str("course", assignmentCfg.Course).
				Str("parentpath", parentPath).
				Msg("cannot resolve parent group for assignment")
			return 0, err
		}
		parentID = &resolvedParentID
	}

	fmt.Printf("GitLab group for assignment does not exist, creating group %s at %s\n", name, assignmentCfg.Path)

	visibility := gitlab.InternalVisibility
	options := &gitlab.CreateGroupOptions{
		Name:       &name,
		Path:       &path,
		Visibility: &visibility,
		ParentID:   parentID,
	}

	g, _, err := c.Groups.CreateGroup(options)
	if err != nil {
		log.Error().Err(err).
			Str("name", name).
			Str("path", path).
			Msg("cannot create group")
	}
	return g.ID, nil
}
