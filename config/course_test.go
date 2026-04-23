package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestGetCourseConfig_HappyPath_Students(t *testing.T) {
	resetViper(t)
	viper.Set("mpd.students", []string{"alice", "bob"})

	cc := GetCourseConfig("mpd")
	if cc == nil {
		t.Fatal("GetCourseConfig() returned nil")
	}
	if cc.Course != "mpd" {
		t.Fatalf("Course = %q, want %q", cc.Course, "mpd")
	}
	if len(cc.Students) != 2 {
		t.Fatalf("Students = %d, want 2", len(cc.Students))
	}
}

func TestGetCourseConfig_HappyPath_Groups(t *testing.T) {
	resetViper(t)
	viper.Set("mpd.groups.team1.members", []string{"alice"})
	viper.Set("mpd.groups.team2.members", []string{"bob", "carol"})

	cc := GetCourseConfig("mpd")
	if cc == nil {
		t.Fatal("GetCourseConfig() returned nil")
	}
	if len(cc.Groups) != 2 {
		t.Fatalf("Groups = %d, want 2", len(cc.Groups))
	}
}

func TestGetCourseConfig_NoStudentsNoGroups(t *testing.T) {
	resetViper(t)
	// Just set the top-level key so IsSet("mpd") returns true
	viper.Set("mpd.semesterpath", "ss26")

	cc := GetCourseConfig("mpd")
	if cc == nil {
		t.Fatal("GetCourseConfig() returned nil")
	}
	if cc.Course != "mpd" {
		t.Fatalf("Course = %q, want %q", cc.Course, "mpd")
	}
}
