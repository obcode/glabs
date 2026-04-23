package gitlab

import (
	"fmt"
	"strings"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) syncConfiguredBranches(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, baseBranch string) error {
	if len(assignmentCfg.Branches) == 0 {
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

	if err := c.protectBranch(assignmentCfg, project, false); err != nil {
		log.Debug().Err(err).Str("project", project.Name).Msg("cannot protect configured branches")
	}

	return nil
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
