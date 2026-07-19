package app

import (
	"testing"

	"github.com/obcode/glabs/v3/config"
)

func TestMatchRepos(t *testing.T) {
	targets := []config.RepoTarget{
		{For: "a@hm.edu", Repo: "c-b01-a", URL: "https://gl/c-b01-a"},
		{For: "b@hm.edu", Repo: "c-b01-b", URL: "https://gl/c-b01-b"},
		{For: "c@hm.edu", Repo: "c-b01-c", URL: "https://gl/c-b01-c"},
	}
	existing := map[string]bool{"c-b01-a": true, "c-b01-c": true} // b is missing

	repos, count := matchRepos(targets, existing)
	if count != 2 {
		t.Errorf("existing count = %d, want 2", count)
	}
	if len(repos) != 3 {
		t.Fatalf("repos = %d, want 3", len(repos))
	}
	got := map[string]bool{}
	for _, r := range repos {
		got[r.For] = r.Exists
	}
	if !got["a@hm.edu"] || got["b@hm.edu"] || !got["c@hm.edu"] {
		t.Errorf("exists flags wrong: %+v", got)
	}
}

func TestMatchRepos_nilExistingMarksAllMissing(t *testing.T) {
	targets := []config.RepoTarget{{For: "a@hm.edu", Repo: "r-a"}, {For: "b@hm.edu", Repo: "r-b"}}
	repos, count := matchRepos(targets, nil)
	if count != 0 {
		t.Errorf("count = %d, want 0 for a nil existing set", count)
	}
	for _, r := range repos {
		if r.Exists {
			t.Errorf("%s should be missing when nothing exists", r.For)
		}
	}
}
