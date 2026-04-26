package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
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
		Branches: []BranchRule{{Name: "main", Protect: true, Default: true, AllowForcePush: true}, {Name: "develop", MergeOnly: true, CodeOwnerApprovalRequired: true}},
	}
	out := captureStdout(t, func() { cfg.Show() })
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	plain := ansiPattern.ReplaceAllString(out, "")
	if !strings.Contains(out, "Branches:") || !strings.Contains(out, "develop") {
		t.Fatalf("Show() output does not contain branches block: %q", out)
	}
	if !strings.Contains(plain, "- main:") || !strings.Contains(plain, "protect, default, allowForcePush") {
		t.Fatalf("Show() output does not contain enabled branch flags for main: %q", plain)
	}
	if !strings.Contains(plain, "- develop:") || !strings.Contains(plain, "mergeOnly, codeOwnerApprovalRequired") {
		t.Fatalf("Show() output does not contain enabled branch flags for develop: %q", plain)
	}
	if strings.Contains(plain, "protect=false") || strings.Contains(plain, "mergeOnly=false") || strings.Contains(plain, "default=false") {
		t.Fatalf("Show() output still contains disabled branch flags: %q", plain)
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
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	plain := ansiPattern.ReplaceAllString(out, "")
	if !strings.Contains(plain, "- team1 (2): alice, bob") {
		t.Fatalf("Show(PerGroup) output does not contain inline group count for team1: %q", plain)
	}
	if !strings.Contains(plain, "- team2 (1): carol") {
		t.Fatalf("Show(PerGroup) output does not contain inline group count for team2: %q", plain)
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

func TestShow_WithMergeRequestApprovals(t *testing.T) {
	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{
			Approvals: []MergeRequestApprovalRule{{
				Name:                  "review-main",
				Branches:              []string{"main", "develop"},
				Usernames:             []string{"alice"},
				Groups:                []string{"mpd/tutors"},
				MultiMemberGroupsOnly: true,
				RequiredApprovals:     2,
			}},
		},
	}
	out := captureStdout(t, func() { cfg.Show() })
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	plain := ansiPattern.ReplaceAllString(out, "")
	if !strings.Contains(out, "Approvals:") {
		t.Fatalf("Show() output does not contain Approvals label: %q", out)
	}
	if !strings.Contains(out, "Settings:") {
		t.Fatalf("Show() output does not contain approval settings header: %q", out)
	}
	if !strings.Contains(out, "Rules:") {
		t.Fatalf("Show() output does not contain approval rules header: %q", out)
	}
	if !strings.Contains(out, "review-main") {
		t.Fatalf("Show() output does not contain approval rule name: %q", out)
	}
	if !strings.Contains(plain, "\n      - review-main:") {
		t.Fatalf("Show() output does not contain aligned approval rule label: %q", plain)
	}
	if !strings.Contains(out, "requiredApprovals") {
		t.Fatalf("Show() output does not contain requiredApprovals field: %q", out)
	}
	if !strings.Contains(out, "branches") {
		t.Fatalf("Show() output does not contain branches field for approval rule: %q", out)
	}
	if !strings.Contains(out, "usernames") {
		t.Fatalf("Show() output does not contain usernames field for approval rule: %q", out)
	}
	if !strings.Contains(out, "multiMemberGroupsOnly") {
		t.Fatalf("Show() output does not contain multiMemberGroupsOnly field for approval rule: %q", out)
	}
}

func TestShow_WithMergeRequestApprovals_OmitsEmptyUsersAndGroups(t *testing.T) {
	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{
			Approvals: []MergeRequestApprovalRule{{
				Name:              "review-main",
				Branches:          []string{"main"},
				Usernames:         []string{},
				Groups:            []string{},
				RequiredApprovals: 1,
			}},
		},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if strings.Contains(out, "usernames") {
		t.Fatalf("Show() output should omit empty usernames field for approval rule: %q", out)
	}
	if strings.Contains(out, "groups") {
		t.Fatalf("Show() output should omit empty groups field for approval rule: %q", out)
	}
	if !strings.Contains(out, "branches") || !strings.Contains(out, "requiredApprovals") {
		t.Fatalf("Show() output should keep non-empty approval rule fields: %q", out)
	}
}

func TestShow_WithMergeRequestApprovalSettings(t *testing.T) {
	preventCreator := true
	preventCommitters := true
	preventEditing := true
	reauth := true
	whenCommitAdded := ApprovalRemoveAllApprovals

	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{
			ApprovalSettings: &MergeRequestApprovalSettings{
				PreventApprovalByMergeRequestCreator:       &preventCreator,
				PreventApprovalsByUsersWhoAddCommits:       &preventCommitters,
				PreventEditingApprovalRulesInMergeRequests: &preventEditing,
				RequireUserReauthenticationToApprove:       &reauth,
				WhenCommitAdded:                            &whenCommitAdded,
			},
		},
	}
	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "Approvals:") || !strings.Contains(out, "Settings:") {
		t.Fatalf("Show() output does not contain nested approvals settings block: %q", out)
	}
	if !strings.Contains(out, "PreventApprovalByMergeRequestCreator") {
		t.Fatalf("Show() output does not contain approval settings key: %q", out)
	}
	if !strings.Contains(out, "WhenCommitAdded") || !strings.Contains(out, string(ApprovalRemoveAllApprovals)) {
		t.Fatalf("Show() output does not contain approval whenCommitAdded setting: %q", out)
	}
}

func TestShow_WithUndefinedApprovalsBlocks(t *testing.T) {
	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{},
	}

	out := captureStdout(t, func() { cfg.Show() })
	if !strings.Contains(out, "Approvals:") || !strings.Contains(out, "Settings:") || !strings.Contains(out, "Rules:") {
		t.Fatalf("Show() output does not contain nested approvals headers: %q", out)
	}
	if strings.Count(out, "not defined") < 2 {
		t.Fatalf("Show() output does not contain not defined markers for empty approvals blocks: %q", out)
	}
}

func TestShow_MergeRequestValuesAlign(t *testing.T) {
	cfg := &AssignmentConfig{
		MergeRequest: &MergeRequest{
			MergeMethod:                   SemiLinearHistory,
			SquashOption:                  SquashNever,
			PipelineMustSucceed:           true,
			SkippedPipelinesAreSuccessful: true,
			AllThreadsMustBeResolved:      true,
			StatusChecksMustSucceed:       true,
		},
	}

	out := captureStdout(t, func() { cfg.Show() })
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	plain := ansiPattern.ReplaceAllString(out, "")
	lines := strings.Split(plain, "\n")

	var mergeMethodLine, skippedLine, statusChecksLine string
	for _, line := range lines {
		switch {
		case strings.Contains(line, "MergeMethod:"):
			mergeMethodLine = line
		case strings.Contains(line, "SkippedPipelinesAreSuccessful:"):
			skippedLine = line
		case strings.Contains(line, "StatusChecksMustSucceed:"):
			statusChecksLine = line
		}
	}

	if mergeMethodLine == "" || skippedLine == "" || statusChecksLine == "" {
		t.Fatalf("Show() output is missing merge request lines: %q", plain)
	}

	mergeMethodValueIndex := strings.Index(mergeMethodLine, string(SemiLinearHistory))
	skippedValueIndex := strings.Index(skippedLine, "true")
	statusChecksValueIndex := strings.Index(statusChecksLine, "true")
	if mergeMethodValueIndex < 0 || skippedValueIndex < 0 || statusChecksValueIndex < 0 {
		t.Fatalf("Show() output is missing expected values: %q", plain)
	}

	if mergeMethodValueIndex != skippedValueIndex || mergeMethodValueIndex != statusChecksValueIndex {
		t.Fatalf(
			"Show() merge request values are not aligned: mergeMethod=%d skipped=%d statusChecks=%d in %q",
			mergeMethodValueIndex,
			skippedValueIndex,
			statusChecksValueIndex,
			plain,
		)
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
