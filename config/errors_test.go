package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// These tests were impossible before config loading returned errors: every case
// below used to call log.Fatal (= os.Exit(1)) and would have taken the test
// binary down with it.

func loadErrorFixture(t *testing.T) {
	t.Helper()

	ResetCourses()
	t.Cleanup(ResetCourses)
	resetViper(t)
	viper.Set("gitlab.host", goldenHost)

	if _, err := LoadCourseFile("testdata/errors/edge.yaml"); err != nil {
		t.Fatalf("loading error fixture: %v", err)
	}
}

func TestGetAssignmentConfigErrors(t *testing.T) {
	tests := []struct {
		name       string
		course     string
		assignment string
		wantErr    string
	}{
		{
			name:       "unknown course",
			course:     "nosuchcourse",
			assignment: "fine",
			wantErr:    "configuration for course nosuchcourse not found",
		},
		{
			name:       "unknown assignment",
			course:     "edge",
			assignment: "nosuchassignment",
			wantErr:    "configuration for assignment nosuchassignment not found",
		},
		{
			name:       "abstract assignment used directly",
			course:     "edge",
			assignment: "base",
			wantErr:    "is abstract",
		},
		{
			name:       "cyclic extends across two assignments",
			course:     "edge",
			assignment: "cyclea",
			wantErr:    "cyclic 'extends' inheritance detected",
		},
		{
			name:       "assignment extending itself",
			course:     "edge",
			assignment: "selfcycle",
			wantErr:    "cyclic 'extends' inheritance detected",
		},
		{
			name:       "extends is not a string",
			course:     "edge",
			assignment: "extendsnotstring",
			wantErr:    "'extends' must be the name of another assignment",
		},
		{
			name:       "extends is blank",
			course:     "edge",
			assignment: "extendsempty",
			wantErr:    "'extends' must be the name of another assignment",
		},
		{
			name:       "extends names a missing parent",
			course:     "edge",
			assignment: "extendsmissing",
			wantErr:    `assignment "doesnotexist" referenced by 'extends' not found`,
		},
		{
			name:       "startercode without url",
			course:     "edge",
			assignment: "starterwithouturl",
			wantErr:    "startercode provided without url",
		},
		{
			name:       "seeder without cmd",
			course:     "edge",
			assignment: "seederwithoutcmd",
			wantErr:    "seeder provided without cmd",
		},
		{
			name:       "seeder signKey is not an armored key ring",
			course:     "edge",
			assignment: "seederbadsignkey",
			wantErr:    "cannot read seeder.signKey as an armored PGP key ring",
		},
		{
			name:       "legacy approvals users key",
			course:     "edge",
			assignment: "approvalsusers",
			wantErr:    "no longer supported; use usernames",
		},
		{
			name:       "invalid whenCommitAdded",
			course:     "edge",
			assignment: "badwhencommitadded",
			wantErr:    "invalid mergeRequest.approvals.settings.whenCommitAdded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadErrorFixture(t)

			cfg, err := GetAssignmentConfig(tt.course, tt.assignment)
			if err == nil {
				t.Fatalf("GetAssignmentConfig(%q, %q) = %+v, want error containing %q",
					tt.course, tt.assignment, cfg, tt.wantErr)
			}
			if cfg != nil {
				t.Errorf("GetAssignmentConfig(%q, %q) returned a config alongside the error: %+v",
					tt.course, tt.assignment, cfg)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("GetAssignmentConfig(%q, %q) error = %q, want it to contain %q",
					tt.course, tt.assignment, err, tt.wantErr)
			}
		})
	}
}

// The error fixture must itself be loadable, so a failure above is about the
// assignment under test and not a broken file.
func TestErrorFixtureResolvesValidAssignment(t *testing.T) {
	loadErrorFixture(t)

	cfg, err := GetAssignmentConfig("edge", "fine")
	if err != nil {
		t.Fatalf("GetAssignmentConfig(edge, fine): unexpected error: %v", err)
	}
	if cfg.Per != PerStudent {
		t.Errorf("Per = %q, want %q (inherited from the abstract base)", cfg.Per, PerStudent)
	}
}

func TestGetCourseConfigUnknownCourse(t *testing.T) {
	loadErrorFixture(t)

	cfg, err := GetCourseConfig("nosuchcourse")
	if err == nil {
		t.Fatalf("GetCourseConfig(nosuchcourse) = %+v, want error", cfg)
	}
	if cfg != nil {
		t.Errorf("GetCourseConfig(nosuchcourse) returned a config alongside the error: %+v", cfg)
	}
	if want := "configuration for course nosuchcourse not found"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
}
