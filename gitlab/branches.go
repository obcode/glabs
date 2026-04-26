package gitlab

import (
	"fmt"
	"strings"

	"github.com/obcode/glabs/v2/config"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) syncConfiguredBranches(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, baseBranch string, memberCount int) error {
	if len(assignmentCfg.Branches) == 0 {
		if hasMergeRequestApprovalConfig(assignmentCfg.MergeRequest) {
			return c.protectBranchForMemberCount(assignmentCfg, project, false, memberCount)
		}
		return nil
	}

	for _, branch := range assignmentCfg.Branches {
		if branch.Name == "" {
			continue
		}

		if branch.Name != baseBranch {
			opts := &gitlab.CreateBranchOptions{
				Branch: gitlab.Ptr(branch.Name),
				Ref:    gitlab.Ptr(baseBranch),
			}

			_, _, err := c.Branches.CreateBranch(project.ID, opts)
			if err != nil && !isBranchAlreadyExistsError(err) {
				return fmt.Errorf("error while creating branch %s from %s: %w", branch.Name, baseBranch, err)
			}
		}
	}

	defaultBranch := defaultBranchName(assignmentCfg.Branches, baseBranch)
	projectOpts := &gitlab.EditProjectOptions{DefaultBranch: gitlab.Ptr(defaultBranch)}
	_, _, err := c.Projects.EditProject(project.ID, projectOpts)
	if err != nil {
		return fmt.Errorf("error while switching default branch to %s: %w", defaultBranch, err)
	}

	return c.protectBranchForMemberCount(assignmentCfg, project, false, memberCount)
}

func defaultBranchName(branches []config.BranchRule, fallback string) string {
	for _, branch := range branches {
		if branch.Default && branch.Name != "" {
			return branch.Name
		}
	}

	if fallback != "" {
		return fallback
	}

	if len(branches) > 0 && branches[0].Name != "" {
		return branches[0].Name
	}

	return "main"
}

func isBranchAlreadyExistsError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "has already been taken")
}
