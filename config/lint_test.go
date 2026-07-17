package config

import (
	"slices"
	"strings"
	"testing"
)

func lintFixture(t *testing.T, name string) []Finding {
	t.Helper()
	course, decoded := decodeFixture(t, name)
	return Lint(course, decoded)
}

func findingFor(findings []Finding, path string) *Finding {
	for i := range findings {
		if findings[i].Path == path {
			return &findings[i]
		}
	}
	return nil
}

func paths(findings []Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Path)
	}
	return out
}

// Silently-ignored keys are the reason lint exists: nothing complains about
// them, so they look exactly like settings that work. Both cases here were real
// and were live in the course files until lint surfaced them — `clone.clone` in
// every single one, and dockerImages nested a level too deep in vss/blatt2,
// where all six images were dropped on the floor.
func TestLintReportsSilentlyIgnoredKeys(t *testing.T) {
	t.Parallel()

	course, decoded := decodeYAML(t, `
demo:
  coursepath: demo/semester
  blatt1:
    assignmentpath: blatt-1
    clone:
      localpath: /tmp/demo
      clone: true
    release:
      mergeRequest:
        pipeline: true
        dockerImages:
          - service/one
`)
	findings := Lint(course, decoded)

	f := findingFor(findings, "demo.blatt1.release.mergeRequest.dockerImages")
	if f == nil {
		t.Fatalf("dockerImages under mergeRequest not reported; got %v", paths(findings))
	}
	if f.Severity != SeverityProblem {
		t.Errorf("severity = %q, want %q: the images are configured and never applied", f.Severity, SeverityProblem)
	}
	if !strings.Contains(f.Message, "belongs under `release:`") {
		t.Errorf("message %q does not say where dockerImages belongs", f.Message)
	}

	f = findingFor(findings, "demo.blatt1.clone.clone")
	if f == nil {
		t.Fatalf("stray clone.clone not reported; got %v", paths(findings))
	}
	if f.Severity != SeverityProblem {
		t.Errorf("severity = %q, want %q", f.Severity, SeverityProblem)
	}
}

// The real course files must lint free of problems. Deprecations are allowed to
// remain — they still work — but anything that silently does nothing is a bug
// in the config and should not be sitting there unnoticed.
func TestLintFindsNoProblemsInRealFixtures(t *testing.T) {
	t.Parallel()

	for _, name := range realCourseFixtures {
		for _, f := range lintFixture(t, name) {
			if f.Severity == SeverityProblem {
				t.Errorf("%s.yaml: %s: %s", name, f.Path, f.Message)
			}
		}
	}
}

// A superseded setting is worse than a merely deprecated one: it is present,
// looks meaningful, and does nothing. lint must distinguish the two.
func TestLintDistinguishesSupersededFromDeprecated(t *testing.T) {
	t.Parallel()

	findings := lintFixture(t, "legacy")

	// devbranch has no branches: block, so the legacy keys are honoured.
	if f := findingFor(findings, "legacy.devbranch.startercode.devBranch"); f == nil {
		t.Errorf("devBranch not reported at all; got %v", paths(findings))
	} else if f.Severity != SeverityDeprecated {
		t.Errorf("severity = %q, want %q: without a branches: block the key still works",
			f.Severity, SeverityDeprecated)
	}

	// devbranchoverridden has one, so they are dead.
	if f := findingFor(findings, "legacy.devbranchoverridden.startercode.devBranch"); f == nil {
		t.Errorf("overridden devBranch not reported; got %v", paths(findings))
	} else {
		if f.Severity != SeverityProblem {
			t.Errorf("severity = %q, want %q: with a branches: block the key does nothing",
				f.Severity, SeverityProblem)
		}
		if !strings.Contains(f.Message, "supersedes") {
			t.Errorf("message %q does not explain what overrides it", f.Message)
		}
	}

	// Same shape for issues, but the trigger is the block merely existing.
	if f := findingFor(findings, "legacy.legacyissuesshadowed.startercode.replicateIssue"); f == nil {
		t.Errorf("shadowed replicateIssue not reported; got %v", paths(findings))
	} else if f.Severity != SeverityProblem {
		t.Errorf("severity = %q, want %q", f.Severity, SeverityProblem)
	}
}

func TestLintReportsDroppedAndClampedApprovalRules(t *testing.T) {
	t.Parallel()

	findings := lintFixture(t, "legacy")

	if f := findingFor(findings, "legacy.approvalsnormalized.mergeRequest.approvals.rules[0]"); f == nil {
		t.Errorf("rule without branches not reported; got %v", paths(findings))
	} else if !strings.Contains(f.Message, "silently dropped") {
		t.Errorf("message %q does not say the rule is dropped", f.Message)
	}

	if f := findingFor(findings, "legacy.approvalsnormalized.mergeRequest.approvals.rules[1].requiredApprovals"); f == nil {
		t.Errorf("negative requiredApprovals not reported; got %v", paths(findings))
	}
}

func TestLintReportsDeprecatedAliases(t *testing.T) {
	t.Parallel()

	findings := lintFixture(t, "legacy")

	for _, want := range []string{
		"legacy.approvalslist.mergeRequest.approvals[0].required_approvals",
		"legacy.approvalslist.mergeRequest.approvals[1].approvalsRequired",
	} {
		f := findingFor(findings, want)
		if f == nil {
			t.Errorf("alias %q not reported; got %v", want, paths(findings))
			continue
		}
		if f.Severity != SeverityDeprecated {
			t.Errorf("%s: severity = %q, want %q", want, f.Severity, SeverityDeprecated)
		}
		if !strings.Contains(f.Message, "migrate") {
			t.Errorf("%s: message %q does not point at `glabs config migrate`", want, f.Message)
		}
	}
}

func TestLintReportsBrokenExtends(t *testing.T) {
	t.Parallel()

	course := &CourseSource{
		Name: "c",
		Assignments: map[string]*AssignmentSource{
			"child": {Extends: "nosuchparent"},
		},
	}
	findings := Lint(course, &DecodeResult{})

	if f := findingFor(findings, "c.child.extends"); f == nil {
		t.Fatalf("dangling extends not reported; got %v", paths(findings))
	} else if f.Severity != SeverityProblem {
		t.Errorf("severity = %q, want %q", f.Severity, SeverityProblem)
	}
}

// A clean course must lint clean, or the command is noise and gets ignored.
//
// Built here rather than taken from testdata: none of the fixtures are clean.
// Neither is any real course file — every one carries at least a stray
// clone.clone — which is the whole reason this command exists.
func TestLintAcceptsCleanCourse(t *testing.T) {
	t.Parallel()

	branch := "main"
	course := &CourseSource{
		Name:                  "clean",
		CoursePath:            "clean/semester",
		SemesterPath:          "ss2026",
		UseCoursenameAsPrefix: true,
		Students:              []string{"a@example.edu"},
		Assignments: map[string]*AssignmentSource{
			"base": {Abstract: true, Per: "student"},
			"blatt01": {
				Extends:        "base",
				AssignmentPath: "blatt-01",
				Description:    "Blatt 1",
				Startercode: &StartercodeSource{
					URL:        "git@gitlab.lrz.de:clean/startercode.git",
					FromBranch: "template",
					ToBranch:   "main",
				},
				Branches: []BranchRuleSource{{Name: "main", Default: true, MergeOnly: true}},
				Issues:   &IssuesSource{ReplicateFromStartercode: true, IssueNumbers: []int{1}},
				Clone:    &CloneSource{Branch: &branch},
				MergeRequest: &MergeRequestSource{
					MergeMethod: "semi_linear",
					Approvals: &ApprovalsSource{
						Rules: []ApprovalRuleSource{{Name: "review", Branches: []string{"main"}, RequiredApprovals: 1}},
					},
				},
			},
		},
	}

	if findings := Lint(course, &DecodeResult{}); len(findings) != 0 {
		t.Errorf("clean course produced findings: %v", paths(findings))
	}
}

// Deterministic output matters: lint is meant to gate commits, and a set that
// reorders between runs makes a diff unreadable.
func TestLintIsOrdered(t *testing.T) {
	t.Parallel()

	findings := lintFixture(t, "legacy")
	got := paths(findings)
	if !slices.IsSorted(got) {
		t.Errorf("findings are not sorted by path: %v", got)
	}
}
