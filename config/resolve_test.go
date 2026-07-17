package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gopkg.in/yaml.v3"
)

// The pure resolver has exactly one job: produce what the viper-based loader
// produces, for every assignment in every fixture. Running both against the same
// input and diffing is a stronger check than the goldens — the goldens pin a
// recorded snapshot, this pins the two implementations to each other, including
// on inputs nobody thought to record.
//
// Once the loader is gone this test goes with it; until then it is what makes
// the swap provable rather than hopeful.

// rawCourseBody reads a fixture and returns the course name and the raw body
// under its single top-level key — the input side of the pure resolver.
func rawCourseBody(t *testing.T, path string) (string, any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var top map[string]any
	if err := yaml.Unmarshal(data, &top); err != nil {
		t.Fatalf("parsing fixture: %v", err)
	}
	if len(top) != 1 {
		t.Fatalf("fixture %s has %d top-level keys, want 1", path, len(top))
	}
	for name, body := range top {
		return name, body
	}
	return "", nil
}

func TestResolveMatchesViperLoader(t *testing.T) {
	for _, path := range courseFixtures(t) {
		course := loadCourseFixture(t, path)

		for _, assignment := range concreteAssignments(t, course) {
			t.Run(course+"/"+assignment, func(t *testing.T) {
				// viper mutates itself while resolving `extends`, so reload.
				loadCourseFixture(t, path)
				want := mustAssignmentConfig(t, course, assignment)

				name, body := rawCourseBody(t, path)
				got, err := ResolveAssignment(name, body, Globals{GitlabHost: goldenHost}, assignment)
				if err != nil {
					t.Fatalf("ResolveAssignment(%q, %q): %v", name, assignment, err)
				}

				if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(openpgpEntityPlaceholder{})); diff != "" {
					t.Errorf("pure resolver disagrees with the viper loader (-viper +pure):\n%s", diff)
				}
			})
		}
	}
}

// openpgpEntityPlaceholder exists only to give cmpopts something to name. No
// fixture configures a sign key, so Seeder.SignKey is nil throughout and cmp
// never has to walk an openpgp.Entity.
type openpgpEntityPlaceholder struct{ _ int }

// The filter arguments are regexps, and they select students and groups the same
// way in both loaders.
func TestResolveMatchesViperLoaderWithFilters(t *testing.T) {
	tests := []struct {
		course     string
		assignment string
		onlyFor    []string
	}{
		{"vss", "blatt1", []string{"grp0[1-3]"}},
		{"vss", "blatt1", []string{"grp01", "grp02"}},
		{"vss", "blatt0", []string{"s0"}},
		{"mpd", "blatt10", []string{"s161"}},
		{"casecourse", "upperkeys", []string{"grp"}},
		{"vss", "blatt1", []string{"nomatch"}},
	}

	for _, tt := range tests {
		t.Run(tt.course+"/"+tt.assignment, func(t *testing.T) {
			path := filepath.Join("testdata", "courses", tt.course+".yaml")

			loadCourseFixture(t, path)
			want := mustAssignmentConfig(t, tt.course, tt.assignment, tt.onlyFor...)

			name, body := rawCourseBody(t, path)
			got, err := ResolveAssignment(name, body, Globals{GitlabHost: goldenHost}, tt.assignment, tt.onlyFor...)
			if err != nil {
				t.Fatalf("ResolveAssignment: %v", err)
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("pure resolver disagrees with the viper loader for onlyFor=%v (-viper +pure):\n%s",
					tt.onlyFor, diff)
			}
		})
	}
}

// The error paths must agree too, or a config that is rejected today would
// quietly resolve tomorrow.
func TestResolveMatchesViperLoaderOnErrors(t *testing.T) {
	tests := []struct {
		name       string
		course     string
		assignment string
	}{
		{"unknown assignment", "edge", "nosuchassignment"},
		{"abstract assignment", "edge", "base"},
		{"cyclic extends", "edge", "cyclea"},
		{"self cycle", "edge", "selfcycle"},
		{"extends not a string", "edge", "extendsnotstring"},
		{"extends blank", "edge", "extendsempty"},
		{"extends missing parent", "edge", "extendsmissing"},
		{"seeder without cmd", "edge", "seederwithoutcmd"},
		{"seeder bad signkey", "edge", "seederbadsignkey"},
		{"invalid whenCommitAdded", "edge", "badwhencommitadded"},
	}

	const path = "testdata/errors/edge.yaml"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadErrorFixture(t)
			_, viperErr := GetAssignmentConfig(tt.course, tt.assignment)

			name, body := rawCourseBody(t, path)
			_, pureErr := ResolveAssignment(name, body, Globals{GitlabHost: goldenHost}, tt.assignment)

			if viperErr == nil {
				t.Fatalf("viper loader unexpectedly succeeded; fixture or test is wrong")
			}
			if pureErr == nil {
				t.Fatalf("pure resolver accepted %q, which the viper loader rejects with: %v", tt.assignment, viperErr)
			}
		})
	}
}

// The one place the pure resolver is allowed to differ: it never mutates global
// state, so resolving in any order gives the same answer. The viper loader
// writes the merged `extends` result back into the global registry, which is
// exactly what this replaces.
func TestResolveIsIndependentOfOrder(t *testing.T) {
	t.Parallel()

	name, body := rawCourseBody(t, filepath.Join("testdata", "courses", "mpd.yaml"))
	g := Globals{GitlabHost: goldenHost}

	// blatt12 sits at the end of a five-deep extends chain.
	first, err := ResolveAssignment(name, body, g, "blatt12")
	if err != nil {
		t.Fatalf("resolving blatt12 first: %v", err)
	}

	// Resolving its ancestors in between must not change the answer.
	for _, ancestor := range []string{"blatt08", "blatt09", "blatt10", "blatt11"} {
		if _, err := ResolveAssignment(name, body, g, ancestor); err != nil {
			t.Fatalf("resolving %s: %v", ancestor, err)
		}
	}

	second, err := ResolveAssignment(name, body, g, "blatt12")
	if err != nil {
		t.Fatalf("resolving blatt12 again: %v", err)
	}

	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("resolution depends on order (-first +second):\n%s", diff)
	}
}
