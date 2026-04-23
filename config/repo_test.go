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
	viper.Set("course.a1.startercode.replicateIssue", true)

	s := startercode("course.a1")
	if s == nil {
		t.Fatal("startercode should not be nil")
	}
	if s.FromBranch != "main" || s.ToBranch != "main" || s.DevBranch != "main" {
		t.Fatalf("unexpected startercode defaults: %#v", s)
	}
	if !s.ReplicateIssue {
		t.Fatal("ReplicateIssue should be true")
	}
	if !reflect.DeepEqual(s.IssueNumbers, []int{1}) {
		t.Fatalf("IssueNumbers = %#v, want [1]", s.IssueNumbers)
	}
}

func TestStartercodeOverrides(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.startercode", map[string]string{"url": "git@example.org:starter.git"})
	viper.Set("course.a1.startercode.url", "git@example.org:starter.git")
	viper.Set("course.a1.startercode.fromBranch", "template")
	viper.Set("course.a1.startercode.toBranch", "submission")
	viper.Set("course.a1.startercode.devBranch", "develop")
	viper.Set("course.a1.startercode.additionalBranches", []string{"release", "demo"})
	viper.Set("course.a1.startercode.replicateIssue", true)
	viper.Set("course.a1.startercode.issueNumbers", []int{4, 7})
	viper.Set("course.a1.startercode.protectToBranch", true)
	viper.Set("course.a1.startercode.protectDevBranchMergeOnly", true)

	s := startercode("course.a1")
	if s.FromBranch != "template" || s.ToBranch != "submission" || s.DevBranch != "develop" {
		t.Fatalf("startercode branches = %#v", s)
	}
	if !reflect.DeepEqual(s.AdditionalBranches, []string{"release", "demo"}) {
		t.Fatalf("additional branches = %#v", s.AdditionalBranches)
	}
	if !reflect.DeepEqual(s.IssueNumbers, []int{4, 7}) {
		t.Fatalf("IssueNumbers = %#v", s.IssueNumbers)
	}
	if !s.ProtectToBranch || !s.ProtectDevBranchMergeOnly {
		t.Fatalf("protect flags = %#v", s)
	}
}

func TestCloneDefaultsAndOverrides(t *testing.T) {
	resetViper(t)

	c := clone("course.a1")
	if c.LocalPath != "." || c.Branch != "main" || c.Force {
		t.Fatalf("clone defaults = %#v", c)
	}

	viper.Set("course.a1.clone", map[string]string{"localpath": "/tmp/repos", "branch": "dev"})
	viper.Set("course.a1.clone.localpath", "/tmp/repos")
	viper.Set("course.a1.clone.branch", "dev")
	viper.Set("course.a1.clone.force", true)

	c = clone("course.a1")
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
