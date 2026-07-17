package config

import (
	"fmt"
	"sort"
	"strings"
)

// Resolution turns the source form into the effective one: `extends` applied,
// course-level students merged in, defaults filled, Path and URL computed.
//
// Unlike the viper-based loader this replaces, it is a pure function of its
// inputs — no global state, no writes, no process exit — so two courses can be
// resolved concurrently and a bad config is an error rather than a fatality.
//
// Inheritance is resolved on the raw map, not on decoded structs, and this is
// not an accident. A decoded child cannot express "I did not mention pipeline":
// its Pipeline field is false either way, and merging structs would let that
// false overwrite the parent's true. Absence only survives as a missing map key,
// so the merge has to happen before the struct decode. That is why Resolve takes
// the course body rather than a *CourseSource.

// Globals are the settings that live outside any course file.
type Globals struct {
	// GitlabHost is the base for the URLs glabs prints and searches for.
	GitlabHost string
}

// ResolveAssignment resolves one assignment out of a raw course body — the
// value under the course file's single top-level key.
//
// onlyForStudentsOrGroups filters students or groups by regexp, matching the
// CLI's positional arguments.
func ResolveAssignment(courseName string, body any, g Globals, assignment string, onlyForStudentsOrGroups ...string) (*AssignmentConfig, error) {
	courseMap, ok := asStringMap(body)
	if !ok {
		return nil, fmt.Errorf("configuration for course %s not found", courseName)
	}

	own, ok := lookupAssignment(courseMap, assignment)
	if !ok {
		return nil, fmt.Errorf("course %s: configuration for assignment %s not found", courseName, assignment)
	}

	// The abstract flag is read from the assignment's own map, before the merge,
	// so an inherited flag never makes a concrete child abstract.
	if truthy(own[abstractKey]) {
		return nil, fmt.Errorf("course %s: assignment %s is abstract (a base for 'extends') and cannot be used directly",
			courseName, assignment)
	}

	merged, err := mergeAssignment(courseMap, courseName, assignment, map[string]bool{})
	if err != nil {
		return nil, err
	}
	delete(merged, inheritKey)
	delete(merged, abstractKey)

	normalized, _ := normalizeLegacyKeys(merged, courseName+"."+assignment)

	// `users` was renamed to `usernames`. It has to be caught on the raw map:
	// the schema has no such field, so decoding would drop it silently and the
	// rule would apply to nobody.
	if containsApprovalUsersKey(normalized) {
		return nil, fmt.Errorf("course %s, assignment %s: mergeRequest.approvals.rules[].users is no longer supported; use usernames",
			courseName, assignment)
	}

	var src AssignmentSource
	if err := decodeInto(&src, normalized, nil); err != nil {
		return nil, fmt.Errorf("course %s, assignment %s: cannot decode configuration: %w", courseName, assignment, err)
	}

	course, err := decodeCourseSettings(courseName, courseMap)
	if err != nil {
		return nil, err
	}

	return buildAssignmentConfig(courseName, assignment, course, &src, g, onlyForStudentsOrGroups...)
}

// lookupAssignment finds an assignment by name, folding case the way the rest of
// the loader does.
func lookupAssignment(courseMap map[string]any, assignment string) (map[string]any, bool) {
	for key, value := range courseMap {
		if !strings.EqualFold(key, assignment) {
			continue
		}
		if reservedCourseKey(key) {
			return nil, false
		}
		m, ok := asStringMap(value)
		return m, ok
	}
	return nil, false
}

func reservedCourseKey(key string) bool {
	switch strings.ToLower(key) {
	case "coursepath", "semesterpath", "usecoursenameasprefix", "useemaildomainassuffix", "students", "groups":
		return true
	}
	return false
}

// containsApprovalUsersKey reports whether the tree still uses the `users` key,
// renamed to `usernames`. Unlike the other deprecated spellings this is not
// aliased but rejected: silently applying an approval rule to nobody is worse
// than refusing to load.
func containsApprovalUsersKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if strings.EqualFold(key, "users") {
				return true
			}
			if containsApprovalUsersKey(item) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsApprovalUsersKey(item) {
				return true
			}
		}
	}
	return false
}

// truthy mirrors how viper coerced a value to bool: YAML gives a real bool, but
// a value that arrived as a string still has to count.
func truthy(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

// mergeAssignment resolves the `extends` chain, returning the assignment's map
// with all inherited configuration merged in. The child's own values win.
func mergeAssignment(courseMap map[string]any, courseName, assignment string, seen map[string]bool) (map[string]any, error) {
	if seen[strings.ToLower(assignment)] {
		return nil, fmt.Errorf("course %s, assignment %s: cyclic 'extends' inheritance detected in assignment configuration",
			courseName, assignment)
	}
	seen[strings.ToLower(assignment)] = true

	own, ok := lookupAssignment(courseMap, assignment)
	if !ok {
		return nil, fmt.Errorf("course %s: configuration for assignment %s not found", courseName, assignment)
	}

	parentRaw, ok := own[inheritKey]
	if !ok {
		return own, nil
	}

	parent, ok := parentRaw.(string)
	if !ok || strings.TrimSpace(parent) == "" {
		return nil, fmt.Errorf("course %s, assignment %s: 'extends' must be the name of another assignment in the same course",
			courseName, assignment)
	}
	parent = strings.TrimSpace(parent)

	if _, ok := lookupAssignment(courseMap, parent); !ok {
		return nil, fmt.Errorf("course %s, assignment %s: assignment %q referenced by 'extends' not found",
			courseName, assignment, parent)
	}

	parentMap, err := mergeAssignment(courseMap, courseName, parent, seen)
	if err != nil {
		return nil, err
	}

	return deepMerge(parentMap, own), nil
}

func buildAssignmentConfig(courseName, assignment string, course *courseSourceRaw, src *AssignmentSource, g Globals, onlyFor ...string) (*AssignmentConfig, error) {
	per := resolvePer(src.Per)
	path := resolveAssignmentPath(course, src)
	starter, err := resolveStartercode(courseName, assignment, src.Startercode)
	if err != nil {
		return nil, err
	}
	branchRules := resolveBranches(src, starter)
	release := resolveRelease(src.Release)

	containerRegistry := src.ContainerRegistry
	if release != nil && release.DockerImages != nil {
		containerRegistry = true
	}

	mergeRequest, err := resolveMergeRequest(courseName, assignment, src.MergeRequest)
	if err != nil {
		return nil, err
	}

	seeder, err := resolveSeeder(courseName, assignment, src.Seeder)
	if err != nil {
		return nil, err
	}

	// Absent means true; an explicit false means false.
	useEmailDomainAsSuffix := true
	if course.UseEmailDomainAsSuffix != nil {
		useEmailDomainAsSuffix = *course.UseEmailDomainAsSuffix
	}

	return &AssignmentConfig{
		Course:                 courseName,
		Name:                   assignment,
		UseCoursenameAsPrefix:  course.UseCoursenameAsPrefix,
		UseEmailDomainAsSuffix: useEmailDomainAsSuffix,
		Path:                   path,
		URL:                    g.GitlabHost + "/" + path,
		Per:                    per,
		Description:            resolveDescription(src.Description),
		ContainerRegistry:      containerRegistry,
		AccessLevel:            resolveAccessLevel(src.AccessLevel),
		MergeRequest:           mergeRequest,
		Branches:               branchRules,
		Issues:                 resolveIssues(src),
		Students:               resolveStudents(per, course, src, onlyFor...),
		Groups:                 resolveGroups(per, course, src, onlyFor...),
		Startercode:            starter,
		Clone:                  resolveClone(src.Clone, defaultBranch(branchRules, "main")),
		Release:                release,
		Seeder:                 seeder,
		DeferredBranches:       resolveDeferredBranches(src, starter),
	}, nil
}

func resolvePer(per string) Per {
	if strings.EqualFold(per, string(PerGroup)) {
		return PerGroup
	}
	return PerStudent
}

func resolveDescription(description string) string {
	if description != "" {
		return description
	}
	return "generated by glabs"
}

func resolveAccessLevel(level string) AccessLevel {
	switch strings.ToLower(level) {
	case "guest":
		return Guest
	case "reporter":
		return Reporter
	case "maintainer":
		return Maintainer
	}
	return Developer
}

func resolveAssignmentPath(course *courseSourceRaw, src *AssignmentSource) string {
	path := course.CoursePath
	if course.SemesterPath != "" {
		path += "/" + course.SemesterPath
	}
	if src.AssignmentPath != "" {
		path += "/" + src.AssignmentPath
	}
	return gitlabGroupPath(path)
}

// ResolveCourse resolves the course-level students and groups out of a raw
// course body.
func ResolveCourse(courseName string, body any) (*CourseConfig, error) {
	course, err := decodeCourseSettings(courseName, body)
	if err != nil {
		return nil, err
	}
	empty := &AssignmentSource{}
	return &CourseConfig{
		Course:   courseName,
		Students: resolveStudents(PerStudent, course, empty),
		Groups:   resolveGroups(PerGroup, course, empty),
	}, nil
}

// ResolveCoursePath returns the course subgroup path (coursepath/semesterpath).
func ResolveCoursePath(courseName string, body any) (string, error) {
	course, err := decodeCourseSettings(courseName, body)
	if err != nil {
		return "", err
	}
	path := course.CoursePath
	if course.SemesterPath != "" {
		path += "/" + course.SemesterPath
	}
	return gitlabGroupPath(path), nil
}

func decodeCourseSettings(courseName string, body any) (*courseSourceRaw, error) {
	courseMap, ok := asStringMap(body)
	if !ok {
		return nil, fmt.Errorf("configuration for course %s not found", courseName)
	}
	var course courseSourceRaw
	if err := decodeInto(&course, courseMap, nil); err != nil {
		return nil, fmt.Errorf("course %s: cannot decode course settings: %w", courseName, err)
	}
	return &course, nil
}

func resolveStartercode(courseName, assignment string, src *StartercodeSource) (*Startercode, error) {
	if src == nil {
		return nil, nil
	}
	if src.URL == "" {
		return nil, fmt.Errorf("course %s, assignment %s: startercode provided without url", courseName, assignment)
	}

	fromBranch := "main"
	if src.FromBranch != "" {
		fromBranch = src.FromBranch
	}
	templateMessage := "Initial"
	if src.TemplateMessage != "" {
		templateMessage = src.TemplateMessage
	}
	toBranch := "main"
	if src.ToBranch != "" {
		toBranch = src.ToBranch
	}

	return &Startercode{
		URL:                src.URL,
		FromBranch:         fromBranch,
		Tag:                src.Tag,
		Template:           src.Template,
		TemplateMessage:    templateMessage,
		ToBranch:           toBranch,
		AdditionalBranches: src.AdditionalBranches,
	}, nil
}

func resolveBranches(src *AssignmentSource, starter *Startercode) []BranchRule {
	rules := make([]BranchRule, 0, len(src.Branches))
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

	for _, rule := range src.Branches {
		appendOrMerge(BranchRule(rule))
	}

	// The startercode branch keys are a legacy shorthand and only apply when no
	// `branches:` block was configured at all.
	if len(rules) == 0 && src.Startercode != nil {
		s := src.Startercode
		if starter != nil {
			appendOrMerge(BranchRule{Name: starter.ToBranch, Default: true})
		}
		if s.DevBranch != "" {
			appendOrMerge(BranchRule{Name: s.DevBranch, Default: true})
		}
		for _, name := range s.AdditionalBranches {
			appendOrMerge(BranchRule{Name: name})
		}
		if s.ProtectToBranch && starter != nil {
			appendOrMerge(BranchRule{Name: starter.ToBranch, Protect: true})
		}
		if s.ProtectDevBranchMergeOnly {
			target := s.DevBranch
			if target == "" && starter != nil {
				target = starter.ToBranch
			}
			appendOrMerge(BranchRule{Name: target, MergeOnly: true})
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

func resolveIssues(src *AssignmentSource) *IssueReplication {
	replicate := false
	var numbers []int
	includeChildTasks := false

	if src.Issues != nil {
		replicate = src.Issues.ReplicateFromStartercode
		numbers = src.Issues.IssueNumbers
		includeChildTasks = src.Issues.IncludeChildTasks
	} else if src.Startercode != nil {
		// Legacy shorthand: only consulted when no `issues:` block exists at all,
		// not merely when replication is off.
		replicate = src.Startercode.ReplicateIssue
		numbers = src.Startercode.IssueNumbers
	}

	if !replicate {
		return &IssueReplication{ReplicateFromStartercode: false}
	}
	if len(numbers) == 0 {
		numbers = []int{1}
	}
	return &IssueReplication{ReplicateFromStartercode: true, IssueNumbers: numbers, IncludeChildTasks: includeChildTasks}
}

func resolveClone(src *CloneSource, defaultCloneBranch string) *Clone {
	clone := &Clone{LocalPath: ".", Branch: defaultCloneBranch}
	if src == nil {
		return clone
	}
	if src.LocalPath != nil {
		clone.LocalPath = *src.LocalPath
	}
	if src.Branch != nil {
		clone.Branch = *src.Branch
	}
	clone.Force = src.Force
	return clone
}

func resolveRelease(src *ReleaseSource) *Release {
	if src == nil {
		return nil
	}

	var mr *ReleaseMergeRequest
	if src.MergeRequest != nil {
		sourceBranch := "develop"
		if src.MergeRequest.Source != "" {
			sourceBranch = src.MergeRequest.Source
		}
		targetBranch := "main"
		if src.MergeRequest.Target != "" {
			targetBranch = src.MergeRequest.Target
		}
		mr = &ReleaseMergeRequest{
			SourceBranch: sourceBranch,
			TargetBranch: targetBranch,
			HasPipeline:  src.MergeRequest.Pipeline,
		}
	}

	images := src.DockerImages
	if len(images) == 0 {
		images = nil
	}

	return &Release{MergeRequest: mr, DockerImages: images}
}

func resolveDeferredBranches(src *AssignmentSource, starter *Startercode) map[string]*DeferredBranch {
	out := make(map[string]*DeferredBranch, len(src.DeferredBranches))
	for name, branch := range src.DeferredBranches {
		// viper compatibility: it lowercased every map key it loaded, and these
		// names are used verbatim (`glabs push mpd blatt07 devcontainer`) and end
		// up in branch names. Keeping the written case would silently change both.
		name = strings.ToLower(name)

		url := ""
		if starter != nil {
			url = starter.URL
		}
		if branch.URL != nil {
			url = *branch.URL
		}

		toBranch := branch.FromBranch
		if branch.ToBranch != nil {
			toBranch = *branch.ToBranch
		}

		orphan := true
		if branch.Orphan != nil {
			orphan = *branch.Orphan
		}

		orphanMessage := fmt.Sprintf("Snapshot of %s", name)
		if branch.OrphanMessage != nil {
			orphanMessage = *branch.OrphanMessage
		}

		out[name] = &DeferredBranch{
			URL:           url,
			FromBranch:    branch.FromBranch,
			ToBranch:      toBranch,
			Orphan:        orphan,
			OrphanMessage: orphanMessage,
		}
	}
	return out
}

func resolveMergeRequest(courseName, assignment string, src *MergeRequestSource) (*MergeRequest, error) {
	if src == nil {
		src = &MergeRequestSource{}
	}

	mergeMethod := MergeCommit
	switch src.MergeMethod {
	case "semi_linear":
		mergeMethod = SemiLinearHistory
	case "ff":
		mergeMethod = FastForward
	case "merge":
		mergeMethod = MergeCommit
	}

	squashOption := SquashDefaultOff
	switch src.SquashOption {
	case "never":
		squashOption = SquashNever
	case "always":
		squashOption = SquashAlways
	case "default_on":
		squashOption = SquashDefaultOn
	case "default_off":
		squashOption = SquashDefaultOff
	}

	approvals := resolveApprovalRules(src.Approvals)
	settings, err := resolveApprovalSettings(courseName, assignment, src.Approvals)
	if err != nil {
		return nil, err
	}

	return &MergeRequest{
		MergeMethod:                   mergeMethod,
		SquashOption:                  squashOption,
		PipelineMustSucceed:           src.Pipeline,
		SkippedPipelinesAreSuccessful: src.SkippedPipelinesAreSuccessful,
		AllThreadsMustBeResolved:      src.AllThreadsMustBeResolved,
		StatusChecksMustSucceed:       src.StatusChecksMustSucceed,
		Approvals:                     approvals,
		ApprovalSettings:              settings,
	}, nil
}

func resolveApprovalRules(src *ApprovalsSource) []MergeRequestApprovalRule {
	if src == nil || src.Rules == nil {
		return nil
	}

	normalized := make([]MergeRequestApprovalRule, 0, len(src.Rules))
	for _, rule := range src.Rules {
		out := MergeRequestApprovalRule{
			Name:                  strings.TrimSpace(rule.Name),
			MultiMemberGroupsOnly: rule.MultiMemberGroupsOnly,
			RequiredApprovals:     rule.RequiredApprovals,
		}

		branches := rule.Branches
		if branch := strings.TrimSpace(rule.Branch); branch != "" {
			branches = append(append([]string(nil), branches...), branch)
		}
		out.Branches = trimUnique(branches)
		out.Usernames = trimNonEmpty(rule.Usernames)
		out.Groups = trimNonEmpty(rule.Groups)

		if out.RequiredApprovals < 0 {
			out.RequiredApprovals = 0
		}
		if len(out.Branches) == 0 {
			continue
		}
		normalized = append(normalized, out)
	}
	return normalized
}

func trimUnique(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func trimNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func resolveApprovalSettings(courseName, assignment string, src *ApprovalsSource) (*MergeRequestApprovalSettings, error) {
	if src == nil || src.Settings == nil {
		return nil, nil
	}
	s := src.Settings

	settings := &MergeRequestApprovalSettings{
		PreventApprovalByMergeRequestCreator:       s.PreventApprovalByMergeRequestCreator,
		PreventApprovalsByUsersWhoAddCommits:       s.PreventApprovalsByUsersWhoAddCommits,
		PreventEditingApprovalRulesInMergeRequests: s.PreventEditingApprovalRulesInMergeRequests,
		RequireUserReauthenticationToApprove:       s.RequireUserReauthenticationToApprove,
	}

	if s.WhenCommitAdded != nil {
		v := ApprovalWhenCommitAdded(*s.WhenCommitAdded)
		switch v {
		case ApprovalKeepApprovals, ApprovalRemoveAllApprovals, ApprovalRemoveCodeOwnerApprovalsIfFilesChanged:
			settings.WhenCommitAdded = &v
		default:
			return nil, fmt.Errorf("course %s, assignment %s: invalid mergeRequest.approvals.settings.whenCommitAdded %q, must be one of %s, %s, %s",
				courseName, assignment, v,
				ApprovalKeepApprovals, ApprovalRemoveAllApprovals, ApprovalRemoveCodeOwnerApprovalsIfFilesChanged)
		}
	}

	if settings.PreventApprovalByMergeRequestCreator == nil &&
		settings.PreventApprovalsByUsersWhoAddCommits == nil &&
		settings.PreventEditingApprovalRulesInMergeRequests == nil &&
		settings.RequireUserReauthenticationToApprove == nil &&
		settings.WhenCommitAdded == nil {
		return nil, nil
	}

	return settings, nil
}

func resolveStudents(per Per, course *courseSourceRaw, src *AssignmentSource, onlyFor ...string) []*Student {
	if per == PerGroup {
		return nil
	}

	studs := src.Students
	if len(studs) == 0 {
		studs = course.Students
	} else {
		studs = append(append([]string(nil), studs...), course.Students...)
	}

	studs = filterByPatterns(studs, onlyFor)
	sort.Strings(studs)
	return mkStudents(studs)
}

func resolveGroups(per Per, course *courseSourceRaw, src *AssignmentSource, onlyFor ...string) []*Group {
	if per == PerStudent {
		return nil
	}

	// viper compatibility: group names were lowercased on load. They end up in
	// repository names, so keeping the written case would point at a different
	// GitLab project.
	groupsMap := make(map[string][]string, len(course.Groups)+len(src.Groups))
	for name, members := range course.Groups {
		groupsMap[strings.ToLower(name)] = members
	}
	for name, members := range src.Groups {
		groupsMap[strings.ToLower(name)] = members
	}

	if len(onlyFor) > 0 {
		filtered := make(map[string][]string)
		for _, pattern := range onlyFor {
			for name, members := range groupsMap {
				if matchesPattern(pattern, name) {
					filtered[name] = members
				}
			}
		}
		groupsMap = filtered
	}

	names := make([]string, 0, len(groupsMap))
	for name := range groupsMap {
		names = append(names, name)
	}
	sort.Strings(names)

	groups := make([]*Group, 0, len(groupsMap))
	for _, name := range names {
		members := append([]string(nil), groupsMap[name]...)
		sort.Strings(members)
		groups = append(groups, &Group{Name: name, Members: mkStudents(members)})
	}
	return groups
}

func filterByPatterns(values []string, patterns []string) []string {
	if len(patterns) == 0 {
		return values
	}
	out := make([]string, 0, len(values))
	for _, pattern := range patterns {
		for _, value := range values {
			if matchesPattern(pattern, value) {
				out = append(out, value)
			}
		}
	}
	return out
}

func resolveSeeder(courseName, assignment string, src *SeederSource) (*Seeder, error) {
	if src == nil {
		return nil, nil
	}
	if src.Cmd == "" {
		return nil, fmt.Errorf("course %s, assignment %s: seeder provided without cmd", courseName, assignment)
	}

	toBranch := "main"
	if src.ToBranch != "" {
		toBranch = src.ToBranch
	}

	entity, err := parseSignKey(courseName+"."+assignment, src.SignKey)
	if err != nil {
		return nil, err
	}

	return &Seeder{
		Command:         src.Cmd,
		Args:            src.Args,
		Name:            src.Name,
		EMail:           src.EMail,
		SignKey:         entity,
		ToBranch:        toBranch,
		ProtectToBranch: src.ProtectToBranch,
	}, nil
}
