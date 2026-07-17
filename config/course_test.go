package config

import "testing"

func TestGetCourseConfigStudents(t *testing.T) {
	registerCourse(t, `
mpd:
  students:
    - alice
    - bob
`)
	cc := mustCourseConfig(t, "mpd")

	if cc.Course != "mpd" {
		t.Errorf("Course = %q, want %q", cc.Course, "mpd")
	}
	if len(cc.Students) != 2 {
		t.Fatalf("Students = %d, want 2", len(cc.Students))
	}
	if cc.Students[0].Raw != "alice" || cc.Students[1].Raw != "bob" {
		t.Errorf("Students = %q, %q", cc.Students[0].Raw, cc.Students[1].Raw)
	}
}

func TestGetCourseConfigGroups(t *testing.T) {
	registerCourse(t, `
mpd:
  groups:
    team1:
      - alice
    team2:
      - bob
      - carol
`)
	cc := mustCourseConfig(t, "mpd")

	if len(cc.Groups) != 2 {
		t.Fatalf("Groups = %d, want 2", len(cc.Groups))
	}
	if cc.Groups[0].Name != "team1" || cc.Groups[1].Name != "team2" {
		t.Errorf("group names = %q, %q, want them sorted", cc.Groups[0].Name, cc.Groups[1].Name)
	}
	if len(cc.Groups[1].Members) != 2 {
		t.Errorf("team2 members = %d, want 2", len(cc.Groups[1].Members))
	}
}

// A course with neither students nor groups resolves rather than erroring: it is
// a valid, if empty, course.
func TestGetCourseConfigWithoutStudentsOrGroups(t *testing.T) {
	registerCourse(t, `
mpd:
  semesterpath: ss26
`)
	cc := mustCourseConfig(t, "mpd")

	if cc.Course != "mpd" {
		t.Errorf("Course = %q, want %q", cc.Course, "mpd")
	}
	if len(cc.Students) != 0 || len(cc.Groups) != 0 {
		t.Errorf("Students = %v, Groups = %v, want both empty", cc.Students, cc.Groups)
	}
}

func TestGetCourseConfigUnknownCourseErrors(t *testing.T) {
	registerCourse(t, `
mpd:
  semesterpath: ss26
`)
	if _, err := GetCourseConfig("nosuchcourse"); err == nil {
		t.Fatal("GetCourseConfig(nosuchcourse) succeeded, want an error")
	}
}

func TestCourseExists(t *testing.T) {
	registerCourse(t, `
mpd:
  semesterpath: ss26
`)
	if !CourseExists("mpd") {
		t.Error("CourseExists(mpd) = false, want true")
	}
	if !CourseExists("MPD") {
		t.Error("CourseExists(MPD) = false, want true: course lookup folds case")
	}
	if CourseExists("nosuchcourse") {
		t.Error("CourseExists(nosuchcourse) = true, want false")
	}
}

func TestGetCourseSubgroupPath(t *testing.T) {
	registerCourse(t, `
mpd:
  coursepath: MPD/Semester
  semesterpath: SS2026
`)
	if got, want := GetCourseSubgroupPath("mpd"), "mpd/semester/ss2026"; got != want {
		t.Errorf("GetCourseSubgroupPath = %q, want %q: group paths are lowercased the way GitLab stores them", got, want)
	}
}
