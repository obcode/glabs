package gitlab

import (
	"fmt"
	"net/mail"
	"sort"
	"strconv"
	"strings"

	"github.com/obcode/glabs/v2/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) applyMergeRequestApprovalRules(assignmentCfg *config.AssignmentConfig, project *gitlab.Project) error {
	return c.applyMergeRequestApprovalRulesForMemberCount(assignmentCfg, project, 0)
}

func (c *Client) applyMergeRequestApprovalRulesForMemberCount(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, memberCount int) error {
	if assignmentCfg.MergeRequest == nil {
		return nil
	}

	if err := c.applyMergeRequestApprovalSettings(assignmentCfg.MergeRequest.ApprovalSettings, project); err != nil {
		return err
	}

	if len(assignmentCfg.MergeRequest.Approvals) == 0 {
		return nil
	}

	protectedBranches, _, err := c.ProtectedBranches.ListProtectedBranches(project.ID, nil)
	if err != nil {
		return fmt.Errorf("cannot list protected branches for approval rules: %w", err)
	}

	protectedBranchIDByName := make(map[string]int64, len(protectedBranches))
	protectedBranchNames := make([]string, 0, len(protectedBranches))
	for _, branch := range protectedBranches {
		if branch == nil || branch.Name == "" || branch.ID <= 0 {
			continue
		}
		protectedBranchIDByName[branch.Name] = branch.ID
		protectedBranchNames = append(protectedBranchNames, branch.Name)
	}

	log.Debug().
		Str("project", project.Name).
		Strs("protectedBranches", protectedBranchNames).
		Msg("protected branches found")

	existingRules, _, err := c.Projects.GetProjectApprovalRules(project.ID, nil)
	if err != nil {
		return fmt.Errorf("cannot list existing project approval rules: %w", err)
	}

	existingByName := make(map[string]*gitlab.ProjectApprovalRule, len(existingRules))
	var existingAnyApproverRule *gitlab.ProjectApprovalRule
	for _, rule := range existingRules {
		if rule == nil {
			continue
		}
		if rule.RuleType == "any_approver" && existingAnyApproverRule == nil {
			existingAnyApproverRule = rule
		}
		if strings.TrimSpace(rule.Name) == "" {
			continue
		}
		existingByName[rule.Name] = rule
	}

	var anyApproverRuleName string
	var anyApproverApprovalsRequired int64
	var anyApproverConfigured bool
	anyApproverBranchIDByName := make(map[string]int64)

	for _, configuredRule := range assignmentCfg.MergeRequest.Approvals {
		if !approvalRuleAppliesForMemberCount(configuredRule, assignmentCfg.Per, memberCount) {
			for _, branchName := range configuredRule.Branches {
				ruleName := approvalRuleName(configuredRule, branchName)
				if existingRule, ok := existingByName[ruleName]; ok {
					_, err := c.Projects.DeleteProjectApprovalRule(project.ID, existingRule.ID)
					if err != nil {
						return fmt.Errorf("cannot delete approval rule %q for branch %q: %w", ruleName, branchName, err)
					}
					delete(existingByName, ruleName)
				}
			}
			continue
		}

		usernames, err := c.resolveApprovalUsernames(configuredRule.Usernames)
		if err != nil {
			return fmt.Errorf("cannot resolve usernames for approval rule %q: %w", configuredRule.Name, err)
		}
		groupIDs, err := c.resolveApprovalGroupIDs(configuredRule.Groups)
		if err != nil {
			return fmt.Errorf("cannot resolve groups for approval rule %q: %w", configuredRule.Name, err)
		}

		log.Debug().
			Str("project", project.Name).
			Str("rule", configuredRule.Name).
			Strs("branches", configuredRule.Branches).
			Msg("processing approval rule branches")

		for _, branchName := range configuredRule.Branches {
			protectedBranchID, ok := protectedBranchIDByName[branchName]
			if !ok {
				msg := fmt.Sprintf("skipping approval rule %q for branch %q in project %s: branch is not protected (configure it in branches with protect or mergeOnly)", configuredRule.Name, branchName, project.Name)
				fmt.Printf("warning: %s\n", msg)
				log.Warn().Str("project", project.Name).Str("rule", configuredRule.Name).Str("branch", branchName).Msg("skipping merge request approval rule for unprotected branch")
				continue
			}

			ruleName := approvalRuleName(configuredRule, branchName)
			if ruleName == "" {
				return fmt.Errorf("approval rule for branch %q has no name", branchName)
			}

			branchIDs := []int64{protectedBranchID}
			clearRule := configuredRule.RequiredApprovals == 0 && len(usernames) == 0 && len(groupIDs) == 0
			if clearRule {
				if existingRule, ok := existingByName[ruleName]; ok {
					_, err := c.Projects.DeleteProjectApprovalRule(project.ID, existingRule.ID)
					if err != nil {
						return fmt.Errorf("cannot delete approval rule %q for branch %q: %w", ruleName, branchName, err)
					}
					delete(existingByName, ruleName)
				}
				continue
			}

			// Sammle alle any-approver Regeln (ohne usernames/groups) für alle Branches
			if len(usernames) == 0 && len(groupIDs) == 0 {
				approvalsRequired := int64(configuredRule.RequiredApprovals)
				if !anyApproverConfigured {
					anyApproverConfigured = true
					anyApproverRuleName = ruleName
					anyApproverApprovalsRequired = approvalsRequired
				} else if anyApproverApprovalsRequired != approvalsRequired {
					return fmt.Errorf("cannot configure multiple any-approver rules with different requiredApprovals (%d vs %d)", anyApproverApprovalsRequired, approvalsRequired)
				}
				anyApproverBranchIDByName[branchName] = protectedBranchID
				continue
			}

			approvalsRequired := int64(configuredRule.RequiredApprovals)
			createOpts := &gitlab.CreateProjectLevelRuleOptions{
				Name:                          gitlab.Ptr(ruleName),
				ApprovalsRequired:             gitlab.Ptr(approvalsRequired),
				Usernames:                     &usernames,
				GroupIDs:                      &groupIDs,
				ProtectedBranchIDs:            &branchIDs,
				AppliesToAllProtectedBranches: gitlab.Ptr(false),
			}

			if existingRule, ok := existingByName[ruleName]; ok {
				updateOpts := &gitlab.UpdateProjectLevelRuleOptions{
					Name:                          gitlab.Ptr(ruleName),
					ApprovalsRequired:             gitlab.Ptr(approvalsRequired),
					Usernames:                     &usernames,
					GroupIDs:                      &groupIDs,
					ProtectedBranchIDs:            &branchIDs,
					AppliesToAllProtectedBranches: gitlab.Ptr(false),
				}
				_, _, err = c.Projects.UpdateProjectApprovalRule(project.ID, existingRule.ID, updateOpts)
				if err != nil {
					return fmt.Errorf("cannot update approval rule %q for branch %q: %w", ruleName, branchName, err)
				}
				continue
			}

			_, _, err = c.Projects.CreateProjectApprovalRule(project.ID, createOpts)
			if err != nil {
				return fmt.Errorf("cannot create approval rule %q for branch %q: %w", ruleName, branchName, err)
			}
		}
	}

	if anyApproverConfigured {
		branchNames := make([]string, 0, len(anyApproverBranchIDByName))
		for branchName := range anyApproverBranchIDByName {
			branchNames = append(branchNames, branchName)
		}
		sort.Strings(branchNames)

		branchIDs := make([]int64, 0, len(branchNames))
		for _, branchName := range branchNames {
			branchIDs = append(branchIDs, anyApproverBranchIDByName[branchName])
		}

		if existingAnyApproverRule != nil {
			updateOpts := &gitlab.UpdateProjectLevelRuleOptions{
				Name:                          gitlab.Ptr(anyApproverRuleName),
				ApprovalsRequired:             gitlab.Ptr(anyApproverApprovalsRequired),
				ProtectedBranchIDs:            &branchIDs,
				AppliesToAllProtectedBranches: gitlab.Ptr(false),
			}
			_, _, err = c.Projects.UpdateProjectApprovalRule(project.ID, existingAnyApproverRule.ID, updateOpts)
			if err != nil {
				return fmt.Errorf("cannot update any-approver approval rule for branches %v: %w", branchNames, err)
			}
		} else {
			createOpts := &gitlab.CreateProjectLevelRuleOptions{
				Name:                          gitlab.Ptr(anyApproverRuleName),
				ApprovalsRequired:             gitlab.Ptr(anyApproverApprovalsRequired),
				RuleType:                      gitlab.Ptr("any_approver"),
				ProtectedBranchIDs:            &branchIDs,
				AppliesToAllProtectedBranches: gitlab.Ptr(false),
			}
			_, _, err = c.Projects.CreateProjectApprovalRule(project.ID, createOpts)
			if err != nil {
				return fmt.Errorf("cannot create any-approver approval rule for branches %v: %w", branchNames, err)
			}
		}
	}

	return nil
}

func (c *Client) applyMergeRequestApprovalSettings(settings *config.MergeRequestApprovalSettings, project *gitlab.Project) error {
	if settings == nil {
		return nil
	}

	opts := &gitlab.ChangeApprovalConfigurationOptions{}
	configured := false

	if settings.PreventApprovalByMergeRequestCreator != nil {
		v := !*settings.PreventApprovalByMergeRequestCreator
		opts.MergeRequestsAuthorApproval = &v
		configured = true
	}
	if settings.PreventApprovalsByUsersWhoAddCommits != nil {
		v := *settings.PreventApprovalsByUsersWhoAddCommits
		opts.MergeRequestsDisableCommittersApproval = &v
		configured = true
	}
	if settings.PreventEditingApprovalRulesInMergeRequests != nil {
		v := *settings.PreventEditingApprovalRulesInMergeRequests
		opts.DisableOverridingApproversPerMergeRequest = &v
		configured = true
	}
	if settings.RequireUserReauthenticationToApprove != nil {
		v := *settings.RequireUserReauthenticationToApprove
		opts.RequireReauthenticationToApprove = &v
		configured = true
	}
	if settings.WhenCommitAdded != nil {
		switch *settings.WhenCommitAdded {
		case config.ApprovalKeepApprovals:
			reset := false
			selective := false
			opts.ResetApprovalsOnPush = &reset
			opts.SelectiveCodeOwnerRemovals = &selective
		case config.ApprovalRemoveAllApprovals:
			reset := true
			selective := false
			opts.ResetApprovalsOnPush = &reset
			opts.SelectiveCodeOwnerRemovals = &selective
		case config.ApprovalRemoveCodeOwnerApprovalsIfFilesChanged:
			reset := false
			selective := true
			opts.ResetApprovalsOnPush = &reset
			opts.SelectiveCodeOwnerRemovals = &selective
		default:
			return fmt.Errorf("unsupported approval setting whenCommitAdded=%q", *settings.WhenCommitAdded)
		}
		configured = true
	}

	if !configured {
		return nil
	}

	_, _, err := c.Projects.ChangeApprovalConfiguration(project.ID, opts)
	if err != nil {
		return fmt.Errorf("cannot set merge request approval settings for project %s: %w", project.Name, err)
	}

	return nil
}

func approvalRuleName(rule config.MergeRequestApprovalRule, branchName string) string {
	if rule.Name == "" {
		return fmt.Sprintf("glabs-approval-%s", branchName)
	}
	if len(rule.Branches) <= 1 {
		return rule.Name
	}
	return fmt.Sprintf("%s-%s", rule.Name, branchName)
}

func approvalRuleAppliesForMemberCount(rule config.MergeRequestApprovalRule, per config.Per, memberCount int) bool {
	if !rule.MultiMemberGroupsOnly {
		return true
	}
	if per != config.PerGroup {
		return false
	}
	return memberCount > 1
}

func (c *Client) resolveApprovalUsernames(identifiers []string) ([]string, error) {
	usernames := make([]string, 0, len(identifiers))
	seenUsernames := make(map[string]struct{})

	for _, identifier := range identifiers {
		identifier = strings.TrimSpace(identifier)
		if identifier == "" {
			continue
		}

		if _, err := mail.ParseAddress(identifier); err == nil {
			return nil, fmt.Errorf("email %q is not supported for mergeRequest.approvals.rules[].usernames; use username", identifier)
		}

		if isNumericIdentifier(identifier) {
			return nil, fmt.Errorf("numeric id %q is not supported for mergeRequest.approvals.rules[].usernames; use username", identifier)
		}

		username := strings.TrimSpace(strings.TrimPrefix(identifier, "@"))
		if username == "" {
			return nil, fmt.Errorf("invalid username %q in mergeRequest.approvals.rules[].usernames", identifier)
		}
		if _, ok := seenUsernames[username]; ok {
			continue
		}
		seenUsernames[username] = struct{}{}
		usernames = append(usernames, username)
	}

	return usernames, nil
}

func (c *Client) resolveApprovalGroupIDs(identifiers []string) ([]int64, error) {
	ids := make([]int64, 0, len(identifiers))
	seen := make(map[int64]struct{})

	for _, identifier := range identifiers {
		var id int64
		if isNumericIdentifier(identifier) {
			parsed, _ := strconv.ParseInt(identifier, 10, 64)
			id = parsed
		} else {
			resolved, err := c.getGroupIDByFullPath(identifier)
			if err != nil {
				return nil, fmt.Errorf("cannot resolve group %q: %w", identifier, err)
			}
			id = resolved
		}

		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, nil
}

func isNumericIdentifier(value string) bool {
	if strings.HasPrefix(value, "0") {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}
