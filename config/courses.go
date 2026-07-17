package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// The CLI's view of the loaded course files.
//
// Course files used to be merged into the global viper registry alongside
// gitlab.host, the token and every other setting, which meant a course named
// `courses` would have collided with the course list, resolution mutated global
// state, and nothing could be loaded without running a cobra command. They now
// live here, as the bodies they were written as, and resolution is a pure
// function over them (config/resolve.go).
//
// viper still owns the main config — gitlab.host, gitlab.token, coursesfilepath
// — which is genuinely global process configuration. This file is the only place
// in the package that touches it.

var courses = struct {
	sync.RWMutex
	byName map[string]any
}{byName: map[string]any{}}

// LoadCourseFile reads a course file and registers its contents under the
// course name declared inside it.
func LoadCourseFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read course file %s: %w", path, err)
	}

	var top map[string]any
	if err := yaml.Unmarshal(data, &top); err != nil {
		return "", fmt.Errorf("%s: cannot parse course file as YAML: %w", path, err)
	}
	if len(top) != 1 {
		return "", fmt.Errorf("%s: course file has %d top-level keys, expected exactly one naming the course",
			path, len(top))
	}

	for name, body := range top {
		if body == nil {
			return "", fmt.Errorf("%s: course %s is empty", path, name)
		}
		courses.Lock()
		courses.byName[name] = body
		courses.Unlock()
		return name, nil
	}
	return "", nil
}

// ResetCourses drops every loaded course. Tests use it; the CLI loads once.
func ResetCourses() {
	courses.Lock()
	courses.byName = map[string]any{}
	courses.Unlock()
}

// courseBody looks a course up by name, folding case the way viper's registry
// did.
func courseBody(course string) (any, bool) {
	courses.RLock()
	defer courses.RUnlock()

	if body, ok := courses.byName[course]; ok {
		return body, true
	}
	for name, body := range courses.byName {
		if strings.EqualFold(name, course) {
			return body, true
		}
	}
	return nil, false
}

// CourseExists reports whether a course configuration was loaded.
func CourseExists(course string) bool {
	_, ok := courseBody(course)
	return ok
}

func globals() Globals {
	return Globals{GitlabHost: viper.GetString("gitlab.host")}
}

// GetAssignmentConfig resolves an assignment from the loaded course files.
func GetAssignmentConfig(course, assignment string, onlyForStudentsOrGroups ...string) (*AssignmentConfig, error) {
	body, ok := courseBody(course)
	if !ok {
		return nil, fmt.Errorf("configuration for course %s not found", course)
	}
	return ResolveAssignment(course, body, globals(), assignment, onlyForStudentsOrGroups...)
}

// GetCourseConfig resolves the course-level students and groups.
func GetCourseConfig(course string) (*CourseConfig, error) {
	body, ok := courseBody(course)
	if !ok {
		return nil, fmt.Errorf("configuration for course %s not found", course)
	}
	return ResolveCourse(course, body)
}

// GetCourseSubgroupPath returns the full path to the course subgroup
// (coursepath/semesterpath), for Dependency-Proxy and other group-level features.
func GetCourseSubgroupPath(course string) string {
	body, ok := courseBody(course)
	if !ok {
		return ""
	}
	path, err := ResolveCoursePath(course, body)
	if err != nil {
		return ""
	}
	return path
}

// GetCourseURL prints the course subgroup URL.
func GetCourseURL(course string) {
	fmt.Printf("%s/%s\n", viper.GetString("gitlab.host"), GetCourseSubgroupPath(course))
}
