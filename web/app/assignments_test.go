package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/web/db"
)

func TestAssignmentSchema(t *testing.T) {
	fields := AssignmentSchema()
	if len(fields) == 0 {
		t.Fatal("expected a non-empty schema")
	}
	byKey := map[string]FieldMeta{}
	for _, f := range fields {
		byKey[f.Key] = f
	}

	per, ok := byKey["per"]
	if !ok {
		t.Fatal("expected a 'per' field")
	}
	if per.Kind != KindEnum || len(per.Options) != 2 {
		t.Errorf("per should be an enum with 2 options, got kind=%s options=%d", per.Kind, len(per.Options))
	}
	if !per.Required {
		t.Error("per should be required")
	}

	al, ok := byKey["accesslevel"]
	if !ok || al.Kind != KindEnum {
		t.Fatal("expected an 'accesslevel' enum field")
	}
	// Per-option descriptions are the whole point of server-authored metadata.
	for _, o := range al.Options {
		if o.Description == "" {
			t.Errorf("accesslevel option %q is missing a description", o.Value)
		}
	}
}

const tcCourse = `tc:
  coursepath: tc/sem
  semesterpath: ss26
  base:
    abstract: true
    per: student
    accesslevel: developer
  blatt1:
    extends: base
    description: First sheet
`

func storedCourse(t *testing.T, owner, raw string) *db.StoredCourse {
	t.Helper()
	src, _, err := config.DecodeCourse([]byte(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return &db.StoredCourse{Owner: owner, Name: src.Name, Source: src, RawYAML: []byte(raw)}
}

func TestAssignment_resolvesWithInheritance(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gitlab.example"}
	ctx := ctxAs(owner)

	// A concrete assignment resolves and shows inherited values (per=student
	// comes from `base`).
	v, err := a.Assignment(ctx, "tc", "blatt1")
	if err != nil {
		t.Fatalf("Assignment: %v", err)
	}
	if v == nil {
		t.Fatal("expected a view for blatt1")
	}
	if v.Extends != "base" {
		t.Errorf("extends = %q, want base", v.Extends)
	}
	if v.ResolveError != "" {
		t.Errorf("unexpected resolveError: %s", v.ResolveError)
	}
	if !strings.Contains(v.Resolved, "student") {
		t.Errorf("resolved preview should reflect inherited per=student:\n%s", v.Resolved)
	}

	// Own holds the source values (what the user wrote), not the inherited ones:
	// blatt1 sets description and extends but NOT per (that comes from base).
	own := map[string]string{}
	for _, fv := range v.Own {
		own[fv.Key] = fv.Value
	}
	if own["extends"] != "base" {
		t.Errorf("own[extends] = %q, want base", own["extends"])
	}
	if own["description"] != "First sheet" {
		t.Errorf("own[description] = %q, want 'First sheet'", own["description"])
	}
	if own["per"] != "" {
		t.Errorf("own[per] = %q, want empty (per is inherited, not set on blatt1)", own["per"])
	}
	if own["abstract"] != "false" {
		t.Errorf("own[abstract] = %q, want false", own["abstract"])
	}
}

func TestAssignment_abstractBaseReportsResolveError(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs}
	ctx := ctxAs(owner)

	b, err := a.Assignment(ctx, "tc", "base")
	if err != nil {
		t.Fatalf("Assignment: %v", err)
	}
	if b == nil {
		t.Fatal("expected a view for the abstract base")
	}
	if !b.Abstract {
		t.Error("base should be marked abstract")
	}
	if b.ResolveError == "" {
		t.Error("an abstract base should resolve to a ResolveError, not a preview")
	}
	if b.Resolved != "" {
		t.Errorf("abstract base should have no resolved preview, got:\n%s", b.Resolved)
	}
}

func TestAssignment_unknownAssignmentAndCourse(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs}
	ctx := ctxAs(owner)

	// Course exists, assignment does not → nil, nil (GraphQL null).
	n, err := a.Assignment(ctx, "tc", "does-not-exist")
	if err != nil {
		t.Fatalf("Assignment: %v", err)
	}
	if n != nil {
		t.Errorf("expected nil for an unknown assignment, got %+v", n)
	}

	// Course not the caller's → ErrCourseNotFound propagates.
	if _, err := a.Assignment(ctx, "other", "x"); !errors.Is(err, db.ErrCourseNotFound) {
		t.Errorf("expected ErrCourseNotFound, got %v", err)
	}
}
