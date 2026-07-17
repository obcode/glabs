package config

import (
	"reflect"
	"testing"
)

func TestStartercodeDefaults(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    startercode:
      url: git@example.org:starter.git
`)
	s := mustAssignmentConfig(t, "course", "a1").Startercode

	if s == nil {
		t.Fatal("startercode should not be nil")
	}
	if s.FromBranch != "main" || s.ToBranch != "main" {
		t.Fatalf("unexpected startercode defaults: %#v", s)
	}
	if s.TemplateMessage != "Initial" {
		t.Errorf("TemplateMessage = %q, want %q", s.TemplateMessage, "Initial")
	}
	if s.Tag != "" {
		t.Fatalf("startercode tag = %q, want empty", s.Tag)
	}
}

func TestStartercodeOverrides(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    startercode:
      url: git@example.org:starter.git
      fromBranch: template
      tag: v1.2.3
      toBranch: submission
      additionalBranches: [release, demo]
`)
	s := mustAssignmentConfig(t, "course", "a1").Startercode

	if s.FromBranch != "template" || s.ToBranch != "submission" {
		t.Fatalf("startercode branches = %#v", s)
	}
	if s.Tag != "v1.2.3" {
		t.Fatalf("startercode tag = %q, want %q", s.Tag, "v1.2.3")
	}
	if !reflect.DeepEqual(s.AdditionalBranches, []string{"release", "demo"}) {
		t.Fatalf("additionalBranches = %#v", s.AdditionalBranches)
	}
}

func TestNoStartercode(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    assignmentpath: blatt-01
`)
	if s := mustAssignmentConfig(t, "course", "a1").Startercode; s != nil {
		t.Fatalf("startercode = %#v, want nil when unconfigured", s)
	}
}

func TestBranchesFromExplicitRules(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    startercode:
      url: git@example.org:starter.git
      toBranch: main
    branches:
      - name: main
        mergeOnly: true
      - name: startercode
        protect: true
`)
	b := mustAssignmentConfig(t, "course", "a1").Branches

	want := []BranchRule{
		{Name: "main", MergeOnly: true, Default: true},
		{Name: "startercode", Protect: true},
	}
	if !reflect.DeepEqual(b, want) {
		t.Fatalf("branches = %#v, want %#v", b, want)
	}
}

// With no rule marked default, the first one becomes it.
func TestBranchesForceDefaultOntoFirstRule(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    branches:
      - name: develop
      - name: main
`)
	b := mustAssignmentConfig(t, "course", "a1").Branches

	if !b[0].Default || b[1].Default {
		t.Fatalf("expected the first rule to be default, got %#v", b)
	}
}

// Duplicate rules for the same branch merge by OR-ing their flags.
func TestBranchesMergeDuplicates(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    branches:
      - name: main
        protect: true
      - name: main
        mergeOnly: true
        allowForcePush: true
`)
	b := mustAssignmentConfig(t, "course", "a1").Branches

	want := []BranchRule{{Name: "main", Protect: true, MergeOnly: true, Default: true, AllowForcePush: true}}
	if !reflect.DeepEqual(b, want) {
		t.Fatalf("branches = %#v, want %#v", b, want)
	}
}

// The startercode branch keys are a legacy shorthand, honoured only when no
// branches: block exists.
func TestBranchesFromLegacyStartercodeKeys(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    startercode:
      url: git@example.org:starter.git
      toBranch: main
      devBranch: develop
      protectToBranch: true
      protectDevBranchMergeOnly: true
      additionalBranches: [extra]
`)
	b := mustAssignmentConfig(t, "course", "a1").Branches

	want := []BranchRule{
		{Name: "main", Protect: true, Default: true},
		{Name: "develop", MergeOnly: true, Default: true},
		{Name: "extra"},
	}
	if !reflect.DeepEqual(b, want) {
		t.Fatalf("branches = %#v, want %#v", b, want)
	}
}

func TestBranchesIgnoreLegacyKeysWhenBranchesConfigured(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    startercode:
      url: git@example.org:starter.git
      toBranch: main
      devBranch: develop
      protectToBranch: true
    branches:
      - name: main
        default: true
`)
	b := mustAssignmentConfig(t, "course", "a1").Branches

	want := []BranchRule{{Name: "main", Default: true}}
	if !reflect.DeepEqual(b, want) {
		t.Fatalf("branches = %#v, want %#v: an explicit branches block supersedes the legacy keys", b, want)
	}
}

func TestIssuesDefaults(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    issues:
      replicateFromStartercode: true
`)
	i := mustAssignmentConfig(t, "course", "a1").Issues

	if !i.ReplicateFromStartercode {
		t.Fatal("ReplicateFromStartercode = false, want true")
	}
	if !reflect.DeepEqual(i.IssueNumbers, []int{1}) {
		t.Fatalf("IssueNumbers = %#v, want [1]", i.IssueNumbers)
	}
}

func TestIssuesLegacyStartercodeKeys(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    startercode:
      url: git@example.org:starter.git
      replicateIssue: true
      issueNumbers: [3, 4]
`)
	i := mustAssignmentConfig(t, "course", "a1").Issues

	if !i.ReplicateFromStartercode || !reflect.DeepEqual(i.IssueNumbers, []int{3, 4}) {
		t.Fatalf("issues = %#v, want replication of issues 3 and 4", i)
	}
}

// An issues: block disables the legacy keys merely by existing, even when it
// turns replication off.
func TestIssuesBlockShadowsLegacyStartercodeKeys(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    startercode:
      url: git@example.org:starter.git
      replicateIssue: true
      issueNumbers: [3, 4]
    issues:
      replicateFromStartercode: false
`)
	i := mustAssignmentConfig(t, "course", "a1").Issues

	if i.ReplicateFromStartercode {
		t.Fatalf("issues = %#v, want replication off: the issues block supersedes the legacy keys", i)
	}
}

func TestCloneDefaults(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    branches:
      - name: develop
        default: true
`)
	c := mustAssignmentConfig(t, "course", "a1").Clone

	if c.LocalPath != "." {
		t.Errorf("LocalPath = %q, want %q", c.LocalPath, ".")
	}
	if c.Branch != "develop" {
		t.Errorf("Branch = %q, want the assignment's default branch %q", c.Branch, "develop")
	}
	if c.Force {
		t.Error("Force = true, want false")
	}
}

func TestCloneOverrides(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    clone:
      localpath: /tmp/glabs
      branch: main
      force: true
`)
	c := mustAssignmentConfig(t, "course", "a1").Clone

	want := &Clone{LocalPath: "/tmp/glabs", Branch: "main", Force: true}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("clone = %#v, want %#v", c, want)
	}
}
