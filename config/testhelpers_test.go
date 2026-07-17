package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

// registerCourse writes an inline course document and registers it, so the
// package's public entry points resolve against it.
//
// The tests used to build their input with a pile of viper.Set calls against the
// global registry. Course files no longer go through viper, and writing the YAML
// out is closer to what a user actually has anyway: the setup now reads like the
// config it is testing.
func registerCourse(t *testing.T, doc string) string {
	t.Helper()

	ResetCourses()
	t.Cleanup(ResetCourses)
	resetViper(t)
	viper.Set("gitlab.host", goldenHost)

	path := filepath.Join(t.TempDir(), "course.yaml")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("writing inline course: %v", err)
	}
	name, err := LoadCourseFile(path)
	if err != nil {
		t.Fatalf("LoadCourseFile: %v", err)
	}
	return name
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
