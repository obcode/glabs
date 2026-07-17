package config

import (
	"fmt"
	"strings"
)

// Naming of the repositories an assignment generates. These are methods on the
// resolved config, so they are shared by every caller — CLI, reports, URLs —
// and there is exactly one place that decides what a repository is called.

// RepoSuffix returns the part of a repository name that identifies the student.
//
// Using email addresses rather than usernames or ids puts an "@" in the name,
// which is invalid in both a filesystem path and a GitLab path, so it has to be
// replaced.
func (cfg *AssignmentConfig) RepoSuffix(student *Student) string {
	if student.Email != nil {
		// UseEmailDomainAsSuffix defaults to true, so a false here can only come
		// from the course file saying so explicitly.
		if !cfg.UseEmailDomainAsSuffix {
			local, _, _ := strings.Cut(*student.Email, "@")
			return local
		}
		return strings.ReplaceAll(*student.Email, "@", "_at_")
	}
	if student.Id != nil {
		return fmt.Sprint(*student.Id)
	}
	if student.Username != nil {
		return *student.Username
	}
	return ""
}

func (cfg *AssignmentConfig) RepoBaseName() string {
	if cfg.UseCoursenameAsPrefix {
		return fmt.Sprintf("%s-%s", cfg.Course, cfg.Name)
	}

	return cfg.Name
}

// RepoNameWithSuffix returns the project path for an assignment repository.
// The result is normalized via gitlabProjectPath so it matches the path GitLab
// derives from the project name: names that are already valid paths are kept
// as-is, while names containing characters invalid in a path (e.g. "+") are
// slugified the same way GitLab does. Without this, the printed URL and the
// search used to locate the project would not match the repository GitLab
// actually creates.
func (cfg *AssignmentConfig) RepoNameWithSuffix(suffix string) string {
	return gitlabProjectPath(fmt.Sprintf("%s-%s", cfg.RepoBaseName(), suffix))
}

func (cfg *AssignmentConfig) RepoNameForStudent(student *Student) string {
	return cfg.RepoNameWithSuffix(cfg.RepoSuffix(student))
}

func (cfg *AssignmentConfig) RepoNameForGroup(group *Group) string {
	return cfg.RepoNameWithSuffix(group.Name)
}
