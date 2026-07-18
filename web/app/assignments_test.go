package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/web/db"
)

func ownMap(fvs []FieldValue) map[string]string {
	m := map[string]string{}
	for _, fv := range fvs {
		m[fv.Key] = fv.Value
	}
	return m
}

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

func TestValidateAssignmentDraft(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}
	ctx := ctxAs(owner)

	// A valid draft resolves: ok, preview present, no errors.
	vr, err := a.ValidateAssignmentDraft(ctx, "tc", "blatt1", map[string]string{"description": "Changed"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !vr.OK || vr.Resolved == "" || len(vr.Errors) != 0 {
		t.Errorf("valid draft: ok=%v resolvedLen=%d errors=%v", vr.OK, len(vr.Resolved), vr.Errors)
	}

	// A concrete draft that cannot resolve (missing parent) is a hard error.
	bad, err := a.ValidateAssignmentDraft(ctx, "tc", "blatt1", map[string]string{"extends": "nope"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if bad.OK || len(bad.Errors) == 0 {
		t.Errorf("unresolvable draft should be not-ok with errors, got ok=%v errors=%v", bad.OK, bad.Errors)
	}

	// Validation is read-only: nothing persisted, stored source unchanged.
	if fs.saved != nil {
		t.Error("validate must not persist")
	}
	if got := fs.courses[owner+"/tc"].Source.Assignments["blatt1"].Description; got != "First sheet" {
		t.Errorf("validate mutated the stored source: description = %q", got)
	}
}

func TestSetAssignment_persists(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}
	ctx := ctxAs(owner)

	view, err := a.SetAssignment(ctx, "tc", "blatt1", map[string]string{"description": "Changed sheet"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	// The returned view reflects the new own value and still resolves (per is
	// still inherited from base).
	if own := ownMap(view.Own); own["description"] != "Changed sheet" {
		t.Errorf("own[description] = %q, want 'Changed sheet'", own["description"])
	}
	if !strings.Contains(view.Resolved, "student") {
		t.Errorf("resolved preview lost the inherited per=student:\n%s", view.Resolved)
	}

	// Persisted: SaveCourse was called, the source is updated, and rawYAML was
	// re-marshalled to reflect the edit.
	if fs.saved == nil {
		t.Fatal("SetAssignment did not persist")
	}
	if got := fs.saved.Source.Assignments["blatt1"].Description; got != "Changed sheet" {
		t.Errorf("stored source description = %q", got)
	}
	if !bytes.Contains(fs.saved.RawYAML, []byte("Changed sheet")) {
		t.Errorf("rawYAML was not re-marshalled with the edit:\n%s", fs.saved.RawYAML)
	}
}

func TestSetAssignment_startercodeBlock(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}
	ctx := ctxAs(owner)

	// Set a startercode block on blatt1.
	view, err := a.SetAssignment(ctx, "tc", "blatt1", map[string]string{
		"startercode.url":                "git@gl:tc/sc.git",
		"startercode.fromBranch":         "main",
		"startercode.additionalBranches": "dev, test",
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	own := ownMap(view.Own)
	if own["startercode.url"] != "git@gl:tc/sc.git" {
		t.Errorf("own[startercode.url] = %q", own["startercode.url"])
	}
	if own["startercode.additionalBranches"] != "dev, test" {
		t.Errorf("own[startercode.additionalBranches] = %q", own["startercode.additionalBranches"])
	}
	if !bytes.Contains(fs.saved.RawYAML, []byte("tc/sc.git")) {
		t.Errorf("rawYAML missing the startercode url:\n%s", fs.saved.RawYAML)
	}

	// Unsetting every startercode field removes the block entirely.
	view2, err := a.SetAssignment(ctx, "tc", "blatt1", map[string]string{
		"startercode.url":                "",
		"startercode.fromBranch":         "",
		"startercode.additionalBranches": "",
	})
	if err != nil {
		t.Fatalf("set (unset): %v", err)
	}
	if got := ownMap(view2.Own)["startercode.url"]; got != "" {
		t.Errorf("startercode should be unset, own[startercode.url] = %q", got)
	}
	if fs.saved.Source.Assignments["blatt1"].Startercode != nil {
		t.Error("an all-empty startercode block should be removed (nil), not persisted empty")
	}
}

func TestSetAssignment_mergeRequestBlock(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}
	ctx := ctxAs(owner)

	view, err := a.SetAssignment(ctx, "tc", "blatt1", map[string]string{
		"mergeRequest.mergeMethod":  "ff",
		"mergeRequest.squashOption": "always",
		"mergeRequest.pipeline":     "true",
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	own := ownMap(view.Own)
	if own["mergeRequest.mergeMethod"] != "ff" || own["mergeRequest.squashOption"] != "always" {
		t.Errorf("own merge fields = %q / %q", own["mergeRequest.mergeMethod"], own["mergeRequest.squashOption"])
	}
	if own["mergeRequest.pipeline"] != "true" {
		t.Errorf("own[mergeRequest.pipeline] = %q", own["mergeRequest.pipeline"])
	}
	if !bytes.Contains(fs.saved.RawYAML, []byte("mergeMethod: ff")) {
		t.Errorf("rawYAML missing the merge method:\n%s", fs.saved.RawYAML)
	}

	// Unsetting everything removes the block.
	view2, err := a.SetAssignment(ctx, "tc", "blatt1", map[string]string{
		"mergeRequest.mergeMethod":  "",
		"mergeRequest.squashOption": "",
		"mergeRequest.pipeline":     "false",
	})
	if err != nil {
		t.Fatalf("set (unset): %v", err)
	}
	if got := ownMap(view2.Own)["mergeRequest.mergeMethod"]; got != "" {
		t.Errorf("mergeRequest should be unset, got %q", got)
	}
	if fs.saved.Source.Assignments["blatt1"].MergeRequest != nil {
		t.Error("an all-empty mergeRequest block should be removed (nil)")
	}
}

func TestCreateCourse(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	a := &App{db: fs}
	ctx := ctxAs(owner)

	stored, err := a.CreateCourse(ctx, "newcourse", "nc/sem", "ss26", true, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if stored.Source.CoursePath != "nc/sem" || stored.Source.SemesterPath != "ss26" {
		t.Errorf("paths not stored: %+v", stored.Source)
	}
	if len(stored.RawYAML) == 0 {
		t.Error("expected rawYAML to be marshalled")
	}

	// Creating the same name again fails.
	if _, err := a.CreateCourse(ctx, "newcourse", "x", "y", true, false); err == nil {
		t.Error("expected an error creating a course that already exists")
	}
	// Invalid name is rejected.
	if _, err := a.CreateCourse(ctx, "bad name!", "x", "y", true, false); err == nil {
		t.Error("expected an error for an invalid course name")
	}
}

func TestSetAssignment_upsertCreates(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}
	ctx := ctxAs(owner)

	// The assignment "neu" does not exist yet; a valid draft creates it.
	view, err := a.SetAssignment(ctx, "tc", "neu", map[string]string{
		"per":         "student",
		"description": "Brand new",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if ownMap(view.Own)["description"] != "Brand new" {
		t.Errorf("own[description] = %q", ownMap(view.Own)["description"])
	}
	if _, ok := fs.saved.Source.Assignments["neu"]; !ok {
		t.Error("the new assignment was not persisted")
	}

	// An invalid assignment name is rejected on create.
	if _, err := a.SetAssignment(ctx, "tc", "bad name!", map[string]string{"per": "student"}); err == nil {
		t.Error("expected an error for an invalid new-assignment name")
	}
}

func TestDeleteAssignment(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs}
	ctx := ctxAs(owner)

	ok, err := a.DeleteAssignment(ctx, "tc", "blatt1")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !ok {
		t.Error("expected delete to report true")
	}
	if _, exists := fs.saved.Source.Assignments["blatt1"]; exists {
		t.Error("blatt1 should be gone from the stored source")
	}

	// Deleting a non-existent assignment reports false.
	got, err := a.DeleteAssignment(ctx, "tc", "nope")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got {
		t.Error("deleting a missing assignment should report false")
	}
}

func TestSetAssignment_rejectsUnresolvable(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}
	ctx := ctxAs(owner)

	if _, err := a.SetAssignment(ctx, "tc", "blatt1", map[string]string{"extends": "nope"}); err == nil {
		t.Error("expected SetAssignment to reject an unresolvable concrete draft")
	}
	if fs.saved != nil {
		t.Error("an invalid draft must not be persisted")
	}
}
