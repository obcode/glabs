package config

import (
	"fmt"
	"sort"
	"strings"
)

// Finding is one problem found in a course source.
type Finding struct {
	// Path is the dotted location, e.g. "vss.blatt2.release.mergeRequest.dockerImages".
	Path string
	// Message says what is wrong, in terms of what the config does or fails to do.
	Message string
	// Severity distinguishes "this silently does nothing" from "this still works
	// but has a better spelling".
	Severity Severity
}

type Severity string

const (
	// SeverityProblem: the config does not do what it looks like it does.
	SeverityProblem Severity = "problem"
	// SeverityDeprecated: it works, but the spelling or shape is obsolete.
	SeverityDeprecated Severity = "deprecated"
)

func (f Finding) String() string { return fmt.Sprintf("%s: %s: %s", f.Severity, f.Path, f.Message) }

// Lint reports everything questionable about a decoded course source.
//
// The interesting category is keys that are silently ignored. The loader has
// never complained about them, so they look effective and are not — `clone.clone`
// (present in every real course file, does nothing) and
// `release.mergeRequest.dockerImages` (six images configured, none applied,
// because config/release.go:47 reads release.dockerImages) are both live
// examples.
func Lint(course *CourseSource, decoded *DecodeResult) []Finding {
	var findings []Finding

	for _, key := range decoded.UnknownKeys {
		findings = append(findings, Finding{
			Path:     key,
			Message:  "no such configuration key; it is silently ignored" + unknownKeyHint(key),
			Severity: SeverityProblem,
		})
	}

	for _, legacy := range decoded.LegacyKeys {
		findings = append(findings, Finding{
			Path:     legacy.Path,
			Message:  legacy.Hint + "; `glabs config migrate` rewrites it",
			Severity: SeverityDeprecated,
		})
	}

	for _, name := range sortedAssignmentNames(course) {
		findings = append(findings, lintAssignment(course, name, course.Assignments[name])...)
	}

	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Path < findings[j].Path })
	return findings
}

// unknownKeyHint points at the likely intent for the unknown keys that actually
// occur, rather than leaving the reader to guess.
func unknownKeyHint(key string) string {
	switch {
	case strings.HasSuffix(key, ".clone.clone"):
		return " (the clone command has no such option; configuring `clone:` at all is what enables it)"
	case strings.HasSuffix(key, ".release.mergeRequest.dockerImages"):
		return " (dockerImages belongs under `release:`, not under `release.mergeRequest:`)"
	default:
		return ""
	}
}

func lintAssignment(course *CourseSource, name string, a *AssignmentSource) []Finding {
	path := course.Name + "." + name
	var findings []Finding

	add := func(sub, msg string, sev Severity) {
		p := path
		if sub != "" {
			p += "." + sub
		}
		findings = append(findings, Finding{Path: p, Message: msg, Severity: sev})
	}

	if a.Extends != "" {
		if _, ok := course.Assignments[a.Extends]; !ok {
			add("extends", fmt.Sprintf("extends %q, which is not an assignment in this course", a.Extends), SeverityProblem)
		}
	}

	if s := a.Startercode; s != nil {
		if s.URL == "" {
			add("startercode.url", "startercode without a url", SeverityProblem)
		}
		// The startercode branch keys are only honoured when `branches:` is
		// absent (config/repo.go:98). With both present the legacy ones do
		// nothing at all, which is worth saying out loud.
		legacyBranchKeys := []struct {
			key string
			set bool
		}{
			{"devBranch", s.DevBranch != ""},
			{"protectToBranch", s.ProtectToBranch},
			{"protectDevBranchMergeOnly", s.ProtectDevBranchMergeOnly},
		}
		for _, k := range legacyBranchKeys {
			if !k.set {
				continue
			}
			if len(a.Branches) > 0 {
				add("startercode."+k.key, "ignored because `branches:` is configured, which supersedes it", SeverityProblem)
			} else {
				add("startercode."+k.key, "deprecated; express this with a `branches:` block instead", SeverityDeprecated)
			}
		}

		// Same shape for the issue keys, but the trigger is different: an
		// `issues:` block disables them merely by existing (config/repo.go:193).
		if a.Issues != nil {
			if s.ReplicateIssue {
				add("startercode.replicateIssue", "ignored because an `issues:` block is configured, which supersedes it", SeverityProblem)
			}
			if len(s.IssueNumbers) > 0 {
				add("startercode.issueNumbers", "ignored because an `issues:` block is configured, which supersedes it", SeverityProblem)
			}
		} else {
			if s.ReplicateIssue {
				add("startercode.replicateIssue", "deprecated; use `issues.replicateFromStartercode` instead", SeverityDeprecated)
			}
			if len(s.IssueNumbers) > 0 {
				add("startercode.issueNumbers", "deprecated; use `issues.issueNumbers` instead", SeverityDeprecated)
			}
		}
	}

	if a.Seeder != nil {
		add("seeder", "the seeder is deprecated and will be removed; it is not available in the web app", SeverityDeprecated)
		if a.Seeder.Cmd == "" {
			add("seeder.cmd", "seeder without a cmd", SeverityProblem)
		}
		if a.Seeder.SignKey != "" {
			add("seeder.signKey", "a PGP private key inline in a config file, which gitleaks will flag and which lands in every clone of this repository", SeverityDeprecated)
		}
	}

	if mr := a.MergeRequest; mr != nil && mr.Approvals != nil {
		for i, rule := range mr.Approvals.Rules {
			rp := fmt.Sprintf("mergeRequest.approvals.rules[%d]", i)
			if rule.Branch != "" {
				add(rp+".branch", "deprecated singular form; use `branches` instead", SeverityDeprecated)
			}
			if rule.Branch == "" && len(rule.Branches) == 0 {
				add(rp, fmt.Sprintf("rule %q has no branches and is silently dropped", rule.Name), SeverityProblem)
			}
			if rule.RequiredApprovals < 0 {
				add(rp+".requiredApprovals", "negative; clamped to 0", SeverityProblem)
			}
		}
		if s := mr.Approvals.Settings; s != nil && s.WhenCommitAdded != nil {
			if !validWhenCommitAdded(*s.WhenCommitAdded) {
				add("mergeRequest.approvals.settings.whenCommitAdded",
					fmt.Sprintf("invalid value %q, must be one of %s, %s, %s", *s.WhenCommitAdded,
						ApprovalKeepApprovals, ApprovalRemoveAllApprovals, ApprovalRemoveCodeOwnerApprovalsIfFilesChanged),
					SeverityProblem)
			}
		}
	}

	return findings
}

func validWhenCommitAdded(v string) bool {
	switch ApprovalWhenCommitAdded(v) {
	case ApprovalKeepApprovals, ApprovalRemoveAllApprovals, ApprovalRemoveCodeOwnerApprovalsIfFilesChanged:
		return true
	default:
		return false
	}
}

func sortedAssignmentNames(course *CourseSource) []string {
	names := make([]string, 0, len(course.Assignments))
	for name := range course.Assignments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
