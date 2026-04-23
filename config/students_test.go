package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
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
	resetViper(t)

	if got := accessLevel("course.a1"); got != Developer {
		t.Fatalf("default accessLevel = %v", got)
	}

	viper.Set("course.a1.accesslevel", "guest")
	if got := accessLevel("course.a1"); got != Guest {
		t.Fatalf("guest accessLevel = %v", got)
	}

	viper.Set("course.a1.accesslevel", "reporter")
	if got := accessLevel("course.a1"); got != Reporter {
		t.Fatalf("reporter accessLevel = %v", got)
	}

	viper.Set("course.a1.accesslevel", "maintainer")
	if got := accessLevel("course.a1"); got != Maintainer {
		t.Fatalf("maintainer accessLevel = %v", got)
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

func TestStudentsMergeFilterAndSort(t *testing.T) {
	resetViper(t)

	viper.Set("course.students", []string{"carol", "1002"})
	viper.Set("course.a1.students", []string{"alice", "bob"})

	studs := students(PerStudent, "course", "a1", "^a", "^100")
	if len(studs) != 2 {
		t.Fatalf("students len = %d, want 2", len(studs))
	}

	got := []string{studs[0].Raw, studs[1].Raw}
	want := []string{"1002", "alice"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("students order/filter = %#v, want %#v", got, want)
	}
}

func TestStudentsReturnsNilForGroupMode(t *testing.T) {
	resetViper(t)
	viper.Set("course.students", []string{"alice"})

	if studs := students(PerGroup, "course", "a1"); studs != nil {
		t.Fatalf("students for group mode = %#v, want nil", studs)
	}
}

func TestGroupsMergeFilterAndSort(t *testing.T) {
	resetViper(t)

	viper.Set("course.groups", map[string][]string{
		"g2": {"bob", "alice"},
		"g1": {"1001"},
	})
	viper.Set("course.a1.groups", map[string][]string{
		"g2": {"carol"},
		"g3": {"dave"},
	})

	grps := groups(PerGroup, "course", "a1", "^g[23]$")
	if len(grps) != 2 {
		t.Fatalf("groups len = %d, want 2", len(grps))
	}

	if grps[0].Name != "g2" || len(grps[0].Members) != 1 || grps[0].Members[0].Raw != "carol" {
		t.Fatalf("g2 members = %#v", grps[0])
	}
	if grps[1].Name != "g3" || len(grps[1].Members) != 1 || grps[1].Members[0].Raw != "dave" {
		t.Fatalf("g3 members = %#v", grps[1])
	}
}

func TestGroupsReturnsNilForStudentMode(t *testing.T) {
	resetViper(t)
	viper.Set("course.groups", map[string][]string{"g1": {"alice"}})

	if grps := groups(PerStudent, "course", "a1"); grps != nil {
		t.Fatalf("groups for student mode = %#v, want nil", grps)
	}
}
