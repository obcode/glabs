package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestStartercodeDefaultsAndReplication(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.startercode", map[string]string{"url": "git@example.org:starter.git"})
	viper.Set("course.a1.startercode.url", "git@example.org:starter.git")

	s := startercode("course.a1")
	if s == nil {
		t.Fatal("startercode should not be nil")
	}
	if s.FromBranch != "main" || s.ToBranch != "main" {
		t.Fatalf("unexpected startercode defaults: %#v", s)
	}
}

func TestStartercodeOverrides(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.startercode", map[string]string{"url": "git@example.org:starter.git"})
	viper.Set("course.a1.startercode.url", "git@example.org:starter.git")
	viper.Set("course.a1.startercode.fromBranch", "template")
	viper.Set("course.a1.startercode.toBranch", "submission")
	viper.Set("course.a1.startercode.additionalBranches", []string{"release", "demo"})

	s := startercode("course.a1")
	if s.FromBranch != "template" || s.ToBranch != "submission" {
		t.Fatalf("startercode branches = %#v", s)
	}
	if !reflect.DeepEqual(s.AdditionalBranches, []string{"release", "demo"}) {
		t.Fatalf("startercode additional branches = %#v", s.AdditionalBranches)
	}
}

func TestBranches_DefaultsAndLegacyFallback(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.startercode", map[string]string{"url": "git@example.org:starter.git"})
	viper.Set("course.a1.startercode.url", "git@example.org:starter.git")
	viper.Set("course.a1.startercode.toBranch", "main")
	viper.Set("course.a1.startercode.devBranch", "develop")
	viper.Set("course.a1.startercode.additionalBranches", []string{"release"})
	viper.Set("course.a1.startercode.protectToBranch", true)
	viper.Set("course.a1.startercode.protectDevBranchMergeOnly", true)

	b := branches("course.a1", startercode("course.a1"))
	if len(b) != 3 {
		t.Fatalf("len(branches) = %d, want 3", len(b))
	}
	if b[0].Name != "main" {
		t.Fatalf("first branch = %#v", b[0])
	}
	if !b[0].Protect {
		t.Fatalf("main branch should be protected: %#v", b[0])
	}
	if b[1].Name != "develop" || !b[1].Default || !b[1].MergeOnly {
		t.Fatalf("develop branch = %#v", b[1])
	}
}

func TestBranches_ExplicitConfig(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.branches", []map[string]any{
		{"name": "main", "protect": true, "allowForcePush": true},
		{"name": "dev", "default": true, "mergeOnly": true, "codeOwnerApprovalRequired": true},
	})

	b := branches("course.a1", nil)
	if len(b) != 2 {
		t.Fatalf("len(branches) = %d, want 2", len(b))
	}
	if b[0].Name != "main" || !b[0].Protect || !b[0].AllowForcePush {
		t.Fatalf("main branch = %#v", b[0])
	}
	if b[1].Name != "dev" || !b[1].Default || !b[1].MergeOnly || !b[1].CodeOwnerApprovalRequired {
		t.Fatalf("dev branch = %#v", b[1])
	}
}

func TestBranches_MergesAdditionalBranchFlags(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.branches", []map[string]any{
		{"name": "main", "protect": true},
		{"name": "main", "allowForcePush": true},
		{"name": "main", "codeOwnerApprovalRequired": true},
	})

	b := branches("course.a1", nil)
	if len(b) != 1 {
		t.Fatalf("len(branches) = %d, want 1", len(b))
	}
	if !b[0].Protect || !b[0].AllowForcePush || !b[0].CodeOwnerApprovalRequired {
		t.Fatalf("merged branch = %#v", b[0])
	}
}

func TestBranches_LegacySnakeCaseAdditionalBranchFlags(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.branches", []map[string]any{
		{"name": "main", "protect": true, "allow_force_push": true},
		{"name": "main", "code_owner_approval_required": true},
	})

	b := branches("course.a1", nil)
	if len(b) != 1 {
		t.Fatalf("len(branches) = %d, want 1", len(b))
	}
	if !b[0].Protect || !b[0].AllowForcePush || !b[0].CodeOwnerApprovalRequired {
		t.Fatalf("legacy snake case branch = %#v", b[0])
	}
}

func TestIssues_DefaultsAndLegacyFallback(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.startercode.replicateIssue", true)

	i := issues("course.a1")
	if i == nil || !i.ReplicateFromStartercode {
		t.Fatalf("issues = %#v", i)
	}
	if !reflect.DeepEqual(i.IssueNumbers, []int{1}) {
		t.Fatalf("IssueNumbers = %#v, want [1]", i.IssueNumbers)
	}

	viper.Set("course.a1.issues.replicateFromStartercode", true)
	viper.Set("course.a1.issues.issueNumbers", []int{4, 7})
	i = issues("course.a1")
	if !reflect.DeepEqual(i.IssueNumbers, []int{4, 7}) {
		t.Fatalf("IssueNumbers = %#v", i.IssueNumbers)
	}
}

func TestCloneDefaultsAndOverrides(t *testing.T) {
	resetViper(t)

	c := clone("course.a1", "develop")
	if c.LocalPath != "." || c.Branch != "develop" || c.Force {
		t.Fatalf("clone defaults = %#v", c)
	}

	resetViper(t)
	c = clone("course.a1", "develop")
	if c.Branch != "develop" {
		t.Fatalf("clone default branch = %q, want %q", c.Branch, "develop")
	}

	viper.Set("course.a1.clone", map[string]string{"localpath": "/tmp/repos", "branch": "dev"})
	viper.Set("course.a1.clone.localpath", "/tmp/repos")
	viper.Set("course.a1.clone.branch", "dev")
	viper.Set("course.a1.clone.force", true)

	c = clone("course.a1", "develop")
	if c.LocalPath != "/tmp/repos" || c.Branch != "dev" || !c.Force {
		t.Fatalf("clone overrides = %#v", c)
	}
}

func TestReleaseDefaultsAndOverrides(t *testing.T) {
	resetViper(t)

	viper.Set("course.a1.release", map[string]any{"mergeRequest": true})
	viper.Set("course.a1.release.mergeRequest", map[string]string{"enabled": "true"})

	r := release("course.a1")
	if r == nil || r.MergeRequest == nil {
		t.Fatalf("release = %#v", r)
	}
	if r.MergeRequest.SourceBranch != "develop" || r.MergeRequest.TargetBranch != "main" {
		t.Fatalf("merge request defaults = %#v", r.MergeRequest)
	}

	viper.Set("course.a1.release.mergeRequest.source", "feature/release")
	viper.Set("course.a1.release.mergeRequest.target", "stable")
	viper.Set("course.a1.release.mergeRequest.pipeline", true)
	viper.Set("course.a1.release.dockerImages", []string{"img/app", "img/web"})

	r = release("course.a1")
	if r.MergeRequest.SourceBranch != "feature/release" || r.MergeRequest.TargetBranch != "stable" || !r.MergeRequest.HasPipeline {
		t.Fatalf("merge request overrides = %#v", r.MergeRequest)
	}
	if !reflect.DeepEqual(r.DockerImages, []string{"img/app", "img/web"}) {
		t.Fatalf("docker images = %#v", r.DockerImages)
	}
}
