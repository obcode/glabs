package config

import (
	"reflect"
	"testing"
)

func TestNoSeeder(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    assignmentpath: blatt-01
`)
	if s := mustAssignmentConfig(t, "course", "a1").Seeder; s != nil {
		t.Fatalf("seeder = %#v, want nil when unconfigured", s)
	}
}

func TestSeederDefaults(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    seeder:
      cmd: /usr/bin/true
`)
	s := mustAssignmentConfig(t, "course", "a1").Seeder

	if s.Command != "/usr/bin/true" {
		t.Errorf("Command = %q, want %q", s.Command, "/usr/bin/true")
	}
	if s.ToBranch != "main" {
		t.Errorf("ToBranch = %q, want the default %q", s.ToBranch, "main")
	}
	if s.SignKey != nil {
		t.Error("SignKey is set, want nil when unconfigured")
	}
	if s.ProtectToBranch {
		t.Error("ProtectToBranch = true, want false")
	}
}

func TestSeederOverrides(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    seeder:
      cmd: /usr/bin/seed
      args: ["--path", "%s"]
      name: Seeder Bot
      email: bot@example.org
      toBranch: seeded
      protectToBranch: true
`)
	s := mustAssignmentConfig(t, "course", "a1").Seeder

	want := &Seeder{
		Command:         "/usr/bin/seed",
		Args:            []string{"--path", "%s"},
		Name:            "Seeder Bot",
		EMail:           "bot@example.org",
		ToBranch:        "seeded",
		ProtectToBranch: true,
	}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("seeder = %#v, want %#v", s, want)
	}
}

func TestSeederWithoutCmdIsAnError(t *testing.T) {
	registerCourse(t, `
course:
  a1:
    seeder:
      name: Seeder Bot
`)
	if _, err := GetAssignmentConfig("course", "a1"); err == nil {
		t.Fatal("a seeder without cmd was accepted, want an error")
	}
}
