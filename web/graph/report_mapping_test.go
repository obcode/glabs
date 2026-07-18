package graph

import (
	"testing"
	"time"

	"github.com/obcode/glabs/v3/gitlab/report"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func TestToGraphAssignmentReport(t *testing.T) {
	now := time.Now()
	rep := &report.Reports{
		Course:                 "mpd",
		Assignment:             "blatt1",
		URL:                    "https://gl/mpd/x",
		Description:            "d",
		Generated:              &now,
		HasReleaseMergeRequest: true,
		Projects: []*report.ProjectReport{{
			Name:     "blatt1-alice",
			IsActive: true,
			Commits:  3,
			WebURL:   "https://gl/p",
			Members: []*gitlab.ProjectMember{
				{Name: "Alice", Username: "alice", WebURL: "https://gl/u/alice"},
			},
			LastCommit: &report.Commit{Title: "init", CommitterName: "Alice", WebURL: "https://gl/c"},
			Release: &report.Release{
				MergeRequest: &report.MergeRequest{Found: true, WebURL: "https://gl/mr", PipelineStatus: "success"},
			},
		}},
	}

	out := toGraphAssignmentReport(rep)
	if out.Course != "mpd" || out.Assignment != "blatt1" || !out.HasReleaseMergeRequest {
		t.Fatalf("top-level fields wrong: %+v", out)
	}
	if len(out.Projects) != 1 {
		t.Fatalf("projects = %d, want 1", len(out.Projects))
	}
	p := out.Projects[0]
	if p.Name != "blatt1-alice" || !p.Active || p.Commits != 3 {
		t.Errorf("project fields wrong: %+v", p)
	}
	if len(p.Members) != 1 || p.Members[0].Username != "alice" || p.Members[0].Name != "Alice" {
		t.Errorf("members wrong: %+v", p.Members)
	}
	if p.LastCommit == nil || p.LastCommit.Title != "init" {
		t.Errorf("lastCommit wrong: %+v", p.LastCommit)
	}
	if p.Release == nil || p.Release.MergeRequest == nil || !p.Release.MergeRequest.Found {
		t.Errorf("release wrong: %+v", p.Release)
	}
}
