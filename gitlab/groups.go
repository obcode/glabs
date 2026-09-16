package gitlab

import (
	"fmt"
	"strings"
	"time"

	"github.com/obcode/glabs/v3/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// getGroupByFullPath returns the GitLab group stored at the given full path.
// The search endpoint only matches the last path segment, so the results are
// filtered by full path here.
func (c *Client) getGroupByFullPath(fullPath string) (*gitlab.Group, error) {
	pathParts := strings.Split(fullPath, "/")
	searchTerm := pathParts[len(pathParts)-1]

	groups, _, err := c.Groups.SearchGroup(searchTerm)
	if err != nil {
		log.Error().Err(err).
			Str("grouppath", fullPath).
			Msg("error while searching id of group path")
		return nil, err
	}

	for _, group := range groups {
		if group.FullPath == fullPath {
			return group, nil
		}
	}

	return nil, fmt.Errorf("no gitlab group found for path %s", fullPath)
}

func (c *Client) getGroupIDByFullPath(fullPath string) (int64, error) {
	group, err := c.getGroupByFullPath(fullPath)
	if err != nil {
		return 0, err
	}
	return group.ID, nil
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

// subgroupVisibility returns the visibility a new group should be created with.
// GitLab rejects a subgroup that is less restrictive than its parent ("internal
// is not allowed since the parent group has a private visibility"), so the
// preferred internal visibility is capped at the parent's. A parent whose
// visibility the API did not report yields nil, which omits the field from the
// request and lets GitLab pick a visibility that is valid for that parent.
func subgroupVisibility(parent *gitlab.Group) *gitlab.VisibilityValue {
	if parent == nil {
		return gitlab.Ptr(gitlab.InternalVisibility)
	}

	switch parent.Visibility {
	case gitlab.PrivateVisibility:
		return gitlab.Ptr(gitlab.PrivateVisibility)
	case gitlab.InternalVisibility, gitlab.PublicVisibility:
		return gitlab.Ptr(gitlab.InternalVisibility)
	default:
		return nil
	}
}

func (c *Client) createGroup(assignmentCfg *config.AssignmentConfig) (int64, error) {
	pathParts := strings.Split(assignmentCfg.Path, "/")
	path := pathParts[len(pathParts)-1]
	name := pathParts[len(pathParts)-1]

	var (
		parentID    *int64
		parentGroup *gitlab.Group
	)
	if len(pathParts) > 1 {
		parentPath := strings.Join(pathParts[:len(pathParts)-1], "/")
		parent, err := c.getGroupByFullPath(parentPath)
		if err != nil {
			log.Error().Err(err).
				Str("course", assignmentCfg.Course).
				Str("parentpath", parentPath).
				Msg("cannot resolve parent group for assignment")
			return 0, err
		}
		parentGroup = parent
		parentID = &parent.ID
	}

	fmt.Printf("GitLab group for assignment does not exist, creating group %s at %s\n", name, assignmentCfg.Path)

	options := &gitlab.CreateGroupOptions{
		Name:       &name,
		Path:       &path,
		Visibility: subgroupVisibility(parentGroup),
		ParentID:   parentID,
	}

	g, _, err := c.Groups.CreateGroup(options)
	if err != nil {
		log.Error().Err(err).
			Str("name", name).
			Str("path", path).
			Msg("cannot create group")
		return 0, fmt.Errorf("cannot create group %s: %w", assignmentCfg.Path, err)
	}
	return g.ID, nil
}

// AddGroupGuests adds all students from an assignment config as guests to the course subgroup
// (coursepath/semesterpath). This enables students to use the Dependency-Proxy.
func (c *Client) AddGroupGuests(courseName string) error {
	courseConfig, err := config.GetCourseConfig(courseName)
	if err != nil {
		return err
	}

	subgroupPath := config.GetCourseSubgroupPath(courseName)
	log.Info().
		Str("course", courseName).
		Str("subgroupPath", subgroupPath).
		Msg("adding students as guests to course subgroup")

	groupID, err := c.getGroupIDByFullPath(subgroupPath)
	if err != nil {
		return fmt.Errorf("cannot get course subgroup ID: %w", err)
	}

	studentMap := collectUniqueStudents(courseConfig)
	if len(studentMap) == 0 {
		log.Info().Str("course", courseName).Msg("no students found to add to course subgroup")
		return nil
	}

	successCount := 0
	for _, student := range studentMap {
		userID, err := c.getUserID(student)
		if err != nil {
			// Fallback: invite by email (works on instances that disallow user lookup by email)
			if student.Email != nil {
				info, inviteErr := c.inviteGroupByEmail(groupID, *student.Email, config.Guest)
				if inviteErr != nil {
					log.Warn().Err(inviteErr).Str("email", *student.Email).Msg("cannot invite student to group")
				} else {
					log.Debug().Str("email", *student.Email).Str("info", info).Msg(info)
					successCount++
				}
			} else {
				log.Warn().Err(err).Interface("student", student).Msg("cannot get user ID for student")
			}
			continue
		}

		info, err := c.addGroupMember(groupID, userID, config.Guest)
		if err != nil {
			log.Warn().Err(err).Int64("userID", userID).Str("groupPath", subgroupPath).Msg("cannot add student to group")
			continue
		}

		log.Debug().Int64("userID", userID).Str("info", info).Msg(info)
		successCount++
	}

	fmt.Printf("Added/invited %d students as guests to %s\n", successCount, subgroupPath)
	return nil
}

// inviteGroupByEmail sends a group invitation to the given email address
func (c *Client) inviteGroupByEmail(groupID int64, email string, accessLevel config.AccessLevel) (string, error) {
	expiresAt := gitlab.ISOTime(time.Now().UTC().AddDate(1, 0, 0))
	m := &gitlab.InvitesOptions{
		Email:       &email,
		AccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(accessLevel)),
		ExpiresAt:   &expiresAt,
	}
	resp, _, err := c.Invites.GroupInvites(groupID, m)
	if err != nil {
		return "", err
	}
	if resp.Status != "success" {
		return "", fmt.Errorf("inviting user %s failed with %s", email, resp.Message[email])
	}
	return fmt.Sprintf("successfully invited %s", email), nil
}

// addGroupMember adds a user as a member to a group with the specified access level
func (c *Client) addGroupMember(groupID, userID int64, accessLevel config.AccessLevel) (string, error) {
	member, _, _ := c.GroupMembers.GetGroupMember(groupID, userID, nil)
	if member != nil {
		if member.AccessLevel == gitlab.OwnerPermissions {
			return "already owner", nil
		}

		if member.AccessLevel != gitlab.AccessLevelValue(accessLevel) {
			e := &gitlab.EditGroupMemberOptions{
				AccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(accessLevel)),
			}
			_, _, err := c.GroupMembers.EditGroupMember(groupID, userID, e)
			if err != nil {
				return "", fmt.Errorf("error while trying to change access level: %w", err)
			}

			return fmt.Sprintf("set accesslevel from %s to %s", config.AccessLevel(member.AccessLevel).String(), accessLevel.String()), nil
		}

		return fmt.Sprintf("already member with accesslevel %s", config.AccessLevel(member.AccessLevel).String()), nil
	}

	m := &gitlab.AddGroupMemberOptions{
		UserID:      gitlab.Ptr(userID),
		AccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(accessLevel)),
		ExpiresAt:   gitlab.Ptr(time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02")),
	}
	member, _, err := c.GroupMembers.AddGroupMember(groupID, m)
	if err != nil {
		return "", fmt.Errorf("problem while adding member with id %d: %w", userID, err)
	}

	log.Debug().Int64("groupID", groupID).Msg("granted access to group")
	return fmt.Sprintf("added successfully with accesslevel %s", config.AccessLevel(member.AccessLevel).String()), nil
}

// collectUniqueStudents returns a deduplicated map of all students in the course config
func collectUniqueStudents(courseConfig *config.CourseConfig) map[string]*config.Student {
	studentMap := make(map[string]*config.Student)
	for _, student := range courseConfig.Students {
		if key := config.StudentKey(student); key != "" {
			studentMap[key] = student
		}
	}
	for _, group := range courseConfig.Groups {
		for _, member := range group.Members {
			if key := config.StudentKey(member); key != "" {
				studentMap[key] = member
			}
		}
	}
	return studentMap
}
