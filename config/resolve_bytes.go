package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ResolveAssignmentFromBytes resolves one assignment straight from a course
// file's raw bytes — the form the web server keeps in `rawYAML`. It parses the
// bytes into the generic body map under the course's single top-level key and
// hands that to ResolveAssignment, so the web reaches the exact same resolution
// (inheritance, legacy-key handling, lowercasing) the CLI does.
func ResolveAssignmentFromBytes(data []byte, courseName, assignment string, g Globals, onlyForStudentsOrGroups ...string) (*AssignmentConfig, error) {
	var top map[string]any
	if err := yaml.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("cannot parse course file: %w", err)
	}
	body, ok := top[courseName]
	if !ok {
		return nil, fmt.Errorf("course %s not found in file", courseName)
	}
	return ResolveAssignment(courseName, body, g, assignment, onlyForStudentsOrGroups...)
}
