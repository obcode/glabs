package config

import (
	"reflect"
	"testing"
)

func TestSetAccessLevel(t *testing.T) {
	cfg := &AssignmentConfig{AccessLevel: Developer}

	cfg.SetAccessLevel("guest")
	if cfg.AccessLevel != Guest {
		t.Fatalf("SetAccessLevel(guest) = %v", cfg.AccessLevel)
	}

	cfg.SetAccessLevel("reporter")
	if cfg.AccessLevel != Reporter {
		t.Fatalf("SetAccessLevel(reporter) = %v", cfg.AccessLevel)
	}

	cfg.SetAccessLevel("maintainer")
	if cfg.AccessLevel != Maintainer {
		t.Fatalf("SetAccessLevel(maintainer) = %v", cfg.AccessLevel)
	}

	cfg.SetAccessLevel("unknown")
	if cfg.AccessLevel != Developer {
		t.Fatalf("SetAccessLevel(default) = %v", cfg.AccessLevel)
	}
}

func TestAccessLevel(t *testing.T) {
	registerCourse(t, `
course:
  bare:
    assignmentpath: a
  guest:
    accesslevel: guest
  reporter:
    accesslevel: reporter
  maintainer:
    accesslevel: maintainer
  bogus:
    accesslevel: nonsense
`)

	for _, tt := range []struct {
		assignment string
		want       AccessLevel
	}{
		{"bare", Developer},
		{"guest", Guest},
		{"reporter", Reporter},
		{"maintainer", Maintainer},
		{"bogus", Developer},
	} {
		if got := mustAssignmentConfig(t, "course", tt.assignment).AccessLevel; got != tt.want {
			t.Errorf("%s: AccessLevel = %v, want %v", tt.assignment, got, tt.want)
		}
	}
}

func TestMkStudentsClassifiesIdentifiers(t *testing.T) {
	students := mkStudents([]string{"1001", "alice", "alice@example.org", "0123"})

	if students[0].Id == nil || *students[0].Id != 1001 {
		t.Fatalf("expected first student to be user id, got %#v", students[0])
	}
	if students[1].Username == nil || *students[1].Username != "alice" {
		t.Fatalf("expected second student username alice, got %#v", students[1])
	}
	if students[2].Email == nil || *students[2].Email != "alice@example.org" {
		t.Fatalf("expected third student email, got %#v", students[2])
	}
	if students[3].Username == nil || *students[3].Username != "0123" {
		t.Fatalf("leading zero ids must be treated as username, got %#v", students[3])
	}
}

// Assignment-level students are appended to the course-level list, then sorted.
func TestStudentsMergeFilterAndSort(t *testing.T) {
	registerCourse(t, `
course:
  students:
    - carol@example.org
    - alice@example.org
  a1:
    students:
      - bob@example.org
`)
	studs := mustAssignmentConfig(t, "course", "a1").Students

	var got []string
	for _, s := range studs {
		got = append(got, s.Raw)
	}
	want := []string{"alice@example.org", "bob@example.org", "carol@example.org"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("students = %v, want %v", got, want)
	}
}

// The positional arguments are regexps.
func TestStudentsFilteredByPattern(t *testing.T) {
	registerCourse(t, `
course:
  students:
    - alice@example.org
    - bob@example.org
    - carol@example.org
  a1:
    assignmentpath: a
`)
	studs := mustAssignmentConfig(t, "course", "a1", "^[ab]").Students

	var got []string
	for _, s := range studs {
		got = append(got, s.Raw)
	}
	want := []string{"alice@example.org", "bob@example.org"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("students = %v, want %v", got, want)
	}
}

func TestStudentsReturnsNilForGroupMode(t *testing.T) {
	registerCourse(t, `
course:
  students:
    - alice@example.org
  a1:
    per: group
`)
	if s := mustAssignmentConfig(t, "course", "a1").Students; s != nil {
		t.Fatalf("students = %v, want nil for per: group", s)
	}
}

// Assignment-level groups override course-level ones per key; names and members
// are sorted, and the keys are lowercased the way the loader has always done.
func TestGroupsMergeFilterAndSort(t *testing.T) {
	registerCourse(t, `
course:
  groups:
    grp02:
      - carol@example.org
      - alice@example.org
    grp01:
      - bob@example.org
  a1:
    per: group
    groups:
      grp01:
        - dave@example.org
`)
	groups := mustAssignmentConfig(t, "course", "a1").Groups

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
	if groups[0].Name != "grp01" || groups[1].Name != "grp02" {
		t.Fatalf("groups = %q, %q, want them sorted", groups[0].Name, groups[1].Name)
	}
	if len(groups[0].Members) != 1 || groups[0].Members[0].Raw != "dave@example.org" {
		t.Fatalf("grp01 = %v, want the assignment-level override to win", groups[0].Members)
	}
	if groups[1].Members[0].Raw != "alice@example.org" {
		t.Fatalf("grp02 members = %v, want them sorted", groups[1].Members)
	}
}

func TestGroupsFilteredByPattern(t *testing.T) {
	registerCourse(t, `
course:
  groups:
    grp01:
      - a@example.org
    grp02:
      - b@example.org
    other:
      - c@example.org
  a1:
    per: group
`)
	groups := mustAssignmentConfig(t, "course", "a1", "^grp").Groups

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2 matching ^grp", len(groups))
	}
}

func TestGroupsReturnsNilForStudentMode(t *testing.T) {
	registerCourse(t, `
course:
  groups:
    grp01:
      - a@example.org
  a1:
    per: student
`)
	if g := mustAssignmentConfig(t, "course", "a1").Groups; g != nil {
		t.Fatalf("groups = %v, want nil for per: student", g)
	}
}
