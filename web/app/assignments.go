package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/obcode/glabs/v3/config"
)

// siblingNames returns the names of all assignments in the course except the
// given one, sorted. These are the valid targets for `extends` (which is
// course-internal), i.e. the choices for the editor's inheritance dropdown.
func siblingNames(src *config.CourseSource, self string) []string {
	names := make([]string, 0, len(src.Assignments))
	for name := range src.Assignments {
		if name != self {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// FieldValue is one field's own (source) value, stringified and keyed by the
// same key as FieldMeta so the GUI can pre-fill the schema-driven form.
type FieldValue struct {
	Key   string
	Value string
}

// assignmentOwn extracts the source values of the schema's fields from an
// AssignmentSource, keyed by FieldMeta.key. Booleans become "true"/"false",
// string lists become comma-separated, and nested blocks use dotted keys.
func assignmentOwn(src *config.AssignmentSource) []FieldValue {
	sc := src.Startercode
	scStr := func(f func(*config.StartercodeSource) string) string {
		if sc == nil {
			return ""
		}
		return f(sc)
	}
	scBool := func(f func(*config.StartercodeSource) bool) string {
		if sc == nil {
			return "false"
		}
		return strconv.FormatBool(f(sc))
	}
	var addBranches string
	if sc != nil {
		addBranches = strings.Join(sc.AdditionalBranches, ", ")
	}

	own := []FieldValue{
		{Key: "extends", Value: src.Extends},
		{Key: "abstract", Value: strconv.FormatBool(src.Abstract)},
		{Key: "per", Value: src.Per},
		{Key: "accesslevel", Value: src.AccessLevel},
		{Key: "description", Value: src.Description},
		{Key: "assignmentpath", Value: src.AssignmentPath},
		{Key: "containerRegistry", Value: strconv.FormatBool(src.ContainerRegistry)},
		{Key: "startercode.url", Value: scStr(func(s *config.StartercodeSource) string { return s.URL })},
		{Key: "startercode.fromBranch", Value: scStr(func(s *config.StartercodeSource) string { return s.FromBranch })},
		{Key: "startercode.tag", Value: scStr(func(s *config.StartercodeSource) string { return s.Tag })},
		{Key: "startercode.toBranch", Value: scStr(func(s *config.StartercodeSource) string { return s.ToBranch })},
		{Key: "startercode.template", Value: scBool(func(s *config.StartercodeSource) bool { return s.Template })},
		{Key: "startercode.templateMessage", Value: scStr(func(s *config.StartercodeSource) string { return s.TemplateMessage })},
		{Key: "startercode.additionalBranches", Value: addBranches},
		{Key: "mergeRequest.mergeMethod", Value: mrStr(src.MergeRequest, func(m *config.MergeRequestSource) string { return m.MergeMethod })},
		{Key: "mergeRequest.squashOption", Value: mrStr(src.MergeRequest, func(m *config.MergeRequestSource) string { return m.SquashOption })},
		{Key: "mergeRequest.pipeline", Value: mrBool(src.MergeRequest, func(m *config.MergeRequestSource) bool { return m.Pipeline })},
		{Key: "mergeRequest.skippedPipelinesAreSuccessful", Value: mrBool(src.MergeRequest, func(m *config.MergeRequestSource) bool { return m.SkippedPipelinesAreSuccessful })},
		{Key: "mergeRequest.allThreadsMustBeResolved", Value: mrBool(src.MergeRequest, func(m *config.MergeRequestSource) bool { return m.AllThreadsMustBeResolved })},
		{Key: "mergeRequest.statusChecksMustSucceed", Value: mrBool(src.MergeRequest, func(m *config.MergeRequestSource) bool { return m.StatusChecksMustSucceed })},
		{Key: "issues.replicateFromStartercode", Value: issuesBool(src.Issues, func(i *config.IssuesSource) bool { return i.ReplicateFromStartercode })},
		{Key: "issues.issueNumbers", Value: issueNumbersStr(src.Issues)},
		{Key: "issues.includeChildTasks", Value: issuesBool(src.Issues, func(i *config.IssuesSource) bool { return i.IncludeChildTasks })},
	}

	// branches (repeat group): a count sentinel plus indexed keys per rule, so
	// the whole list rides in the same flat draft as the scalar fields.
	own = append(own, FieldValue{Key: "branches.count", Value: strconv.Itoa(len(src.Branches))})
	for i, b := range src.Branches {
		p := fmt.Sprintf("branches.%d.", i)
		own = append(own,
			FieldValue{Key: p + "name", Value: b.Name},
			FieldValue{Key: p + "protect", Value: strconv.FormatBool(b.Protect)},
			FieldValue{Key: p + "mergeOnly", Value: strconv.FormatBool(b.MergeOnly)},
			FieldValue{Key: p + "default", Value: strconv.FormatBool(b.Default)},
			FieldValue{Key: p + "allowForcePush", Value: strconv.FormatBool(b.AllowForcePush)},
			FieldValue{Key: p + "codeOwnerApprovalRequired", Value: strconv.FormatBool(b.CodeOwnerApprovalRequired)},
		)
	}

	// mergeRequest.approvals (settings tri-state + rules repeat group), keyed
	// flat as approvals.* so they render as their own section.
	var appr *config.ApprovalsSource
	if src.MergeRequest != nil {
		appr = src.MergeRequest.Approvals
	}
	var settings *config.ApprovalSettingsSource
	if appr != nil {
		settings = appr.Settings
	}
	own = append(own,
		FieldValue{Key: "approvals.settings.preventApprovalByMergeRequestCreator", Value: triBool(settings, func(s *config.ApprovalSettingsSource) *bool { return s.PreventApprovalByMergeRequestCreator })},
		FieldValue{Key: "approvals.settings.preventApprovalsByUsersWhoAddCommits", Value: triBool(settings, func(s *config.ApprovalSettingsSource) *bool { return s.PreventApprovalsByUsersWhoAddCommits })},
		FieldValue{Key: "approvals.settings.preventEditingApprovalRulesInMergeRequests", Value: triBool(settings, func(s *config.ApprovalSettingsSource) *bool { return s.PreventEditingApprovalRulesInMergeRequests })},
		FieldValue{Key: "approvals.settings.requireUserReauthenticationToApprove", Value: triBool(settings, func(s *config.ApprovalSettingsSource) *bool { return s.RequireUserReauthenticationToApprove })},
		FieldValue{Key: "approvals.settings.whenCommitAdded", Value: optStr(settings)},
	)
	var rules []config.ApprovalRuleSource
	if appr != nil {
		rules = appr.Rules
	}
	own = append(own, FieldValue{Key: "approvals.rules.count", Value: strconv.Itoa(len(rules))})
	for i, r := range rules {
		p := fmt.Sprintf("approvals.rules.%d.", i)
		own = append(own,
			FieldValue{Key: p + "name", Value: r.Name},
			FieldValue{Key: p + "requiredApprovals", Value: strconv.Itoa(r.RequiredApprovals)},
			FieldValue{Key: p + "usernames", Value: strings.Join(r.Usernames, ", ")},
			FieldValue{Key: p + "groups", Value: strings.Join(r.Groups, ", ")},
			FieldValue{Key: p + "branches", Value: strings.Join(r.Branches, ", ")},
			FieldValue{Key: p + "multiMemberGroupsOnly", Value: strconv.FormatBool(r.MultiMemberGroupsOnly)},
		)
	}
	return own
}

func triBool(s *config.ApprovalSettingsSource, f func(*config.ApprovalSettingsSource) *bool) string {
	if s == nil {
		return ""
	}
	b := f(s)
	if b == nil {
		return ""
	}
	return strconv.FormatBool(*b)
}

func optStr(s *config.ApprovalSettingsSource) string {
	if s == nil || s.WhenCommitAdded == nil {
		return ""
	}
	return *s.WhenCommitAdded
}

func issuesBool(i *config.IssuesSource, f func(*config.IssuesSource) bool) string {
	if i == nil {
		return "false"
	}
	return strconv.FormatBool(f(i))
}

func issueNumbersStr(i *config.IssuesSource) string {
	if i == nil || len(i.IssueNumbers) == 0 {
		return ""
	}
	parts := make([]string, 0, len(i.IssueNumbers))
	for _, n := range i.IssueNumbers {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ", ")
}

func mrStr(m *config.MergeRequestSource, f func(*config.MergeRequestSource) string) string {
	if m == nil {
		return ""
	}
	return f(m)
}

func mrBool(m *config.MergeRequestSource, f func(*config.MergeRequestSource) bool) string {
	if m == nil {
		return "false"
	}
	return strconv.FormatBool(f(m))
}

// AssignmentView is one assignment in source (own) form plus its resolved
// preview — the same rendering `glabs show` produces. It makes the inheritance
// visible in the browser exactly as the CLI's confirmation gate does.
type AssignmentView struct {
	Course   string
	Name     string
	Extends  string
	Abstract bool
	// ExtendsOptions lists the names of the sibling assignments this one may
	// inherit from (all assignments in the same course except this one, sorted).
	// `extends` is course-internal, so these are exactly the valid choices for
	// the editor's dropdown.
	ExtendsOptions []string
	// Own holds the assignment's own (source) field values, keyed by
	// FieldMeta.key, for pre-filling the editor form.
	Own []FieldValue
	// Resolved is the Show() rendering (may contain ANSI). Empty when the
	// assignment cannot be resolved (then ResolveError explains why).
	Resolved string
	// ResolveError is set when resolution fails — e.g. an abstract base, a
	// missing parent, or an `extends` cycle. Not a fault: an abstract base is a
	// valid document that simply has no resolved form.
	ResolveError string
}

// Assignment returns the source values and resolved preview for one assignment
// of one of the caller's own courses. It returns nil (no error) when the course
// exists but has no assignment by that name; ErrCourseNotFound propagates when
// the course itself is not the caller's.
func (a *App) Assignment(ctx context.Context, course, name string) (*AssignmentView, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}

	src, ok := stored.Source.Assignments[name]
	if !ok || src == nil {
		return nil, nil
	}

	view := &AssignmentView{
		Course:         course,
		Name:           name,
		Extends:        src.Extends,
		Abstract:       src.Abstract,
		ExtendsOptions: siblingNames(stored.Source, name),
		Own:            assignmentOwn(src),
	}

	// Resolve from the stored bytes so the preview matches the CLI exactly
	// (inheritance, legacy keys, lowercasing). Fall back to re-encoding the
	// source if the raw bytes are absent.
	bytes := stored.RawYAML
	if len(bytes) == 0 {
		if encoded, err := config.EncodeCourse(stored.Source); err == nil {
			bytes = encoded
		}
	}
	cfg, err := config.ResolveAssignmentFromBytes(bytes, course, name, config.Globals{GitlabHost: a.gitlabHost})
	if err != nil {
		view.ResolveError = err.Error()
		return view, nil
	}
	view.Resolved = cfg.Show()
	return view, nil
}

// ValidationResult reports whether a draft assignment is saveable and — when it
// resolves — its preview.
type ValidationResult struct {
	OK           bool
	Errors       []string
	Resolved     string
	ResolveError string
}

// applyDraft returns a copy of src with the draft's core-field values applied.
// An empty value unsets a field (so it inherits). Only the schema's core fields
// are editable here; nested blocks (mergeRequest, branches, …) are carried over
// untouched.
func applyDraft(src *config.AssignmentSource, draft map[string]string) *config.AssignmentSource {
	c := *src
	for key, val := range draft {
		switch key {
		case "extends":
			c.Extends = val
		case "abstract":
			c.Abstract = val == "true"
		case "per":
			c.Per = val
		case "accesslevel":
			c.AccessLevel = val
		case "description":
			c.Description = val
		case "assignmentpath":
			c.AssignmentPath = val
		case "containerRegistry":
			c.ContainerRegistry = val == "true"
		}
	}
	c.Startercode = applyStartercodeDraft(src.Startercode, draft)
	c.MergeRequest = applyMergeRequestDraft(src.MergeRequest, draft)
	c.Issues = applyIssuesDraft(src.Issues, draft)
	c.Branches = applyBranchesDraft(src.Branches, draft)
	return &c
}

// applyBranchesDraft rebuilds the branches list from the draft's indexed keys
// (branches.<i>.<field>), driven by the branches.count sentinel so the GUI can
// also clear the list. Rows without a name are dropped. Returns the original when
// the draft does not address branches at all.
func applyBranchesDraft(orig []config.BranchRuleSource, draft map[string]string) []config.BranchRuleSource {
	countStr, ok := draft["branches.count"]
	if !ok {
		return orig
	}
	n, _ := strconv.Atoi(countStr)
	var rules []config.BranchRuleSource
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("branches.%d.", i)
		name := strings.TrimSpace(draft[p+"name"])
		if name == "" {
			continue
		}
		rules = append(rules, config.BranchRuleSource{
			Name:                      name,
			Protect:                   draft[p+"protect"] == "true",
			MergeOnly:                 draft[p+"mergeOnly"] == "true",
			Default:                   draft[p+"default"] == "true",
			AllowForcePush:            draft[p+"allowForcePush"] == "true",
			CodeOwnerApprovalRequired: draft[p+"codeOwnerApprovalRequired"] == "true",
		})
	}
	return rules
}

// applyIssuesDraft rebuilds the issues block from the draft's issues.* keys into
// a NEW struct (never mutating the shared original) and nils it when every field
// is empty. issueNumbers is a comma-separated list of integers; non-numeric
// entries are dropped.
func applyIssuesDraft(orig *config.IssuesSource, draft map[string]string) *config.IssuesSource {
	touched := false
	for k := range draft {
		if strings.HasPrefix(k, "issues.") {
			touched = true
			break
		}
	}
	if !touched {
		return orig
	}

	var is config.IssuesSource
	if orig != nil {
		is = *orig
	}
	for k, v := range draft {
		switch k {
		case "issues.replicateFromStartercode":
			is.ReplicateFromStartercode = v == "true"
		case "issues.includeChildTasks":
			is.IncludeChildTasks = v == "true"
		case "issues.issueNumbers":
			is.IssueNumbers = splitIntList(v)
		}
	}
	if !is.ReplicateFromStartercode && !is.IncludeChildTasks && len(is.IssueNumbers) == 0 {
		return nil
	}
	return &is
}

// splitIntList parses a comma-/newline-separated list of integers, dropping
// blank and non-numeric entries.
func splitIntList(s string) []int {
	var out []int
	for _, f := range splitList(s) {
		if n, err := strconv.Atoi(f); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// applyMergeRequestDraft rebuilds the mergeRequest block (scalars + approvals)
// from the draft into a NEW struct (never mutating the shared original) and nils
// it when every field is empty and there is no approvals block.
func applyMergeRequestDraft(orig *config.MergeRequestSource, draft map[string]string) *config.MergeRequestSource {
	touched := false
	for k := range draft {
		if strings.HasPrefix(k, "mergeRequest.") || strings.HasPrefix(k, "approvals.") {
			touched = true
			break
		}
	}
	if !touched {
		return orig
	}

	var mr config.MergeRequestSource
	if orig != nil {
		mr = *orig
	}
	for k, v := range draft {
		switch k {
		case "mergeRequest.mergeMethod":
			mr.MergeMethod = v
		case "mergeRequest.squashOption":
			mr.SquashOption = v
		case "mergeRequest.pipeline":
			mr.Pipeline = v == "true"
		case "mergeRequest.skippedPipelinesAreSuccessful":
			mr.SkippedPipelinesAreSuccessful = v == "true"
		case "mergeRequest.allThreadsMustBeResolved":
			mr.AllThreadsMustBeResolved = v == "true"
		case "mergeRequest.statusChecksMustSucceed":
			mr.StatusChecksMustSucceed = v == "true"
		}
	}
	mr.Approvals = buildApprovals(mr.Approvals, draft)

	if mr.MergeMethod == "" && mr.SquashOption == "" && !mr.Pipeline &&
		!mr.SkippedPipelinesAreSuccessful && !mr.AllThreadsMustBeResolved &&
		!mr.StatusChecksMustSucceed && mr.Approvals == nil {
		return nil
	}
	return &mr
}

// buildApprovals rebuilds the approvals block from the draft's approvals.* keys
// into a NEW struct; returns the original when the draft does not address
// approvals, and nil when there are neither settings nor rules.
func buildApprovals(orig *config.ApprovalsSource, draft map[string]string) *config.ApprovalsSource {
	touched := false
	for k := range draft {
		if strings.HasPrefix(k, "approvals.") {
			touched = true
			break
		}
	}
	if !touched {
		return orig
	}
	settings := buildApprovalSettings(draft)
	rules := buildApprovalRules(draft)
	if settings == nil && len(rules) == 0 {
		return nil
	}
	return &config.ApprovalsSource{Settings: settings, Rules: rules}
}

func buildApprovalSettings(draft map[string]string) *config.ApprovalSettingsSource {
	s := config.ApprovalSettingsSource{}
	any := false
	setTri := func(key string, dst **bool) {
		if v, ok := draft[key]; ok && v != "" {
			b := v == "true"
			*dst = &b
			any = true
		}
	}
	setTri("approvals.settings.preventApprovalByMergeRequestCreator", &s.PreventApprovalByMergeRequestCreator)
	setTri("approvals.settings.preventApprovalsByUsersWhoAddCommits", &s.PreventApprovalsByUsersWhoAddCommits)
	setTri("approvals.settings.preventEditingApprovalRulesInMergeRequests", &s.PreventEditingApprovalRulesInMergeRequests)
	setTri("approvals.settings.requireUserReauthenticationToApprove", &s.RequireUserReauthenticationToApprove)
	if v, ok := draft["approvals.settings.whenCommitAdded"]; ok {
		if w := strings.TrimSpace(v); w != "" {
			s.WhenCommitAdded = &w
			any = true
		}
	}
	if !any {
		return nil
	}
	return &s
}

func buildApprovalRules(draft map[string]string) []config.ApprovalRuleSource {
	countStr, ok := draft["approvals.rules.count"]
	if !ok {
		return nil
	}
	n, _ := strconv.Atoi(countStr)
	var rules []config.ApprovalRuleSource
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("approvals.rules.%d.", i)
		name := strings.TrimSpace(draft[p+"name"])
		if name == "" {
			continue
		}
		ra, _ := strconv.Atoi(strings.TrimSpace(draft[p+"requiredApprovals"]))
		rules = append(rules, config.ApprovalRuleSource{
			Name:                  name,
			Branches:              splitList(draft[p+"branches"]),
			Usernames:             splitList(draft[p+"usernames"]),
			Groups:                splitList(draft[p+"groups"]),
			MultiMemberGroupsOnly: draft[p+"multiMemberGroupsOnly"] == "true",
			RequiredApprovals:     ra,
		})
	}
	return rules
}

// applyStartercodeDraft rebuilds the startercode block from the draft's
// startercode.* keys. It always returns a NEW struct (never mutates the shared
// original) when the draft touches startercode, and nil when every startercode
// field ends up empty (so the block is unset and inherits). When the draft does
// not touch startercode at all, the original is returned unchanged.
func applyStartercodeDraft(orig *config.StartercodeSource, draft map[string]string) *config.StartercodeSource {
	touched := false
	for k := range draft {
		if strings.HasPrefix(k, "startercode.") {
			touched = true
			break
		}
	}
	if !touched {
		return orig
	}

	var sc config.StartercodeSource
	if orig != nil {
		sc = *orig
	}
	for k, v := range draft {
		switch k {
		case "startercode.url":
			sc.URL = v
		case "startercode.fromBranch":
			sc.FromBranch = v
		case "startercode.tag":
			sc.Tag = v
		case "startercode.toBranch":
			sc.ToBranch = v
		case "startercode.template":
			sc.Template = v == "true"
		case "startercode.templateMessage":
			sc.TemplateMessage = v
		case "startercode.additionalBranches":
			sc.AdditionalBranches = splitList(v)
		}
	}
	if startercodeEmpty(&sc) {
		return nil
	}
	return &sc
}

// splitList parses a comma- or newline-separated list into trimmed, non-empty
// entries.
func splitList(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if t := strings.TrimSpace(f); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func startercodeEmpty(s *config.StartercodeSource) bool {
	// Legacy fields are not editable here, but they must still keep a block
	// alive so saving never silently drops them (glabs config migrate is what
	// removes them). staticcheck flags the deprecated reads; that is intentional.
	//nolint:staticcheck // deprecated legacy fields are read only to preserve them
	legacy := s.DevBranch != "" || s.ProtectToBranch || s.ProtectDevBranchMergeOnly ||
		s.ReplicateIssue || len(s.IssueNumbers) > 0
	return s.URL == "" && s.FromBranch == "" && s.Tag == "" && s.ToBranch == "" &&
		!s.Template && s.TemplateMessage == "" && len(s.AdditionalBranches) == 0 && !legacy
}

// courseWithDraft returns a shallow copy of the stored course source with the
// named assignment replaced by the drafted version, without mutating the stored
// source (its Assignments map is copied before the swap).
func courseWithDraft(src *config.CourseSource, name string, drafted *config.AssignmentSource) *config.CourseSource {
	newCourse := *src
	assignments := make(map[string]*config.AssignmentSource, len(src.Assignments))
	for k, v := range src.Assignments {
		assignments[k] = v
	}
	assignments[name] = drafted
	newCourse.Assignments = assignments
	return &newCourse
}

// validateDrafted runs the drafted course through the real resolver. An abstract
// draft is valid but has no preview; a concrete draft that does not resolve is a
// hard error.
func (a *App) validateDrafted(newCourse *config.CourseSource, course, name string, drafted *config.AssignmentSource) *ValidationResult {
	if drafted.Abstract {
		return &ValidationResult{OK: true, ResolveError: "abstrakte Basis — keine aufgelöste Vorschau"}
	}
	bytes, err := config.EncodeCourse(newCourse)
	if err != nil {
		return &ValidationResult{OK: false, Errors: []string{err.Error()}}
	}
	cfg, err := config.ResolveAssignmentFromBytes(bytes, course, name, config.Globals{GitlabHost: a.gitlabHost})
	if err != nil {
		return &ValidationResult{OK: false, Errors: []string{err.Error()}, ResolveError: err.Error()}
	}
	return &ValidationResult{OK: true, Resolved: cfg.Show()}
}

// draftedAssignment loads the caller's course and applies the draft to a copy of
// the named assignment, returning the new (unsaved) course source and the drafted
// assignment.
func (a *App) draftedAssignment(ctx context.Context, course, name string, draft map[string]string) (*config.CourseSource, *config.AssignmentSource, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, nil, err
	}
	orig, ok := stored.Source.Assignments[name]
	if !ok || orig == nil {
		orig = &config.AssignmentSource{} // upsert: validate as a new assignment
	}
	drafted := applyDraft(orig, draft)
	return courseWithDraft(stored.Source, name, drafted), drafted, nil
}

// ValidateAssignmentDraft validates a draft against the real resolver without
// saving.
func (a *App) ValidateAssignmentDraft(ctx context.Context, course, name string, draft map[string]string) (*ValidationResult, error) {
	newCourse, drafted, err := a.draftedAssignment(ctx, course, name, draft)
	if err != nil {
		return nil, err
	}
	return a.validateDrafted(newCourse, course, name, drafted), nil
}

// SetAssignment applies a draft to one of the caller's assignments: it validates,
// rejects a concrete draft that does not resolve, then persists — re-marshalling
// the whole course YAML (a real edit gives up the verbatim original bytes).
func (a *App) SetAssignment(ctx context.Context, course, name string, draft map[string]string) (*AssignmentView, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}
	orig, ok := stored.Source.Assignments[name]
	if !ok || orig == nil {
		// Upsert: create the assignment. Validate its name, since it becomes a
		// GitLab path segment.
		if !nameRe.MatchString(strings.TrimSpace(name)) {
			return nil, fmt.Errorf("invalid assignment name %q: use only letters, digits, '.', '-' and '_'", name)
		}
		orig = &config.AssignmentSource{}
	}
	drafted := applyDraft(orig, draft)
	newCourse := courseWithDraft(stored.Source, name, drafted)

	if vr := a.validateDrafted(newCourse, course, name, drafted); !vr.OK {
		return nil, fmt.Errorf("assignment %q is not valid: %s", name, strings.Join(vr.Errors, "; "))
	}

	raw, err := config.EncodeCourse(newCourse)
	if err != nil {
		return nil, err
	}
	stored.Source = newCourse
	stored.RawYAML = raw
	stored.UpdatedAt = time.Now()
	if err := a.db.SaveCourse(ctx, stored); err != nil {
		return nil, err
	}
	return a.Assignment(ctx, course, name)
}

// CopyAssignment duplicates one of the caller's assignments under a new name.
// Only the name has to be unique — the copy is otherwise identical to the
// source, including its `extends`. It validates the new name (it becomes a
// GitLab path segment), rejects a name that already exists, then persists —
// re-marshalling the whole course YAML. The persisted round-trip
// (encode → store → decode) makes the copy fully independent of the source.
func (a *App) CopyAssignment(ctx context.Context, course, from, newName string) (*AssignmentView, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}
	src, ok := stored.Source.Assignments[from]
	if !ok || src == nil {
		return nil, fmt.Errorf("course %q has no assignment %q to copy", course, from)
	}
	newName = strings.TrimSpace(newName)
	if !nameRe.MatchString(newName) {
		return nil, fmt.Errorf("invalid assignment name %q: use only letters, digits, '.', '-' and '_'", newName)
	}
	if _, exists := stored.Source.Assignments[newName]; exists {
		return nil, fmt.Errorf("an assignment named %q already exists in course %q", newName, course)
	}

	// Copy the source struct under the new key. Nested block pointers are shared
	// in memory but only ever read here; EncodeCourse serialises them into
	// independent bytes and the reload below decodes fresh structs.
	cp := *src
	newCourse := courseWithDraft(stored.Source, newName, &cp)

	raw, err := config.EncodeCourse(newCourse)
	if err != nil {
		return nil, err
	}
	stored.Source = newCourse
	stored.RawYAML = raw
	stored.UpdatedAt = time.Now()
	if err := a.db.SaveCourse(ctx, stored); err != nil {
		return nil, err
	}
	return a.Assignment(ctx, course, newName)
}

// DeleteAssignment removes one assignment from one of the caller's courses and
// re-marshals the course YAML. It returns false when there was no such
// assignment (and does not touch the course).
func (a *App) DeleteAssignment(ctx context.Context, course, name string) (bool, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return false, err
	}
	if _, ok := stored.Source.Assignments[name]; !ok {
		return false, nil
	}

	newCourse := *stored.Source
	assignments := make(map[string]*config.AssignmentSource, len(stored.Source.Assignments))
	for k, v := range stored.Source.Assignments {
		if k != name {
			assignments[k] = v
		}
	}
	newCourse.Assignments = assignments

	raw, err := config.EncodeCourse(&newCourse)
	if err != nil {
		return false, err
	}
	stored.Source = &newCourse
	stored.RawYAML = raw
	stored.UpdatedAt = time.Now()
	if err := a.db.SaveCourse(ctx, stored); err != nil {
		return false, err
	}
	return true, nil
}
