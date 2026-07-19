package graph

import (
	"sort"
	"strings"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/web/app"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
)

// Mappers from the internal types to the curated GraphQL models. They live here
// rather than in a *.resolvers.go file so gqlgen, which owns those files, leaves
// them alone.

// toGraphCourse projects a stored course onto the GraphQL type. The full
// CourseSource stays in the store; the API exposes what the GUI needs to list and
// inspect a course, with the YAML and lint on their own queries.
func toGraphCourse(s *db.StoredCourse) *model.Course {
	c := &model.Course{
		Name:       s.Name,
		ImportedAt: s.ImportedAt,
		UpdatedAt:  s.UpdatedAt,
	}
	if s.Source != nil {
		c.CoursePath = s.Source.CoursePath
		c.SemesterPath = s.Source.SemesterPath
		c.UseCoursenameAsPrefix = s.Source.UseCoursenameAsPrefix
		// Absent means true (see config.CourseSource.UseEmailDomainAsSuffix).
		c.UseEmailDomainAsSuffix = s.Source.UseEmailDomainAsSuffix == nil || *s.Source.UseEmailDomainAsSuffix
		c.StudentCount = len(s.Source.Students)
		c.GroupCount = len(s.Source.Groups)
		names := make([]string, 0, len(s.Source.Assignments))
		for name := range s.Source.Assignments {
			names = append(names, name)
		}
		sort.Strings(names)
		c.AssignmentNames = names

		c.Students = append([]string{}, s.Source.Students...)
		groupNames := make([]string, 0, len(s.Source.Groups))
		for gname := range s.Source.Groups {
			groupNames = append(groupNames, gname)
		}
		sort.Strings(groupNames)
		c.Groups = make([]*model.Group, 0, len(groupNames))
		for _, gname := range groupNames {
			c.Groups = append(c.Groups, &model.Group{
				Name:    gname,
				Members: append([]string{}, s.Source.Groups[gname]...),
			})
		}
	}
	if c.Students == nil {
		c.Students = []string{}
	}
	if c.Groups == nil {
		c.Groups = []*model.Group{}
	}
	return c
}

func toGraphSeverity(s config.Severity) model.FindingSeverity {
	if s == config.SeverityProblem {
		return model.FindingSeverityProblem
	}
	return model.FindingSeverityDeprecated
}

func toGraphTokenStatus(s *app.GitLabTokenStatus) *model.GitLabTokenStatus {
	return &model.GitLabTokenStatus{Set: s.Set, UpdatedAt: s.UpdatedAt}
}

// toGraphJob projects a scheduled job onto the GraphQL type. The status string
// maps to the enum by upper-casing (the enum values are the upper-case forms of
// the stored lower-case statuses).
func toGraphJob(j *db.ScheduledJob) *model.ScheduledJob {
	keys := make([]string, 0, len(j.Params))
	for k := range j.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	params := make([]*model.JobParam, 0, len(keys))
	for _, k := range keys {
		params = append(params, &model.JobParam{Key: k, Value: j.Params[k]})
	}
	onlyFor := j.OnlyFor
	if onlyFor == nil {
		onlyFor = []string{}
	}
	return &model.ScheduledJob{
		ID:           j.ID,
		Op:           j.Op,
		Course:       j.Course,
		Assignment:   j.Assignment,
		OnlyFor:      onlyFor,
		Params:       params,
		RunAt:        j.RunAt,
		GraceMinutes: j.GraceMin,
		Status:       model.JobStatus(strings.ToUpper(j.Status)),
		CreatedAt:    j.CreatedAt,
		StartedAt:    j.StartedAt,
		FinishedAt:   j.FinishedAt,
		Err:          emptyToNil(j.Err),
	}
}

func toGraphJobs(jobs []*db.ScheduledJob) []*model.ScheduledJob {
	out := make([]*model.ScheduledJob, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, toGraphJob(j))
	}
	return out
}

// toDBStatuses lower-cases the GraphQL enum values back to the stored status
// strings for a query filter.
func toDBStatuses(in []model.JobStatus) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(string(s)))
	}
	return out
}

// toGraphActivity projects the activity log onto the GraphQL type.
func toGraphActivity(entries []*db.ActivityEntry) []*model.ActivityEntry {
	out := make([]*model.ActivityEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, &model.ActivityEntry{
			Assignment: e.Assignment,
			Op:         e.Op,
			Status:     e.Status,
			Detail:     e.Detail,
			At:         e.At,
		})
	}
	return out
}
