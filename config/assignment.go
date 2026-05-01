package config

import (
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func GetCourseURL(course string) {
	fmt.Printf("%s/%s\n", viper.GetString("gitlab.host"), coursePath(course))
}

func GetAssignmentConfig(course, assignment string, onlyForStudentsOrGroups ...string) *AssignmentConfig {
	if !viper.IsSet(course) {
		log.Fatal().
			Str("course", course).
			Msg("configuration for course not found")
	}

	if !viper.IsSet(course + "." + assignment) {
		log.Fatal().
			Str("course", course).
			Str("assignment", assignment).
			Msg("configuration for assignment not found")
	}

	assignmentKey := course + "." + assignment
	per := per(assignmentKey)

	path := assignmentPath(course, assignment)
	url := viper.GetString("gitlab.host") + "/" + path

	containerRegistry := viper.GetBool(assignmentKey + ".containerRegistry")
	release := release(assignmentKey)
	if release != nil && release.DockerImages != nil {
		containerRegistry = true
	}

	starter := startercode(assignmentKey)
	branchRules := branches(assignmentKey, starter)
	defaultCloneBranch := defaultBranch(branchRules, "main")

	deferredBranches := make(map[string]*DeferredBranch)
	deferredBranchesCfg := viper.GetStringMap(assignmentKey + ".deferredBranches")
	if len(deferredBranchesCfg) > 0 {
		for name := range deferredBranchesCfg {
			configMap := viper.GetStringMapString(assignmentKey + ".deferredBranches." + name)

			url, ok := configMap["url"]
			if !ok {
				url = starter.URL
			}
			fromBranch := configMap["frombranch"]
			toBranch, ok := configMap["tobranch"]
			if !ok {
				toBranch = fromBranch
			}
			orphanValue, ok := configMap["orphan"]
			orphan := true
			if ok && orphanValue == "false" {
				orphan = false
			}

			orphanMessage, ok := configMap["orphanmessage"]
			if !ok {
				orphanMessage = fmt.Sprintf("Snapshot of %s", name)
			}

			deferredBranches[name] = &DeferredBranch{
				URL:           url,
				FromBranch:    fromBranch,
				ToBranch:      toBranch,
				Orphan:        orphan,
				OrphanMessage: orphanMessage,
			}
		}
	}

	assignmentConfig := &AssignmentConfig{
		Course:                course,
		Name:                  assignment,
		UseCoursenameAsPrefix: viper.GetBool(course + ".useCoursenameAsPrefix"),
		UseEmailDomainAsSuffix: func() bool {
			if viper.IsSet(course + ".useEmailDomainAsSuffix") {
				return viper.GetBool(course + ".useEmailDomainAsSuffix")
			}
			return true // default
		}(),
		Path:              path,
		URL:               url,
		Per:               per,
		Description:       description(assignmentKey),
		ContainerRegistry: containerRegistry,
		AccessLevel:       accessLevel(assignmentKey),
		MergeRequest:      mergeRequest(assignmentKey),
		Branches:          branchRules,
		Issues:            issues(assignmentKey),
		Students:          students(per, course, assignment, onlyForStudentsOrGroups...),
		Groups:            groups(per, course, assignment, onlyForStudentsOrGroups...),
		Startercode:       starter,
		Clone:             clone(assignmentKey, defaultCloneBranch),
		Release:           release,
		Seeder:            seeder(assignmentKey),
		DeferredBranches:  deferredBranches,
	}

	return assignmentConfig
}

// Using email addresses instead of usernames/user-id's results in @ in the student's name.
// This is incompatible to the filesystem and gitlab so replacing the values is necessary.
func (cfg *AssignmentConfig) RepoSuffix(student *Student) string {
	if student.Email != nil {
		// If explicitly set to false, always use part before @
		if viper.IsSet(cfg.Course+".useEmailDomainAsSuffix") && !cfg.UseEmailDomainAsSuffix {
			parts := strings.SplitN(*student.Email, "@", 2)
			return parts[0]
		}
		// Default: use _at_ replacement
		return strings.ReplaceAll(*student.Email, "@", "_at_")
	}
	if student.Id != nil {
		return fmt.Sprint(*student.Id)
	}
	if student.Username != nil {
		return *student.Username
	}
	return ""
}

func (cfg *AssignmentConfig) RepoBaseName() string {
	if cfg.UseCoursenameAsPrefix {
		return fmt.Sprintf("%s-%s", cfg.Course, cfg.Name)
	}

	return cfg.Name
}

func (cfg *AssignmentConfig) RepoNameWithSuffix(suffix string) string {
	return fmt.Sprintf("%s-%s", cfg.RepoBaseName(), suffix)
}

func (cfg *AssignmentConfig) RepoNameForStudent(student *Student) string {
	return cfg.RepoNameWithSuffix(cfg.RepoSuffix(student))
}

func (cfg *AssignmentConfig) RepoNameForGroup(group *Group) string {
	return cfg.RepoNameWithSuffix(group.Name)
}

func coursePath(course string) string {
	path := viper.GetString(course + ".coursepath")
	if semesterpath := viper.GetString(course + ".semesterpath"); len(semesterpath) > 0 {
		path += "/" + semesterpath
	}

	return path
}

func assignmentPath(course, assignment string) string {
	path := coursePath(course)

	assignmentpath := path
	if group := viper.GetString(course + "." + assignment + ".assignmentpath"); len(group) > 0 {
		assignmentpath += "/" + group
	}

	return assignmentpath
}

func per(assignmentKey string) Per {
	if per := viper.GetString(assignmentKey + ".per"); per == "group" {
		return PerGroup
	}
	return PerStudent
}

func description(assignmentKey string) string {
	description := "generated by glabs"

	if desc := viper.GetString(assignmentKey + ".description"); desc != "" {
		description = desc
	}

	return description
}

func mergeRequest(assignmentKey string) *MergeRequest {
	mergeMethod := MergeCommit
	switch viper.GetString(assignmentKey + ".mergeRequest.mergeMethod") {
	case "semi_linear":
		mergeMethod = SemiLinearHistory
	case "ff":
		mergeMethod = FastForward
	case "merge":
		mergeMethod = MergeCommit
	}

	squashOption := SquashDefaultOff
	switch viper.GetString(assignmentKey + ".mergeRequest.squashOption") {
	case "never":
		squashOption = SquashNever
	case "always":
		squashOption = SquashAlways
	case "default_on":
		squashOption = SquashDefaultOn
	case "default_off":
		squashOption = SquashDefaultOff
	}

	return &MergeRequest{
		MergeMethod:                   mergeMethod,
		SquashOption:                  squashOption,
		PipelineMustSucceed:           viper.GetBool(assignmentKey + ".mergeRequest.pipeline"),
		SkippedPipelinesAreSuccessful: viper.GetBool(assignmentKey + ".mergeRequest.skippedPipelinesAreSuccessful"),
		AllThreadsMustBeResolved:      viper.GetBool(assignmentKey + ".mergeRequest.allThreadsMustBeResolved"),
		StatusChecksMustSucceed:       viper.GetBool(assignmentKey + ".mergeRequest.statusChecksMustSucceed"),
		Approvals:                     mergeRequestApprovals(assignmentKey),
		ApprovalSettings:              mergeRequestApprovalSettings(assignmentKey),
	}
}

func mergeRequestApprovals(assignmentKey string) []MergeRequestApprovalRule {
	raw := viper.Get(assignmentKey + ".mergeRequest.approvals")
	raw = extractMergeRequestApprovalRulesRaw(raw)
	raw = normalizeMergeRequestApprovalConfigKeys(raw)
	if raw == nil {
		return nil
	}

	if containsLegacyApprovalUsersKey(raw) {
		log.Fatal().Str("assignmentKey", assignmentKey).Msg("mergeRequest.approvals.rules[].users is no longer supported; use usernames")
	}

	var configured []MergeRequestApprovalRule
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &configured,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	})
	if err != nil {
		log.Fatal().Err(err).Str("assignmentKey", assignmentKey).Msg("cannot create mergeRequest.approvals decoder")
	}
	if err := decoder.Decode(raw); err != nil {
		log.Fatal().Err(err).Str("assignmentKey", assignmentKey).Msg("cannot parse mergeRequest.approvals config")
	}

	normalized := make([]MergeRequestApprovalRule, 0, len(configured))
	for _, rule := range configured {
		rule.Name = strings.TrimSpace(rule.Name)
		rule.Branch = strings.TrimSpace(rule.Branch)
		if rule.Branch != "" {
			rule.Branches = append(rule.Branches, rule.Branch)
		}

		seenBranches := make(map[string]struct{})
		branches := make([]string, 0, len(rule.Branches))
		for _, branch := range rule.Branches {
			branch = strings.TrimSpace(branch)
			if branch == "" {
				continue
			}
			if _, ok := seenBranches[branch]; ok {
				continue
			}
			seenBranches[branch] = struct{}{}
			branches = append(branches, branch)
		}
		rule.Branches = branches
		rule.Branch = ""

		usernames := make([]string, 0, len(rule.Usernames))
		for _, username := range rule.Usernames {
			username = strings.TrimSpace(username)
			if username != "" {
				usernames = append(usernames, username)
			}
		}
		rule.Usernames = usernames

		groups := make([]string, 0, len(rule.Groups))
		for _, group := range rule.Groups {
			group = strings.TrimSpace(group)
			if group != "" {
				groups = append(groups, group)
			}
		}
		rule.Groups = groups

		if rule.RequiredApprovals < 0 {
			rule.RequiredApprovals = 0
		}

		if len(rule.Branches) == 0 {
			continue
		}
		normalized = append(normalized, rule)
	}

	return normalized
}

func extractMergeRequestApprovalRulesRaw(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if rules, ok := typed["rules"]; ok {
			return rules
		}
		return nil
	default:
		// Backward compatibility: approvals can still be configured directly as a list.
		return value
	}
}

func containsLegacyApprovalUsersKey(value any) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if containsLegacyApprovalUsersKey(item) {
				return true
			}
		}
		return false
	case []map[string]any:
		for _, item := range typed {
			if containsLegacyApprovalUsersKey(item) {
				return true
			}
		}
		return false
	case map[string]any:
		for key, item := range typed {
			if key == "users" {
				return true
			}
			if containsLegacyApprovalUsersKey(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func mergeRequestApprovalSettings(assignmentKey string) *MergeRequestApprovalSettings {
	prefix := assignmentKey + ".mergeRequest.approvals.settings"
	if !viper.IsSet(prefix) {
		return nil
	}

	settings := &MergeRequestApprovalSettings{}

	if viper.IsSet(prefix + ".preventApprovalByMergeRequestCreator") {
		v := viper.GetBool(prefix + ".preventApprovalByMergeRequestCreator")
		settings.PreventApprovalByMergeRequestCreator = &v
	}
	if viper.IsSet(prefix + ".preventApprovalsByUsersWhoAddCommits") {
		v := viper.GetBool(prefix + ".preventApprovalsByUsersWhoAddCommits")
		settings.PreventApprovalsByUsersWhoAddCommits = &v
	}
	if viper.IsSet(prefix + ".preventEditingApprovalRulesInMergeRequests") {
		v := viper.GetBool(prefix + ".preventEditingApprovalRulesInMergeRequests")
		settings.PreventEditingApprovalRulesInMergeRequests = &v
	}
	if viper.IsSet(prefix + ".requireUserReauthenticationToApprove") {
		v := viper.GetBool(prefix + ".requireUserReauthenticationToApprove")
		settings.RequireUserReauthenticationToApprove = &v
	}
	if viper.IsSet(prefix + ".whenCommitAdded") {
		v := ApprovalWhenCommitAdded(viper.GetString(prefix + ".whenCommitAdded"))
		switch v {
		case ApprovalKeepApprovals, ApprovalRemoveAllApprovals, ApprovalRemoveCodeOwnerApprovalsIfFilesChanged:
			settings.WhenCommitAdded = &v
		default:
			log.Fatal().
				Str("assignmentKey", assignmentKey).
				Str("whenCommitAdded", string(v)).
				Msg("invalid mergeRequest.approvals.settings.whenCommitAdded")
		}
	}

	if settings.PreventApprovalByMergeRequestCreator == nil &&
		settings.PreventApprovalsByUsersWhoAddCommits == nil &&
		settings.PreventEditingApprovalRulesInMergeRequests == nil &&
		settings.RequireUserReauthenticationToApprove == nil &&
		settings.WhenCommitAdded == nil {
		return nil
	}

	return settings
}

func normalizeMergeRequestApprovalConfigKeys(value any) any {
	switch typed := value.(type) {
	case []any:
		normalized := make([]any, len(typed))
		for i, item := range typed {
			normalized[i] = normalizeMergeRequestApprovalConfigKeys(item)
		}
		return normalized
	case []map[string]any:
		normalized := make([]any, len(typed))
		for i, item := range typed {
			normalized[i] = normalizeMergeRequestApprovalConfigKeys(item)
		}
		return normalized
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			switch key {
			case "required_approvals", "approvalsRequired":
				key = "requiredApprovals"
			}
			normalized[key] = normalizeMergeRequestApprovalConfigKeys(item)
		}
		return normalized
	default:
		return value
	}
}
