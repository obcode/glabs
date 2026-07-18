package graph

import (
	"github.com/obcode/glabs/v3/gitlab/report"
	"github.com/obcode/glabs/v3/web/graph/model"
)

// Mappers for the assignment report. They project the internal report types
// (which carry raw GitLab API structs, e.g. *gitlab.ProjectMember) onto the
// GraphQL model, exposing only the display fields — never GitLab-internal ids.

func toGraphAssignmentReport(rep *report.Reports) *model.AssignmentReport {
	projects := make([]*model.ProjectReport, 0, len(rep.Projects))
	for _, p := range rep.Projects {
		if p == nil {
			continue
		}
		projects = append(projects, toGraphProjectReport(p))
	}
	return &model.AssignmentReport{
		Course:                 rep.Course,
		Assignment:             rep.Assignment,
		URL:                    rep.URL,
		Description:            rep.Description,
		Generated:              rep.Generated,
		HasReleaseMergeRequest: rep.HasReleaseMergeRequest,
		HasReleaseDockerImages: rep.HasReleaseDockerImages,
		Projects:               projects,
	}
}

func toGraphProjectReport(p *report.ProjectReport) *model.ProjectReport {
	members := make([]*model.ProjectMemberReport, 0, len(p.Members))
	for _, m := range p.Members {
		if m == nil {
			continue
		}
		members = append(members, &model.ProjectMemberReport{
			Name:     m.Name,
			Username: m.Username,
			WebURL:   m.WebURL,
		})
	}

	var lastCommit *model.CommitReport
	if p.LastCommit != nil {
		lastCommit = &model.CommitReport{
			Title:         p.LastCommit.Title,
			CommitterName: p.LastCommit.CommitterName,
			CommittedDate: p.LastCommit.CommittedDate,
			WebURL:        p.LastCommit.WebURL,
		}
	}

	return &model.ProjectReport{
		Name:                   p.Name,
		Active:                 p.IsActive,
		EmptyRepo:              p.IsEmpty,
		Commits:                p.Commits,
		CreatedAt:              p.CreatedAt,
		LastActivity:           p.LastActivity,
		LastCommit:             lastCommit,
		OpenIssuesCount:        p.OpenIssuesCount,
		OpenMergeRequestsCount: p.OpenMergeRequestsCount,
		WebURL:                 p.WebURL,
		Members:                members,
		Release:                toGraphReleaseReport(p.Release),
	}
}

func toGraphReleaseReport(rel *report.Release) *model.ReleaseReport {
	if rel == nil {
		return nil
	}
	out := &model.ReleaseReport{}
	if rel.MergeRequest != nil {
		out.MergeRequest = &model.ReleaseMergeRequestReport{
			Found:          rel.MergeRequest.Found,
			WebURL:         rel.MergeRequest.WebURL,
			PipelineStatus: rel.MergeRequest.PipelineStatus,
		}
	}
	if rel.DockerImages != nil {
		images := make([]*model.DockerImageReport, 0, len(rel.DockerImages.Images))
		for _, img := range rel.DockerImages.Images {
			if img == nil {
				continue
			}
			images = append(images, &model.DockerImageReport{Wanted: img.Wanted, Image: img.Image})
		}
		out.DockerImages = &model.DockerImagesReport{Status: rel.DockerImages.Status, Images: images}
	}
	return out
}
