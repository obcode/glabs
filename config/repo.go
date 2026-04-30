package config

import (
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func startercode(assignmentKey string) *Startercode {
	startercodeMap := viper.GetStringMapString(assignmentKey + ".startercode")

	if len(startercodeMap) == 0 {
		log.Debug().Str("assignmemtKey", assignmentKey).Msg("no startercode provided")
		return nil
	}

	url, ok := startercodeMap["url"]
	if !ok {
		log.Fatal().Str("assignmemtKey", assignmentKey).Msg("startercode provided without url")
		return nil
	}

	fromBranch := "main"
	if fB := viper.GetString(assignmentKey + ".startercode.fromBranch"); len(fB) > 0 {
		fromBranch = fB
	}

	template := viper.GetBool(assignmentKey + ".startercode.template")

	templateMessage := "Initial"
	if tM := viper.GetString(assignmentKey + ".startercode.templateMessage"); len(tM) > 0 {
		templateMessage = tM
	}

	toBranch := "main"
	if tB := viper.GetString(assignmentKey + ".startercode.toBranch"); len(tB) > 0 {
		toBranch = tB
	}

	additionalBranches := viper.GetStringSlice(assignmentKey + ".startercode.additionalBranches")

	return &Startercode{
		URL:                url,
		FromBranch:         fromBranch,
		Template:           template,
		TemplateMessage:    templateMessage,
		ToBranch:           toBranch,
		AdditionalBranches: additionalBranches,
	}
}

func branches(assignmentKey string, starter *Startercode) []BranchRule {
	var configured []BranchRule
	raw := normalizeBranchRuleConfigKeys(viper.Get(assignmentKey + ".branches"))
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &configured,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	})
	if err != nil {
		log.Fatal().Err(err).Str("assignmentKey", assignmentKey).Msg("cannot create branches decoder")
	}
	if err := decoder.Decode(raw); err != nil {
		log.Fatal().Err(err).Str("assignmentKey", assignmentKey).Msg("cannot parse branches config")
	}

	rules := make([]BranchRule, 0)
	seen := make(map[string]int)
	appendOrMerge := func(rule BranchRule) {
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			return
		}
		if idx, ok := seen[rule.Name]; ok {
			rules[idx].Protect = rules[idx].Protect || rule.Protect
			rules[idx].MergeOnly = rules[idx].MergeOnly || rule.MergeOnly
			rules[idx].Default = rules[idx].Default || rule.Default
			rules[idx].AllowForcePush = rules[idx].AllowForcePush || rule.AllowForcePush
			rules[idx].CodeOwnerApprovalRequired = rules[idx].CodeOwnerApprovalRequired || rule.CodeOwnerApprovalRequired
			return
		}
		seen[rule.Name] = len(rules)
		rules = append(rules, rule)
	}

	for _, rule := range configured {
		appendOrMerge(rule)
	}

	if len(rules) == 0 {
		if starter != nil {
			appendOrMerge(BranchRule{Name: starter.ToBranch, Default: true})
		}

		// Legacy compatibility for old startercode-based branch config.
		legacyDevBranch := viper.GetString(assignmentKey + ".startercode.devBranch")
		if legacyDevBranch != "" {
			appendOrMerge(BranchRule{Name: legacyDevBranch, Default: true})
		}

		for _, branchName := range viper.GetStringSlice(assignmentKey + ".startercode.additionalBranches") {
			appendOrMerge(BranchRule{Name: branchName})
		}

		if viper.GetBool(assignmentKey+".startercode.protectToBranch") && starter != nil {
			appendOrMerge(BranchRule{Name: starter.ToBranch, Protect: true})
		}

		if viper.GetBool(assignmentKey + ".startercode.protectDevBranchMergeOnly") {
			legacyTarget := legacyDevBranch
			if legacyTarget == "" && starter != nil {
				legacyTarget = starter.ToBranch
			}
			appendOrMerge(BranchRule{Name: legacyTarget, MergeOnly: true})
		}
	}

	hasDefault := false
	for _, rule := range rules {
		if rule.Default {
			hasDefault = true
			break
		}
	}

	if !hasDefault && len(rules) > 0 {
		rules[0].Default = true
	}

	return rules
}

func normalizeBranchRuleConfigKeys(value any) any {
	switch typed := value.(type) {
	case []any:
		normalized := make([]any, len(typed))
		for i, item := range typed {
			normalized[i] = normalizeBranchRuleConfigKeys(item)
		}
		return normalized
	case []map[string]any:
		normalized := make([]any, len(typed))
		for i, item := range typed {
			normalized[i] = normalizeBranchRuleConfigKeys(item)
		}
		return normalized
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			switch key {
			case "allow_force_push":
				key = "allowForcePush"
			case "code_owner_approval_required":
				key = "codeOwnerApprovalRequired"
			}
			normalized[key] = normalizeBranchRuleConfigKeys(item)
		}
		return normalized
	default:
		return value
	}
}

func defaultBranch(rules []BranchRule, fallback string) string {
	for _, rule := range rules {
		if rule.Default && rule.Name != "" {
			return rule.Name
		}
	}
	if fallback != "" {
		return fallback
	}
	if len(rules) > 0 {
		return rules[0].Name
	}
	return "main"
}

func issues(assignmentKey string) *IssueReplication {
	replicate := viper.GetBool(assignmentKey + ".issues.replicateFromStartercode")
	numbers := viper.GetIntSlice(assignmentKey + ".issues.issueNumbers")

	// Legacy compatibility for old startercode issue replication config.
	if !replicate && !viper.IsSet(assignmentKey+".issues") {
		replicate = viper.GetBool(assignmentKey + ".startercode.replicateIssue")
		numbers = viper.GetIntSlice(assignmentKey + ".startercode.issueNumbers")
	}

	if !replicate {
		return &IssueReplication{ReplicateFromStartercode: false}
	}

	if len(numbers) == 0 {
		numbers = []int{1}
	}

	return &IssueReplication{ReplicateFromStartercode: true, IssueNumbers: numbers}
}

func clone(assignmentKey, defaultBranch string) *Clone {
	cloneMap := viper.GetStringMapString(assignmentKey + ".clone")

	localpath, ok := cloneMap["localpath"]
	if !ok {
		localpath = "."
	}

	branch, ok := cloneMap["branch"]
	if !ok {
		branch = defaultBranch
	}

	force := viper.GetBool(assignmentKey + ".clone.force")

	return &Clone{
		LocalPath: localpath,
		Branch:    branch,
		Force:     force,
	}
}

func (cfg *AssignmentConfig) SetBranch(branch string) {
	cfg.Clone.Branch = branch
}

func (cfg *AssignmentConfig) SetProtectToBranch(branch string) {
	if branch == "" && len(cfg.Branches) > 0 {
		branch = cfg.Branches[0].Name
	}
	if branch == "" {
		branch = "main"
	}

	for i := range cfg.Branches {
		if cfg.Branches[i].Name == branch {
			cfg.Branches[i].Protect = true
			return
		}
	}

	cfg.Branches = append(cfg.Branches, BranchRule{Name: branch, Protect: true})
}

func (cfg *AssignmentConfig) SetLocalpath(localpath string) {
	cfg.Clone.LocalPath = localpath
}

func (cfg *AssignmentConfig) SetForce() {
	cfg.Clone.Force = true
}
