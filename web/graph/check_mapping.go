package graph

import (
	"github.com/obcode/glabs/v3/gitlab"
	"github.com/obcode/glabs/v3/web/graph/model"
)

// Mappers for the course-check result: project the gitlab check types onto the
// GraphQL model.

func toGraphCheckResult(r *gitlab.CheckResult) *model.CheckResult {
	students := make([]*model.StudentCheck, 0, len(r.Students))
	for _, s := range r.Students {
		students = append(students, toGraphStudentCheck(s))
	}
	groups := make([]*model.GroupCheck, 0, len(r.Groups))
	for _, g := range r.Groups {
		members := make([]*model.StudentCheck, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, toGraphStudentCheck(m))
		}
		groups = append(groups, &model.GroupCheck{Name: g.Name, Members: members})
	}
	dups := make([]*model.DuplicateCheck, 0, len(r.Duplicates))
	for _, d := range r.Duplicates {
		dups = append(dups, &model.DuplicateCheck{Student: d.Student, Groups: d.Groups})
	}
	return &model.CheckResult{
		Course:     r.Course,
		Students:   students,
		Groups:     groups,
		Duplicates: dups,
		Errors:     r.Errors,
		Ok:         r.OK,
	}
}

func toGraphStudentCheck(s *gitlab.StudentCheck) *model.StudentCheck {
	return &model.StudentCheck{Input: s.Input, Status: toGraphCheckStatus(s.Status), Message: s.Message}
}

func toGraphCheckStatus(st gitlab.StudentCheckStatus) model.StudentCheckStatus {
	switch st {
	case gitlab.CheckOK:
		return model.StudentCheckStatusOk
	case gitlab.CheckInvite:
		return model.StudentCheckStatusInvite
	case gitlab.CheckDeprecated:
		return model.StudentCheckStatusDeprecated
	default:
		return model.StudentCheckStatusError
	}
}
