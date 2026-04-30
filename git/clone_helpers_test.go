package git

import (
	"strings"
	"testing"

	"github.com/obcode/glabs/v2/config"
)

func TestCloneurl_HTTPS(t *testing.T) {
	cfg := &config.AssignmentConfig{
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: true,
	}

	got := ProjectRepoUrl(cfg, "alice")
	want := "git@gitlab.example.org:mpd/ss26/blatt-01/mpd-blatt01-alice"
	if got != want {
		t.Fatalf("cloneurl() = %q, want %q", got, want)
	}
}

func TestCloneurl_ContainsExpectedParts(t *testing.T) {
	cfg := &config.AssignmentConfig{
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: true,
	}

	got := ProjectRepoUrl(cfg, "bob")

	if strings.Contains(got, "https://") {
		t.Fatalf("cloneurl() should not contain https://, got %q", got)
	}
	if !strings.HasPrefix(got, "git@") {
		t.Fatalf("cloneurl() should start with git@, got %q", got)
	}
	if !strings.HasSuffix(got, "bob") {
		t.Fatalf("cloneurl() should end with suffix, got %q", got)
	}
}

func TestCloneurl_WithoutCoursePrefix(t *testing.T) {
	cfg := &config.AssignmentConfig{
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: false,
	}

	got := ProjectRepoUrl(cfg, "team1")
	want := "git@gitlab.example.org:mpd/ss26/blatt-01/blatt01-team1"
	if got != want {
		t.Fatalf("cloneurl() = %q, want %q", got, want)
	}
}

func TestLocalpath(t *testing.T) {
	cfg := &config.AssignmentConfig{
		Clone:                 &config.Clone{LocalPath: "/home/user/repos"},
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: true,
	}

	got := localpath(cfg, "alice")
	want := "/home/user/repos/mpd-blatt01-alice"
	if got != want {
		t.Fatalf("localpath() = %q, want %q", got, want)
	}
}

func TestLocalpath_TrailingSlash(t *testing.T) {
	cfg := &config.AssignmentConfig{
		Clone:                 &config.Clone{LocalPath: "/repos"},
		Name:                  "hw01",
		Course:                "fun",
		UseCoursenameAsPrefix: true,
	}

	got := localpath(cfg, "team2")
	want := "/repos/fun-hw01-team2"
	if got != want {
		t.Fatalf("localpath() = %q, want %q", got, want)
	}
}

func TestLocalpath_WithoutCoursePrefix(t *testing.T) {
	cfg := &config.AssignmentConfig{
		Clone:                 &config.Clone{LocalPath: "/repos"},
		Name:                  "hw01",
		Course:                "fun",
		UseCoursenameAsPrefix: false,
	}

	got := localpath(cfg, "teamA")
	want := "/repos/hw01-teamA"
	if got != want {
		t.Fatalf("localpath() = %q, want %q", got, want)
	}
}
