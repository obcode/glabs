package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout to a pipe and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestUrls_AssignmentURL(t *testing.T) {
	cfg := &AssignmentConfig{URL: "https://gitlab.example.org/mpd/ss26/blatt-01"}
	out := captureStdout(t, func() { cfg.Urls(true) })
	if out != "https://gitlab.example.org/mpd/ss26/blatt-01\n" {
		t.Fatalf("Urls(true) = %q", out)
	}
}

func TestUrls_PerStudent(t *testing.T) {
	alice := "alice"
	bob := "bob"
	cfg := &AssignmentConfig{
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: true,
		Per:                   PerStudent,
		Students: []*Student{
			{Username: &alice, Raw: "alice"},
			{Username: &bob, Raw: "bob"},
		},
	}
	out := captureStdout(t, func() { cfg.Urls(false) })
	want := fmt.Sprintf("%s/%s\n%s/%s\n",
		cfg.URL, cfg.RepoNameForStudent(cfg.Students[0]),
		cfg.URL, cfg.RepoNameForStudent(cfg.Students[1]),
	)
	if out != want {
		t.Fatalf("Urls(PerStudent) = %q, want %q", out, want)
	}
}

func TestUrls_PerGroup(t *testing.T) {
	cfg := &AssignmentConfig{
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Name:                  "blatt01",
		Course:                "mpd",
		UseCoursenameAsPrefix: true,
		Per:                   PerGroup,
		Groups: []*Group{
			{Name: "team1"},
			{Name: "team2"},
		},
	}
	out := captureStdout(t, func() { cfg.Urls(false) })
	want := fmt.Sprintf("%s/%s\n%s/%s\n",
		cfg.URL, cfg.RepoNameForGroup(cfg.Groups[0]),
		cfg.URL, cfg.RepoNameForGroup(cfg.Groups[1]),
	)
	if out != want {
		t.Fatalf("Urls(PerGroup) = %q, want %q", out, want)
	}
}

func TestShow_Minimal(t *testing.T) {
	cfg := &AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	// Show() should not panic with minimal config
	cfg.Show()
}

func TestShow_ContainerRegistryEnabled(t *testing.T) {
	cfg := &AssignmentConfig{ContainerRegistry: true}
	cfg.Show()
}

func TestShow_WithStartercode_NoIssues(t *testing.T) {
	cfg := &AssignmentConfig{
		Startercode: &Startercode{
			URL:        "https://gitlab.example.org/starter",
			FromBranch: "main",
			ToBranch:   "main",
		},
	}
	cfg.Show()
}

func TestShow_WithStartercode_AdditionalBranches(t *testing.T) {
	cfg := &AssignmentConfig{
		Startercode: &Startercode{
			URL:                "https://gitlab.example.org/starter",
			FromBranch:         "main",
			ToBranch:           "main",
			AdditionalBranches: []string{"solution", "startercode"},
		},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "AdditionalBranches") || !strings.Contains(out, "solution") {
		t.Fatalf("Show() output does not contain startercode additional branches: %q", out)
	}
}

func TestShow_WithStartercode_WithIssues(t *testing.T) {
	cfg := &AssignmentConfig{
		Startercode: &Startercode{
			URL:        "https://gitlab.example.org/starter",
			FromBranch: "main",
			ToBranch:   "main",
		},
		Issues: &IssueReplication{
			ReplicateFromStartercode: true,
			IssueNumbers:             []int{1, 2, 3},
		},
	}
	cfg.Show()
}

func TestShow_WithBranches(t *testing.T) {
	cfg := &AssignmentConfig{
		Branches: []BranchRule{{Name: "main", Protect: true, Default: true}, {Name: "develop", MergeOnly: true}},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "Branches:") || !strings.Contains(out, "develop") {
		t.Fatalf("Show() output does not contain branches block: %q", out)
	}
	if !strings.Contains(out, "mergeOnly") || !strings.Contains(out, "default") {
		t.Fatalf("Show() output does not contain compact branch flags: %q", out)
	}
}

func TestShow_WithSeeder(t *testing.T) {
	cfg := &AssignmentConfig{
		Seeder: &Seeder{
			Command:         "make",
			Args:            []string{"build"},
			Name:            "Bot",
			EMail:           "bot@example.com",
			ToBranch:        "main",
			ProtectToBranch: true,
		},
	}
	cfg.Show()
}

func TestShow_WithClone(t *testing.T) {
	cfg := &AssignmentConfig{
		Clone: &Clone{LocalPath: "/tmp/repos", Branch: "main", Force: true},
	}
	cfg.Show()
}

func TestShow_WithRelease_MergeRequestAndDockerImages(t *testing.T) {
	cfg := &AssignmentConfig{
		Release: &Release{
			MergeRequest: &ReleaseMergeRequest{
				SourceBranch: "develop",
				TargetBranch: "main",
				HasPipeline:  true,
			},
			DockerImages: []string{"myimage:latest", "myimage:1.0"},
		},
	}
	cfg.Show()
}

func TestShow_WithRelease_MergeRequestOnly(t *testing.T) {
	cfg := &AssignmentConfig{
		Release: &Release{
			MergeRequest: &ReleaseMergeRequest{
				SourceBranch: "develop",
				TargetBranch: "main",
			},
		},
	}
	cfg.Show()
}

func TestShow_WithRelease_DockerImagesOnly(t *testing.T) {
	cfg := &AssignmentConfig{
		Release: &Release{
			DockerImages: []string{"myimage:latest"},
		},
	}
	cfg.Show()
}

func TestShow_OutputContainsCourseName(t *testing.T) {
	cfg := &AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "mpd") {
		t.Fatalf("Show() output does not contain course name %q: %q", "mpd", out)
	}
}

func TestShow_PerStudent_ListsStudents(t *testing.T) {
	alice := "alice"
	bob := "bob"
	cfg := &AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Per:    PerStudent,
		Students: []*Student{
			{Username: &alice, Raw: "alice"},
			{Username: &bob, Raw: "bob"},
		},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "alice") {
		t.Fatalf("Show(PerStudent) output does not contain student alice: %q", out)
	}
	if !strings.Contains(out, "bob") {
		t.Fatalf("Show(PerStudent) output does not contain student bob: %q", out)
	}
}

func TestShow_PerGroup_ListsGroups(t *testing.T) {
	alice := "alice"
	bob := "bob"
	cfg := &AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Per:    PerGroup,
		Groups: []*Group{
			{Name: "team1", Members: []*Student{{Username: &alice, Raw: "alice"}, {Username: &bob, Raw: "bob"}}},
			{Name: "team2", Members: []*Student{{Username: &alice, Raw: "carol"}}},
		},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "team1") {
		t.Fatalf("Show(PerGroup) output does not contain team1: %q", out)
	}
}

func TestShow_WithMergeMethod(t *testing.T) {
	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{MergeMethod: SemiLinearHistory},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "MergeRequest:") {
		t.Fatalf("Show() output does not contain MergeRequest header: %q", out)
	}
	if !strings.Contains(out, "MergeMethod:") {
		t.Fatalf("Show() output does not contain nested MergeMethod key: %q", out)
	}
	if !strings.Contains(out, "semi_linear") {
		t.Fatalf("Show() output does not contain merge method semi_linear: %q", out)
	}
}

func TestShow_WithSquashOption(t *testing.T) {
	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{MergeMethod: MergeCommit, SquashOption: SquashAlways},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "SquashOption:") {
		t.Fatalf("Show() output does not contain SquashOption key: %q", out)
	}
	if !strings.Contains(out, "always") {
		t.Fatalf("Show() output does not contain squash option 'always': %q", out)
	}
}

func TestShow_WithMergeChecks(t *testing.T) {
	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{
			MergeMethod:                   MergeCommit,
			SquashOption:                  SquashDefaultOff,
			PipelineMustSucceed:           true,
			SkippedPipelinesAreSuccessful: true,
			AllThreadsMustBeResolved:      true,
			StatusChecksMustSucceed:       true,
		},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "PipelineMustSucceed:") || !strings.Contains(out, "true") {
		t.Fatalf("Show() output does not contain PipelineMustSucceed=true: %q", out)
	}
	if !strings.Contains(out, "AllThreadsMustBeResolved:") {
		t.Fatalf("Show() output does not contain AllThreadsMustBeResolved key: %q", out)
	}
	if !strings.Contains(out, "SkippedPipelinesAreSuccessful:") {
		t.Fatalf("Show() output does not contain SkippedPipelinesAreSuccessful key: %q", out)
	}
	if !strings.Contains(out, "StatusChecksMustSucceed:") {
		t.Fatalf("Show() output does not contain StatusChecksMustSucceed key: %q", out)
	}
}

func TestShow_NoFmtArtifacts(t *testing.T) {
	cfg := &AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		UseCoursenameAsPrefix: true,
		Per:                   PerStudent,
		MergeRequest: &MergeRequest{
			PipelineMustSucceed:           true,
			SkippedPipelinesAreSuccessful: true,
			AllThreadsMustBeResolved:      true,
			StatusChecksMustSucceed:       true,
		},
		Branches: []BranchRule{{Name: "main", MergeOnly: true, Default: true}},
		Issues: &IssueReplication{
			ReplicateFromStartercode: true,
			IssueNumbers:             []int{1},
		},
		Clone: &Clone{LocalPath: ".", Branch: "main", Force: false},
	}

	out := captureStdout(t, func() { cfg.Show() })
	if strings.Contains(out, "%!") {
		t.Fatalf("Show() output contains fmt artifacts: %q", out)
	}
}
