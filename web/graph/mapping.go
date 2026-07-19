package graph

import (
	"sort"

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
