package config

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Golden tests pin the *current* resolved output of GetAssignmentConfig for a
// set of course files, so that the upcoming rewrite of config loading (viper ->
// typed source schema) can be proven behaviour-preserving.
//
// They deliberately freeze behaviour as it is today, bugs included. If a golden
// looks wrong, that is a finding to fix in its own commit — not something to
// quietly "correct" while regenerating, or the goldens stop being a net.
//
// Refresh with:  go test ./config/ -run TestGolden -update

var updateGolden = flag.Bool("update", false, "update golden files")

// goldenHost is fixed so the recorded URLs do not depend on any local config.
const goldenHost = "https://gitlab.lrz.de"

// reservedCourseKeys are the course-level keys that are not assignments.
// viper lowercases all keys, so these are compared lowercase.
var reservedCourseKeys = map[string]bool{
	"coursepath":             true,
	"semesterpath":           true,
	"usecoursenameasprefix":  true,
	"useemaildomainassuffix": true,
	"students":               true,
	"groups":                 true,
}

// goldenStudent records the resolved identity together with the names derived
// from it. The derived names are the point: a change in case handling would not
// show up in Email/Raw, but it silently renames the GitLab project.
type goldenStudent struct {
	Raw        string  `json:"raw"`
	Email      *string `json:"email"`
	Username   *string `json:"username"`
	ID         *int    `json:"id"`
	RepoSuffix string  `json:"repoSuffix"`
	RepoName   string  `json:"repoName"`
}

type goldenGroup struct {
	Name     string          `json:"name"`
	RepoName string          `json:"repoName"`
	Members  []goldenStudent `json:"members"`
}

// goldenSeeder replaces Seeder.SignKey (*openpgp.Entity), which has no stable
// serialization, with a boolean.
type goldenSeeder struct {
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	Name            string   `json:"name"`
	EMail           string   `json:"email"`
	HasSignKey      bool     `json:"hasSignKey"`
	ToBranch        string   `json:"toBranch"`
	ProtectToBranch bool     `json:"protectToBranch"`
}

type goldenAssignment struct {
	Course                 string                     `json:"course"`
	Name                   string                     `json:"name"`
	UseCoursenameAsPrefix  bool                       `json:"useCoursenameAsPrefix"`
	UseEmailDomainAsSuffix bool                       `json:"useEmailDomainAsSuffix"`
	Path                   string                     `json:"path"`
	URL                    string                     `json:"url"`
	RepoBaseName           string                     `json:"repoBaseName"`
	Per                    string                     `json:"per"`
	Description            string                     `json:"description"`
	ContainerRegistry      bool                       `json:"containerRegistry"`
	AccessLevel            string                     `json:"accessLevel"`
	AccessLevelValue       int                        `json:"accessLevelValue"`
	MergeRequest           *MergeRequest              `json:"mergeRequest"`
	Branches               []BranchRule               `json:"branches"`
	Issues                 *IssueReplication          `json:"issues"`
	Startercode            *Startercode               `json:"startercode"`
	Clone                  *Clone                     `json:"clone"`
	Release                *Release                   `json:"release"`
	Seeder                 *goldenSeeder              `json:"seeder"`
	DeferredBranches       map[string]*DeferredBranch `json:"deferredBranches"`
	Students               []goldenStudent            `json:"students"`
	Groups                 []goldenGroup              `json:"groups"`
}

func goldenView(cfg *AssignmentConfig) *goldenAssignment {
	view := &goldenAssignment{
		Course:                 cfg.Course,
		Name:                   cfg.Name,
		UseCoursenameAsPrefix:  cfg.UseCoursenameAsPrefix,
		UseEmailDomainAsSuffix: cfg.UseEmailDomainAsSuffix,
		Path:                   cfg.Path,
		URL:                    cfg.URL,
		RepoBaseName:           cfg.RepoBaseName(),
		Per:                    string(cfg.Per),
		Description:            cfg.Description,
		ContainerRegistry:      cfg.ContainerRegistry,
		AccessLevel:            cfg.AccessLevel.String(),
		AccessLevelValue:       int(cfg.AccessLevel),
		MergeRequest:           cfg.MergeRequest,
		Branches:               cfg.Branches,
		Issues:                 cfg.Issues,
		Startercode:            cfg.Startercode,
		Clone:                  cfg.Clone,
		Release:                cfg.Release,
		DeferredBranches:       cfg.DeferredBranches,
	}

	if cfg.Seeder != nil {
		view.Seeder = &goldenSeeder{
			Command:         cfg.Seeder.Command,
			Args:            cfg.Seeder.Args,
			Name:            cfg.Seeder.Name,
			EMail:           cfg.Seeder.EMail,
			HasSignKey:      cfg.Seeder.SignKey != nil,
			ToBranch:        cfg.Seeder.ToBranch,
			ProtectToBranch: cfg.Seeder.ProtectToBranch,
		}
	}

	for _, student := range cfg.Students {
		view.Students = append(view.Students, goldenStudentView(cfg, student))
	}

	for _, group := range cfg.Groups {
		g := goldenGroup{Name: group.Name, RepoName: cfg.RepoNameForGroup(group)}
		for _, member := range group.Members {
			g.Members = append(g.Members, goldenStudentView(cfg, member))
		}
		view.Groups = append(view.Groups, g)
	}

	return view
}

func goldenStudentView(cfg *AssignmentConfig, student *Student) goldenStudent {
	return goldenStudent{
		Raw:        student.Raw,
		Email:      student.Email,
		Username:   student.Username,
		ID:         student.Id,
		RepoSuffix: cfg.RepoSuffix(student),
		RepoName:   cfg.RepoNameForStudent(student),
	}
}

// loadCourseFixture resets viper and loads a single course file, mimicking what
// cmd/root.go initConfig() does for a course file. viper is global and
// GetAssignmentConfig writes back to it while resolving `extends`
// (config/inheritance.go:54), so every assignment gets a fresh load — otherwise
// results would depend on resolution order.
func loadCourseFixture(t *testing.T, path string) string {
	t.Helper()

	resetViper(t)
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	viper.Set("gitlab.host", goldenHost)

	course := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if !viper.IsSet(course) {
		t.Fatalf("fixture %s has no top-level key %q — filename and course name must match", path, course)
	}
	return course
}

// concreteAssignments returns the assignments that can actually be operated on:
// every course-level key that holds a map, minus the reserved course settings
// and minus abstract bases.
func concreteAssignments(t *testing.T, course string) []string {
	t.Helper()

	var names []string
	for key, value := range viper.GetStringMap(course) {
		if reservedCourseKeys[key] {
			continue
		}
		if _, isMap := asStringMap(value); !isMap {
			continue
		}
		if viper.GetBool(course + "." + key + "." + abstractKey) {
			continue
		}
		names = append(names, key)
	}
	sort.Strings(names)

	if len(names) == 0 {
		t.Fatalf("course %q has no concrete assignments — fixture or discovery is broken", course)
	}
	return names
}

func courseFixtures(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "courses", "*.yaml"))
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no course fixtures found — glob broken?")
	}
	sort.Strings(paths)
	return paths
}

func TestGolden(t *testing.T) {
	for _, path := range courseFixtures(t) {
		course := loadCourseFixture(t, path)

		for _, assignment := range concreteAssignments(t, course) {
			t.Run(course+"/"+assignment, func(t *testing.T) {
				// Reload per assignment: GetAssignmentConfig mutates viper.
				loadCourseFixture(t, path)

				cfg := mustAssignmentConfig(t, course, assignment)

				got, err := json.MarshalIndent(goldenView(cfg), "", "  ")
				if err != nil {
					t.Fatalf("marshalling golden view: %v", err)
				}
				got = append(got, '\n')

				assertGolden(t, filepath.Join("testdata", "golden", course+"."+assignment+".json"), got)
			})
		}
	}
}

func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (run: go test ./config/ -run TestGolden -update)", path, err)
	}

	if string(got) != string(want) {
		t.Errorf("resolved config differs from golden %s\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}
