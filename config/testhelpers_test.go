package config

import (
	"testing"

	"github.com/spf13/viper"
)

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

// The must* helpers keep the existing happy-path tests readable now that config
// loading returns errors. Error paths are asserted explicitly in
// errors_test.go, which is only possible at all because these no longer exit
// the process.

func mustAssignmentConfig(t *testing.T, course, assignment string, onlyFor ...string) *AssignmentConfig {
	t.Helper()
	cfg, err := GetAssignmentConfig(course, assignment, onlyFor...)
	if err != nil {
		t.Fatalf("GetAssignmentConfig(%q, %q): unexpected error: %v", course, assignment, err)
	}
	return cfg
}

func mustCourseConfig(t *testing.T, course string) *CourseConfig {
	t.Helper()
	cfg, err := GetCourseConfig(course)
	if err != nil {
		t.Fatalf("GetCourseConfig(%q): unexpected error: %v", course, err)
	}
	return cfg
}

func mustStartercode(t *testing.T, assignmentKey string) *Startercode {
	t.Helper()
	s, err := startercode(assignmentKey)
	if err != nil {
		t.Fatalf("startercode(%q): unexpected error: %v", assignmentKey, err)
	}
	return s
}

func mustBranches(t *testing.T, assignmentKey string, starter *Startercode) []BranchRule {
	t.Helper()
	b, err := branches(assignmentKey, starter)
	if err != nil {
		t.Fatalf("branches(%q): unexpected error: %v", assignmentKey, err)
	}
	return b
}

func mustSeeder(t *testing.T, assignmentKey string) *Seeder {
	t.Helper()
	s, err := seeder(assignmentKey)
	if err != nil {
		t.Fatalf("seeder(%q): unexpected error: %v", assignmentKey, err)
	}
	return s
}
