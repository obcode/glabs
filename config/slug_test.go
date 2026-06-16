package config

import "testing"

func TestParameterize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "design-principles-alice", "design-principles-alice"},
		{"underscores preserved", "blatt01-a_at_b", "blatt01-a_at_b"},
		{
			"email with plus and dot",
			"design-principles-fabian+gitinvited_at_hm.edu",
			"design-principles-fabian-gitinvited_at_hm-edu",
		},
		{"at sign", "repo-a@b.com", "repo-a-b-com"},
		{"spaces", "team one", "team-one"},
		{"collapses separators", "a+.+b", "a-b"},
		{"trims separators", "+team1+", "team1"},
		{"downcase", "Repo-Name", "repo-name"},
		{"diacritics", "müller-groß", "muller-gross"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parameterize(tt.in); got != tt.want {
				t.Errorf("parameterize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGitlabProjectPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Already valid paths are kept verbatim — case and dots preserved.
		{"plain", "design-principles-alice", "design-principles-alice"},
		{"dot in domain preserved", "mpd-blatt01-alice_at_example.org", "mpd-blatt01-alice_at_example.org"},
		{"uppercase preserved", "hw01-teamA", "hw01-teamA"},
		// Invalid characters force GitLab to slugify the whole name.
		{
			"plus forces parameterize",
			"design-principles-fabian+gitinvited_at_hm.edu",
			"design-principles-fabian-gitinvited_at_hm-edu",
		},
		{"space forces parameterize", "blatt01-Team One", "blatt01-team-one"},
		{".git suffix forces parameterize", "repo.git", "repo-git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitlabProjectPath(tt.in); got != tt.want {
				t.Errorf("gitlabProjectPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRepoNameWithSuffix_SlugifiesEmail(t *testing.T) {
	cfg := &AssignmentConfig{
		Course:                "homl",
		Name:                  "design-principles",
		UseCoursenameAsPrefix: false,
	}
	got := cfg.RepoNameWithSuffix("fabian+gitinvited_at_hm.edu")
	want := "design-principles-fabian-gitinvited_at_hm-edu"
	if got != want {
		t.Fatalf("RepoNameWithSuffix() = %q, want %q", got, want)
	}
}
