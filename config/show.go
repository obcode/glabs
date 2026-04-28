package config

import (
	"fmt"
	"strings"

	"github.com/logrusorgru/aurora/v4"
)

func (cfg *AssignmentConfig) Show() {
	var out strings.Builder

	maxLabelWidth := func(labels ...string) int {
		width := 0
		for _, label := range labels {
			if len(label)+1 > width {
				width = len(label) + 1
			}
		}
		return width
	}

	fieldValueColumn := func(candidates ...int) int {
		width := 0
		for _, candidate := range candidates {
			if candidate > width {
				width = candidate
			}
		}
		return width
	}

	fieldCandidate := func(indent int, label string) int {
		return indent + len(label) + 2
	}

	valueColumn := 0

	approvalSettingsLabelWidth := maxLabelWidth(
		"PreventApprovalByMergeRequestCreator",
		"PreventApprovalsByUsersWhoAddCommits",
		"PreventEditingApprovalRulesInMergeRequests",
		"RequireUserReauthenticationToApprove",
		"WhenCommitAdded",
	)

	writeTopField := func(label string, value any) {
		labelWidth := valueColumn - 1
		fmt.Fprintf(&out, "%-*v %v\n", labelWidth, aurora.Cyan(label+":"), aurora.Yellow(value))
	}

	writeSectionHeader := func(name string) {
		fmt.Fprintf(&out, "%s\n", aurora.Cyan(name+":"))
	}

	writeSectionField := func(label string, value any) {
		labelWidth := valueColumn - 3
		fmt.Fprintf(&out, "  %-*v %v\n", labelWidth, aurora.Cyan(label+":"), aurora.Yellow(value))
	}

	writeIndentedHeader := func(indent int, label string) {
		fmt.Fprintf(&out, "%s%s\n", strings.Repeat(" ", indent), aurora.Cyan(label+":"))
	}

	writeIndentedField := func(indent int, labelWidth int, label string, value any) {
		adjustedWidth := valueColumn - indent - 1
		if adjustedWidth < labelWidth {
			adjustedWidth = labelWidth
		}
		fmt.Fprintf(&out, "%s%-*v %v\n", strings.Repeat(" ", indent), adjustedWidth, aurora.Cyan(label+":"), aurora.Yellow(value))
	}

	writeSectionNotDefined := func() {
		fmt.Fprintf(&out, "  %s\n", aurora.Red("not defined"))
	}

	writeIndentedNotDefined := func(indent int) {
		fmt.Fprintf(&out, "%s%s\n", strings.Repeat(" ", indent), aurora.Red("not defined"))
	}

	mergeMethod := MergeCommit
	squashOption := SquashDefaultOff
	pipelineMustSucceed := false
	skippedPipelinesAreSuccessful := false
	allThreadsMustBeResolved := false
	statusChecksMustSucceed := false
	var approvals []MergeRequestApprovalRule
	var approvalSettings *MergeRequestApprovalSettings
	if cfg.MergeRequest != nil {
		mergeMethod = cfg.MergeRequest.MergeMethod
		squashOption = cfg.MergeRequest.SquashOption
		pipelineMustSucceed = cfg.MergeRequest.PipelineMustSucceed
		skippedPipelinesAreSuccessful = cfg.MergeRequest.SkippedPipelinesAreSuccessful
		allThreadsMustBeResolved = cfg.MergeRequest.AllThreadsMustBeResolved
		statusChecksMustSucceed = cfg.MergeRequest.StatusChecksMustSucceed
		approvals = cfg.MergeRequest.Approvals
		approvalSettings = cfg.MergeRequest.ApprovalSettings
	}

	valueColumn = fieldValueColumn(
		fieldCandidate(0, "Course"),
		fieldCandidate(0, "Assignment"),
		fieldCandidate(0, "Coursename-Prefix"),
		fieldCandidate(0, "EmailDomain-Suffix"),
		fieldCandidate(0, "Per"),
		fieldCandidate(0, "Base-URL"),
		fieldCandidate(0, "Description"),
		fieldCandidate(0, "AccessLevel"),
		fieldCandidate(0, "Container-Registry"),
		fieldCandidate(2, "URL"),
		fieldCandidate(2, "FromBranch"),
		fieldCandidate(2, "ToBranch"),
		fieldCandidate(2, "AdditionalBranches"),
		fieldCandidate(2, "ReplicateFromStartercode"),
		fieldCandidate(2, "IssueNumbers"),
		fieldCandidate(2, "MergeMethod"),
		fieldCandidate(2, "SquashOption"),
		fieldCandidate(2, "PipelineMustSucceed"),
		fieldCandidate(2, "SkippedPipelinesAreSuccessful"),
		fieldCandidate(2, "AllThreadsMustBeResolved"),
		fieldCandidate(2, "StatusChecksMustSucceed"),
		fieldCandidate(2, "Command"),
		fieldCandidate(2, "Args"),
		fieldCandidate(2, "Author"),
		fieldCandidate(2, "EMail"),
		fieldCandidate(2, "ProtectToBranch"),
		fieldCandidate(2, "MergeRequest"),
		fieldCandidate(2, "MergeRequest.SourceBranch"),
		fieldCandidate(2, "MergeRequest.TargetBranch"),
		fieldCandidate(2, "MergeRequest.Pipeline"),
		fieldCandidate(2, "DockerImages"),
		fieldCandidate(2, "Localpath"),
		fieldCandidate(2, "Branch"),
		fieldCandidate(2, "Force"),
		fieldCandidate(6, "PreventApprovalByMergeRequestCreator"),
		fieldCandidate(6, "PreventApprovalsByUsersWhoAddCommits"),
		fieldCandidate(6, "PreventEditingApprovalRulesInMergeRequests"),
		fieldCandidate(6, "RequireUserReauthenticationToApprove"),
		fieldCandidate(6, "WhenCommitAdded"),
	)
	for _, branch := range cfg.Branches {
		valueColumn = fieldValueColumn(valueColumn, fieldCandidate(2, "- "+branch.Name))
	}
	for _, approval := range approvals {
		valueColumn = fieldValueColumn(valueColumn, fieldCandidate(6, "- "+approval.Name))
	}
	for _, grp := range cfg.Groups {
		valueColumn = fieldValueColumn(valueColumn, fieldCandidate(2, "- "+grp.Name))
	}

	containerRegistry := aurora.Red("disabled")
	if cfg.ContainerRegistry {
		containerRegistry = aurora.Green("enabled")
	}

	writeTopField("Course", cfg.Course)
	writeTopField("Assignment", cfg.Name)
	writeTopField("Coursename-Prefix", cfg.UseCoursenameAsPrefix)
	writeTopField("EmailDomain-Suffix", cfg.UseEmailDomainAsSuffix)
	writeTopField("Per", cfg.Per)
	writeTopField("Base-URL", cfg.URL)
	writeTopField("Description", cfg.Description)
	writeTopField("AccessLevel", cfg.AccessLevel.String())
	writeTopField("Container-Registry", containerRegistry)

	writeSectionHeader("Startercode")
	if cfg.Startercode == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("URL", cfg.Startercode.URL)
		writeSectionField("FromBranch", cfg.Startercode.FromBranch)
		writeSectionField("ToBranch", cfg.Startercode.ToBranch)
		writeSectionField("AdditionalBranches", cfg.Startercode.AdditionalBranches)
	}

	writeSectionHeader("Issues")
	if cfg.Issues == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("ReplicateFromStartercode", cfg.Issues.ReplicateFromStartercode)
		if cfg.Issues.ReplicateFromStartercode {
			writeSectionField("IssueNumbers", cfg.Issues.IssueNumbers)
		} else {
			writeSectionField("IssueNumbers", "not used")
		}
	}

	writeSectionHeader("Branches")
	if len(cfg.Branches) == 0 {
		writeSectionNotDefined()
	} else {
		for _, branch := range cfg.Branches {
			flags := make([]string, 0, 5)
			if branch.Protect {
				flags = append(flags, "protect")
			}
			if branch.MergeOnly {
				flags = append(flags, "mergeOnly")
			}
			if branch.Default {
				flags = append(flags, "default")
			}
			if branch.AllowForcePush {
				flags = append(flags, "allowForcePush")
			}
			if branch.CodeOwnerApprovalRequired {
				flags = append(flags, "codeOwnerApprovalRequired")
			}

			if len(flags) == 0 {
				writeSectionField("- "+branch.Name, "")
				continue
			}

			writeSectionField("- "+branch.Name, aurora.Cyan(strings.Join(flags, ", ")))
		}
	}

	writeSectionHeader("MergeRequest")
	writeSectionField("MergeMethod", mergeMethod)
	writeSectionField("SquashOption", squashOption)
	writeSectionField("PipelineMustSucceed", pipelineMustSucceed)
	writeSectionField("SkippedPipelinesAreSuccessful", skippedPipelinesAreSuccessful)
	writeSectionField("AllThreadsMustBeResolved", allThreadsMustBeResolved)
	writeSectionField("StatusChecksMustSucceed", statusChecksMustSucceed)
	writeIndentedHeader(2, "Approvals")
	writeIndentedHeader(4, "Settings")
	if approvalSettings == nil {
		writeIndentedNotDefined(6)
	} else {
		hasApprovalSetting := false
		if approvalSettings.PreventApprovalByMergeRequestCreator != nil {
			hasApprovalSetting = true
			writeIndentedField(6, approvalSettingsLabelWidth, "PreventApprovalByMergeRequestCreator", *approvalSettings.PreventApprovalByMergeRequestCreator)
		}
		if approvalSettings.PreventApprovalsByUsersWhoAddCommits != nil {
			hasApprovalSetting = true
			writeIndentedField(6, approvalSettingsLabelWidth, "PreventApprovalsByUsersWhoAddCommits", *approvalSettings.PreventApprovalsByUsersWhoAddCommits)
		}
		if approvalSettings.PreventEditingApprovalRulesInMergeRequests != nil {
			hasApprovalSetting = true
			writeIndentedField(6, approvalSettingsLabelWidth, "PreventEditingApprovalRulesInMergeRequests", *approvalSettings.PreventEditingApprovalRulesInMergeRequests)
		}
		if approvalSettings.RequireUserReauthenticationToApprove != nil {
			hasApprovalSetting = true
			writeIndentedField(6, approvalSettingsLabelWidth, "RequireUserReauthenticationToApprove", *approvalSettings.RequireUserReauthenticationToApprove)
		}
		if approvalSettings.WhenCommitAdded != nil {
			hasApprovalSetting = true
			writeIndentedField(6, approvalSettingsLabelWidth, "WhenCommitAdded", *approvalSettings.WhenCommitAdded)
		}
		if !hasApprovalSetting {
			writeIndentedNotDefined(6)
		}
	}
	writeIndentedHeader(4, "Rules")
	if len(approvals) == 0 {
		writeIndentedNotDefined(6)
	} else {
		for _, approval := range approvals {
			ruleParts := []string{
				fmt.Sprintf("%v=%v", aurora.Cyan("branches"), aurora.Yellow(approval.Branches)),
			}
			if len(approval.Usernames) > 0 {
				ruleParts = append(ruleParts, fmt.Sprintf("%v=%v", aurora.Cyan("usernames"), aurora.Yellow(approval.Usernames)))
			}
			if len(approval.Groups) > 0 {
				ruleParts = append(ruleParts, fmt.Sprintf("%v=%v", aurora.Cyan("groups"), aurora.Yellow(approval.Groups)))
			}
			if approval.MultiMemberGroupsOnly {
				ruleParts = append(ruleParts, fmt.Sprintf("%v=%v", aurora.Cyan("multiMemberGroupsOnly"), aurora.Yellow(true)))
			}
			ruleParts = append(ruleParts, fmt.Sprintf("%v=%v", aurora.Cyan("requiredApprovals"), aurora.Yellow(approval.RequiredApprovals)))

			writeIndentedField(6, 0, "- "+approval.Name, strings.Join(ruleParts, ", "))
		}
	}

	writeSectionHeader("Seeding")
	if cfg.Seeder == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("Command", cfg.Seeder.Command)
		writeSectionField("Args", cfg.Seeder.Args)
		writeSectionField("Author", cfg.Seeder.Name)
		writeSectionField("EMail", cfg.Seeder.EMail)
		writeSectionField("ToBranch", cfg.Seeder.ToBranch)
		writeSectionField("ProtectToBranch", cfg.Seeder.ProtectToBranch)
	}

	writeSectionHeader("Release")
	if cfg.Release == nil {
		writeSectionNotDefined()
	} else {
		if cfg.Release.MergeRequest == nil {
			writeSectionField("MergeRequest", "not defined")
		} else {
			writeSectionField("MergeRequest.SourceBranch", cfg.Release.MergeRequest.SourceBranch)
			writeSectionField("MergeRequest.TargetBranch", cfg.Release.MergeRequest.TargetBranch)
			writeSectionField("MergeRequest.Pipeline", cfg.Release.MergeRequest.HasPipeline)
		}
		if len(cfg.Release.DockerImages) == 0 {
			writeSectionField("DockerImages", "not defined")
		} else {
			writeSectionField("DockerImages", cfg.Release.DockerImages)
		}
	}

	writeSectionHeader("Clone")
	if cfg.Clone == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("Localpath", cfg.Clone.LocalPath)
		writeSectionField("Branch", cfg.Clone.Branch)
		writeSectionField("Force", cfg.Clone.Force)
	}

	switch cfg.Per {
	case PerStudent:
		writeSectionHeader("Students")
		if len(cfg.Students) == 0 {
			writeSectionNotDefined()
		} else {
			for _, s := range cfg.Students {
				fmt.Fprintf(&out, "  - %s\n", aurora.Yellow(s.Raw))
			}
		}
	case PerGroup:
		writeSectionHeader("Groups")
		if len(cfg.Groups) == 0 {
			writeSectionNotDefined()
		} else {
			for _, grp := range cfg.Groups {
				members := make([]string, 0, len(grp.Members))
				for _, m := range grp.Members {
					members = append(members, m.Raw)
				}
				fmt.Fprintf(
					&out,
					"  - %s %s: %s\n",
					aurora.Yellow(grp.Name),
					aurora.Cyan(fmt.Sprintf("(%d)", len(grp.Members))),
					aurora.Green(strings.Join(members, ", ")),
				)
			}
		}
	}

	fmt.Println(out.String())
}
