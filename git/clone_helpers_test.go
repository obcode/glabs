package git

import (
	"testing"

	"github.com/obcode/glabs/v3/config"
)

func TestProjectRepoUrlIsHTTPS(t *testing.T) {
	cfg := &config.AssignmentConfig{
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: true,
	}

	got := ProjectRepoUrl(cfg, "alice")
	want := "https://gitlab.example.org/mpd/ss26/blatt-01/mpd-blatt01-alice.git"
	if got != want {
		t.Fatalf("ProjectRepoUrl() = %q, want %q", got, want)
	}
}

func TestProjectRepoUrlWithoutCoursePrefix(t *testing.T) {
	cfg := &config.AssignmentConfig{
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: false,
	}

	got := ProjectRepoUrl(cfg, "team1")
	want := "https://gitlab.example.org/mpd/ss26/blatt-01/blatt01-team1.git"
	if got != want {
		t.Fatalf("ProjectRepoUrl() = %q, want %q", got, want)
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
