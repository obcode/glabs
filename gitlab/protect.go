package gitlab

import (
	"fmt"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/reporter"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) ProtectToBranch(assignmentCfg *config.AssignmentConfig) error {
	_, err := c.getGroupID(assignmentCfg)
	if err != nil {
		return fmt.Errorf("GitLab group for assignment does not exist, please create the group %s", assignmentCfg.URL)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.protectToBranchPerGroup(assignmentCfg)
	case config.PerStudent:
		c.protectToBranchPerStudent(assignmentCfg)
	default:
		return fmt.Errorf("it is only possible to protect the branch for students or groups, not for %v", per)
	}
	return nil
}

func (c *Client) protectBranch(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, spin bool) error {
	return c.protectBranchForMemberCount(assignmentCfg, project, spin, 0)
}

func (c *Client) protectBranchForMemberCount(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, spin bool, memberCount int) error {
	if !hasProtectedBranches(assignmentCfg.Branches) && !hasMergeRequestApprovalConfig(assignmentCfg.MergeRequest) {
		return nil
	}

	task := reporter.NopTask()
	if spin {
		task = c.rep.Task(aurora.Sprintf(aurora.Cyan(" protect branch for project %s at %s"),
			aurora.Yellow(project.Name),
			aurora.Magenta(assignmentCfg.URL+"/"+project.Name),
		))
	}

	log.Debug().
		Str("name", project.Name).
		Str("toURL", project.HTTPURLToRepo).
		Interface("branches", assignmentCfg.Branches).
		Msg("protecting branch")

	for _, branch := range assignmentCfg.Branches {
		if !branch.Protect && !branch.MergeOnly {
			continue
		}

		pushLevel := gitlab.MaintainerPermissions
		mergeLevel := gitlab.MaintainerPermissions
		if branch.MergeOnly {
			pushLevel = gitlab.NoPermissions
			mergeLevel = gitlab.DeveloperPermissions
		}

		if err := c.protectSingleBranch(project, branch, pushLevel, mergeLevel); err != nil {
			task.Fail("")
			return err
		}
	}

	if err := c.applyMergeRequestApprovalRulesForMemberCount(assignmentCfg, project, memberCount); err != nil {
		task.Fail("")
		return err
	}

	task.Done(aurora.Sprintf(aurora.Green("ok")))
	return nil
}

func hasProtectedBranches(branches []config.BranchRule) bool {
	for _, branch := range branches {
		if branch.Protect || branch.MergeOnly {
			return true
		}
	}

	return false
}

func hasMergeRequestApprovalConfig(mergeRequest *config.MergeRequest) bool {
	if mergeRequest == nil {
		return false
	}

	return len(mergeRequest.Approvals) > 0 || mergeRequest.ApprovalSettings != nil
}

func (c *Client) protectSingleBranch(
	project *gitlab.Project,
	branch config.BranchRule,
	pushAccessLevel gitlab.AccessLevelValue,
	mergeAccessLevel gitlab.AccessLevelValue,
) error {
	existing, _, err := c.ProtectedBranches.GetProtectedBranch(project.ID, branch.Name)
	if err == nil {
		// GitLab can keep stale push permissions when updating existing rules to
		// "No one" (merge-only). Recreate the rule to enforce the access levels.
		if pushAccessLevel == gitlab.NoPermissions {
			if err := c.recreateProtectedBranch(project, branch, pushAccessLevel, mergeAccessLevel); err != nil {
				return err
			}
			return nil
		}

		updateOpts := &gitlab.UpdateProtectedBranchOptions{
			AllowedToPush:             replaceBranchPermissions(existing.PushAccessLevels, pushAccessLevel),
			AllowedToMerge:            replaceBranchPermissions(existing.MergeAccessLevels, mergeAccessLevel),
			AllowedToUnprotect:        replaceBranchPermissions(existing.UnprotectAccessLevels, gitlab.MaintainerPermissions),
			AllowForcePush:            gitlab.Ptr(branch.AllowForcePush),
			CodeOwnerApprovalRequired: gitlab.Ptr(branch.CodeOwnerApprovalRequired),
		}

		_, _, err = c.ProtectedBranches.UpdateProtectedBranch(project.ID, branch.Name, updateOpts)
		if err != nil {
			log.Debug().Err(err).
				Str("name", project.Name).
				Str("toURL", project.HTTPURLToRepo).
				Str("branch", branch.Name).
				Msg("cannot update protected branch")
			return fmt.Errorf("error while trying to update protected branch %s: %w", branch.Name, err)
		}

		return nil
	}

	if !isProtectedBranchNotFoundError(err) {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.HTTPURLToRepo).
			Str("branch", branch.Name).
			Msg("cannot read protected branch")
		return fmt.Errorf("error while trying to read protected branch %s: %w", branch.Name, err)
	}

	if err := c.recreateProtectedBranch(project, branch, pushAccessLevel, mergeAccessLevel); err != nil {
		return err
	}

	return nil
}

func (c *Client) recreateProtectedBranch(
	project *gitlab.Project,
	branch config.BranchRule,
	pushAccessLevel gitlab.AccessLevelValue,
	mergeAccessLevel gitlab.AccessLevelValue,
) error {
	_, err := c.ProtectedBranches.UnprotectRepositoryBranches(project.ID, branch.Name)
	if err != nil && !isProtectedBranchNotFoundError(err) {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.HTTPURLToRepo).
			Str("branch", branch.Name).
			Msg("cannot unprotect branch")
		return fmt.Errorf("error while trying to unprotect branch %s: %w", branch.Name, err)
	}

	opts := &gitlab.ProtectRepositoryBranchesOptions{
		Name:                      gitlab.Ptr(branch.Name),
		PushAccessLevel:           gitlab.Ptr(pushAccessLevel),
		MergeAccessLevel:          gitlab.Ptr(mergeAccessLevel),
		UnprotectAccessLevel:      gitlab.Ptr(gitlab.MaintainerPermissions),
		AllowForcePush:            gitlab.Ptr(branch.AllowForcePush),
		CodeOwnerApprovalRequired: gitlab.Ptr(branch.CodeOwnerApprovalRequired),
	}

	_, _, err = c.ProtectedBranches.ProtectRepositoryBranches(project.ID, opts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.HTTPURLToRepo).
			Str("branch", branch.Name).
			Msg("error while protecting branch")
		return fmt.Errorf("error while trying to protect branch %s: %w", branch.Name, err)
	}

	return nil
}

func replaceBranchPermissions(existing []*gitlab.BranchAccessDescription, accessLevel gitlab.AccessLevelValue) *[]*gitlab.BranchPermissionOptions {
	destroy := true
	permissions := make([]*gitlab.BranchPermissionOptions, 0, len(existing)+1)
	for _, level := range existing {
		if level == nil || level.ID <= 0 {
			continue
		}

		id := level.ID
		permissions = append(permissions, &gitlab.BranchPermissionOptions{ID: &id, Destroy: &destroy})
	}

	permissions = append(permissions, &gitlab.BranchPermissionOptions{AccessLevel: gitlab.Ptr(accessLevel)})
	return &permissions
}

func isProtectedBranchNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}

func (c *Client) protectToBranchPerStudent(assignmentCfg *config.AssignmentConfig) {
	if len(assignmentCfg.Students) == 0 {
		c.rep.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForStudent(student))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			c.rep.Printf("cannot protect branch for project %s failed with %s", projectname, err)
			continue
		}
		if err := c.protectBranchForMemberCount(assignmentCfg, project, true, 1); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot protect the branch")
		}
	}
}

func (c *Client) protectToBranchPerGroup(assignmentCfg *config.AssignmentConfig) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForGroup(grp))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			c.rep.Printf("cannot protect branch for project %s failed with %s", projectname, err)
			continue
		}
		if err := c.protectBranchForMemberCount(assignmentCfg, project, true, len(grp.Members)); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot protect the branch")
		}
	}
}
