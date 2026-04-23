package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSeeder_NoSeeder_ReturnsNil(t *testing.T) {
	resetViper(t)
	if got := seeder("course.a1"); got != nil {
		t.Fatalf("seeder() = %#v, want nil", got)
	}
}

func TestSeeder_HappyPath_Defaults(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.seeder.cmd", "make")
	viper.Set("course.a1.seeder.args", []string{"build", "test"})
	viper.Set("course.a1.seeder.name", "Seed Bot")
	viper.Set("course.a1.seeder.email", "bot@example.com")

	got := seeder("course.a1")
	if got == nil {
		t.Fatal("seeder() returned nil, want non-nil")
	}
	if got.Command != "make" {
		t.Fatalf("Command = %q, want %q", got.Command, "make")
	}
	if got.ToBranch != "main" {
		t.Fatalf("ToBranch = %q, want default %q", got.ToBranch, "main")
	}
	if got.Name != "Seed Bot" {
		t.Fatalf("Name = %q, want %q", got.Name, "Seed Bot")
	}
	if got.EMail != "bot@example.com" {
		t.Fatalf("EMail = %q, want %q", got.EMail, "bot@example.com")
	}
	if got.SignKey != nil {
		t.Fatal("SignKey should be nil when not set")
	}
}

func TestSeeder_ToBranchOverride(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.seeder.cmd", "make")
	viper.Set("course.a1.seeder.toBranch", "develop")

	got := seeder("course.a1")
	if got == nil {
		t.Fatal("seeder() returned nil")
	}
	if got.ToBranch != "develop" {
		t.Fatalf("ToBranch = %q, want %q", got.ToBranch, "develop")
	}
}

func TestSeeder_WithArgs(t *testing.T) {
	resetViper(t)
	viper.Set("course.a1.seeder.cmd", "python")
	viper.Set("course.a1.seeder.args", []string{"seed.py", "--verbose"})

	got := seeder("course.a1")
	if got == nil {
		t.Fatal("seeder() returned nil")
	}
	if len(got.Args) != 2 {
		t.Fatalf("Args = %v, want 2 elements", got.Args)
	}
	if got.Args[0] != "seed.py" {
		t.Fatalf("Args[0] = %q, want seed.py", got.Args[0])
	}
}
