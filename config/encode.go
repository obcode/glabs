package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// yamlIndent matches the indentation of the existing course files. yaml.v3
// defaults to 4, which would reformat every line of every file.
const yamlIndent = 2

// EncodeCourse renders a course source back to a course file: a single
// top-level key naming the course, with the course mapping underneath.
//
// This is the canonical form. Deprecated spellings are gone (decoding folded
// them into their canonical field) and polymorphic blocks come out in their
// modern shape — which is what makes `glabs config fmt` and `glabs config
// migrate` fall out of the schema rather than needing their own rewriting pass.
//
// Comments and key order are NOT preserved: a struct has neither. Callers that
// must not lose them should keep the original bytes and only re-encode once the
// source has actually been edited.
func EncodeCourse(course *CourseSource) ([]byte, error) {
	if course == nil {
		return nil, fmt.Errorf("cannot encode a nil course")
	}
	if course.Name == "" {
		return nil, fmt.Errorf("cannot encode a course without a name: it is the file's top-level key")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(yamlIndent)

	if err := enc.Encode(map[string]*CourseSource{course.Name: course}); err != nil {
		return nil, fmt.Errorf("course %s: cannot encode configuration: %w", course.Name, err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("course %s: cannot finish encoding: %w", course.Name, err)
	}
	return buf.Bytes(), nil
}
