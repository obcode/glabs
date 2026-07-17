package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func contains(list []string, want string) bool { return slices.Contains(list, want) }

func hasSuffix(list []string, suffix string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.HasSuffix(s, suffix) })
}

// decodeFixture decodes a course fixture by name from testdata/courses/.
func decodeFixture(t *testing.T, name string) (*CourseSource, *DecodeResult) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "courses", name+".yaml"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	course, result, err := DecodeCourse(data)
	if err != nil {
		t.Fatalf("DecodeCourse(%s): %v", name, err)
	}
	return course, result
}

// Every real course file must decode, and the decode must be lossless in the
// sense that re-encoding and decoding again yields the same source. Byte
// equality is explicitly NOT the goal — comments and key order do not survive a
// struct — but the *meaning* must.
func TestRoundTripAllFixtures(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("testdata", "courses", "*.yaml"))
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(paths) < 5 {
		t.Fatalf("found only %d course fixtures — glob broken?", len(paths))
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			first, _, err := DecodeCourse(data)
			if err != nil {
				t.Fatalf("first decode: %v", err)
			}

			encoded, err := EncodeCourse(first)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}

			second, _, err := DecodeCourse(encoded)
			if err != nil {
				t.Fatalf("second decode of re-encoded source: %v\n--- encoded ---\n%s", err, encoded)
			}

			if diff := cmp.Diff(first, second); diff != "" {
				t.Errorf("round trip changed the source (-first +second):\n%s", diff)
			}
		})
	}
}

// The assignments must land in the ",remain" map rather than being reported as
// unknown, and the course settings must not leak into it.
func TestDecodeSeparatesCourseSettingsFromAssignments(t *testing.T) {
	t.Parallel()

	course, _ := decodeFixture(t, "vss")

	if course.Name != "vss" {
		t.Errorf("Name = %q, want %q", course.Name, "vss")
	}
	if course.CoursePath != "vss/semester" {
		t.Errorf("CoursePath = %q, want %q", course.CoursePath, "vss/semester")
	}
	if !course.UseCoursenameAsPrefix {
		t.Error("UseCoursenameAsPrefix = false, want true")
	}
	if course.UseEmailDomainAsSuffix == nil || *course.UseEmailDomainAsSuffix {
		t.Errorf("UseEmailDomainAsSuffix = %v, want explicit false", course.UseEmailDomainAsSuffix)
	}
	// Counted from the fixture: 63 student entries, 42 group keys (grp00–grp43
	// with gaps). Asserted so a decoder that silently drops entries is caught.
	if len(course.Students) != 63 {
		t.Errorf("len(Students) = %d, want 63", len(course.Students))
	}
	if len(course.Groups) != 42 {
		t.Errorf("len(Groups) = %d, want 42", len(course.Groups))
	}

	wantAssignments := []string{"blatt0", "blatt1", "blatt2"}
	if len(course.Assignments) != len(wantAssignments) {
		t.Fatalf("assignments = %v, want exactly %v", keysOf(course.Assignments), wantAssignments)
	}
	for _, name := range wantAssignments {
		if course.Assignments[name] == nil {
			t.Errorf("assignment %q missing; got %v", name, keysOf(course.Assignments))
		}
	}
}

func keysOf(m map[string]*AssignmentSource) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// mapstructure must fold case exactly like viper does, or configs that work
// today would silently lose fields.
func TestDecodeFoldsFieldNameCase(t *testing.T) {
	t.Parallel()

	course, _ := decodeFixture(t, "casecourse")

	a := course.Assignments["mixedfields"]
	if a == nil {
		t.Fatal("assignment mixedfields missing")
	}
	if a.Startercode.FromBranch != "template" {
		t.Errorf("`frombranch` did not fold onto FromBranch: got %q", a.Startercode.FromBranch)
	}
	if a.Startercode.ToBranch != "main" {
		t.Errorf("`tobranch` did not fold onto ToBranch: got %q", a.Startercode.ToBranch)
	}
	if a.Startercode.TemplateMessage != "Init" {
		t.Errorf("`templatemessage` did not fold onto TemplateMessage: got %q", a.Startercode.TemplateMessage)
	}
	if len(a.Branches) != 2 {
		t.Fatalf("len(Branches) = %d, want 2", len(a.Branches))
	}
	if !a.Branches[0].MergeOnly {
		t.Error("`mergeonly` did not fold onto MergeOnly")
	}
	if !a.Branches[1].AllowForcePush {
		t.Error("`allow_force_push` alias did not reach AllowForcePush")
	}
}

// Map keys are data, not field names: unlike viper, the typed decoder must keep
// them verbatim. Resolve is responsible for lowercasing them where the old
// loader did (groups, deferredBranches) — the source keeps what was written.
func TestDecodeKeepsMapKeysVerbatim(t *testing.T) {
	t.Parallel()

	course, _ := decodeFixture(t, "casecourse")

	if _, ok := course.Groups["Grp01"]; !ok {
		t.Errorf("group key was not kept verbatim; got %v", keysOfStrings(course.Groups))
	}

	a := course.Assignments["upperkeys"]
	if a == nil {
		t.Fatal("assignment upperkeys missing")
	}
	if _, ok := a.DeferredBranches["DevContainer"]; !ok {
		t.Errorf("deferredBranch key was not kept verbatim; got %v", keysOfDeferred(a.DeferredBranches))
	}
}

func keysOfStrings(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysOfDeferred(m map[string]*DeferredBranchSource) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// The bare-list form of approvals must normalize into Rules, and encoding must
// always emit the modern {settings, rules} shape — that is what makes
// `glabs config migrate` upgrade files for free.
func TestDecodeNormalizesLegacyApprovalsList(t *testing.T) {
	t.Parallel()

	course, _ := decodeFixture(t, "legacy")

	a := course.Assignments["approvalslist"]
	if a == nil {
		t.Fatal("assignment approvalslist missing")
	}
	if a.MergeRequest.Approvals == nil {
		t.Fatal("Approvals is nil; the bare-list form was not accepted")
	}
	rules := a.MergeRequest.Approvals.Rules
	if len(rules) != 3 {
		t.Fatalf("len(Rules) = %d, want 3", len(rules))
	}
	if rules[0].Name != "review" || rules[0].RequiredApprovals != 2 {
		t.Errorf("rules[0] = %+v, want name=review requiredApprovals=2 (via the required_approvals alias)", rules[0])
	}

	encoded, err := EncodeCourse(course)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !containsAll(string(encoded), "approvals:", "rules:") {
		t.Errorf("encoded output does not use the modern approvals shape:\n%s", encoded)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// realCourseFixtures are the mirrors of actual course files. The synthetic
// fixtures (casecourse, legacy) deliberately carry odd config and are excluded.
var realCourseFixtures = []string{"mpd", "vss", "algdati", "fundc", "fun"}

// The schema must claim every key the real course files use, with no exceptions.
// This is the completeness guard, and it is what makes the round-trip test
// meaningful: a round trip would happily pass while dropping a field the decoder
// never knew about, because it would be absent on both sides.
//
// Anything reported here is a key the loader silently ignores — either a schema
// gap or a genuine typo in a course file. There used to be two classes of the
// latter (`clone.clone` in every file, and dockerImages nested one level too
// deep in vss/blatt2); both are fixed, which is why this allows nothing.
func TestSchemaClaimsEveryKeyInRealFixtures(t *testing.T) {
	t.Parallel()

	for _, name := range realCourseFixtures {
		_, result := decodeFixture(t, name)
		for _, key := range result.UnknownKeys {
			t.Errorf("%s.yaml: key %q is claimed by no field — schema gap or a typo in the fixture",
				name, key)
		}
	}
}

// decodeYAML decodes an inline course document. Used where a fixture would have
// to carry deliberately broken config that the completeness guard would then
// flag.
func decodeYAML(t *testing.T, doc string) (*CourseSource, *DecodeResult) {
	t.Helper()
	course, result, err := DecodeCourse([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeCourse: %v", err)
	}
	return course, result
}

// Unknown keys are the reason lint exists: the loader ignores them silently, so
// they look exactly like settings that work. Both cases below were real, and
// both were live in the course files until `glabs config lint` surfaced them.
func TestDecodeReportsUnknownKeys(t *testing.T) {
	t.Parallel()

	_, result := decodeYAML(t, `
demo:
  coursepath: demo/semester
  blatt1:
    assignmentpath: blatt-1
    clone:
      localpath: /tmp/demo
      clone: true               # no such option; a `+"`clone:`"+` block alone enables cloning
    release:
      mergeRequest:
        pipeline: true
        dockerImages:           # belongs under release:, not under release.mergeRequest:
          - service/one
`)

	for _, want := range []string{
		"demo.blatt1.clone.clone",
		"demo.blatt1.release.mergeRequest.dockerImages",
	} {
		if !contains(result.UnknownKeys, want) {
			t.Errorf("UnknownKeys = %v, want it to contain %q", result.UnknownKeys, want)
		}
	}
}

func TestDecodeReportsLegacyKeys(t *testing.T) {
	t.Parallel()

	_, result := decodeFixture(t, "legacy")
	if len(result.LegacyKeys) == 0 {
		t.Fatal("LegacyKeys is empty, want the deprecated approvals spellings reported")
	}
	var paths []string
	for _, k := range result.LegacyKeys {
		paths = append(paths, k.Path)
	}
	if !hasSuffix(paths, "required_approvals") {
		t.Errorf("LegacyKeys = %v, want required_approvals reported", paths)
	}
	if !hasSuffix(paths, "approvalsRequired") {
		t.Errorf("LegacyKeys = %v, want approvalsRequired reported", paths)
	}
}

// `approvalsRequired` used to be dead: viper lowercases keys to
// `approvalsrequired` before the alias table sees them, so the camelCase entry
// matched nothing and the rule silently required 0 approvals. Both loaders now
// fold case, so both accept it — asserted here and, for the viper path, by the
// legacy.approvalslist golden.
func TestDecodeAcceptsApprovalsRequiredAlias(t *testing.T) {
	t.Parallel()

	course, _ := decodeFixture(t, "legacy")
	rules := course.Assignments["approvalslist"].MergeRequest.Approvals.Rules

	var second *ApprovalRuleSource
	for i := range rules {
		if rules[i].Name == "second" {
			second = &rules[i]
		}
	}
	if second == nil {
		t.Fatalf("rule %q missing; got %d rules", "second", len(rules))
	}
	if second.RequiredApprovals != 1 {
		t.Errorf("RequiredApprovals = %d, want 1 via the approvalsRequired alias (viper yields 0 here)",
			second.RequiredApprovals)
	}
}

func TestDecodeCourseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "empty file", yaml: "", wantErr: "course file is empty"},
		{name: "two top-level keys", yaml: "a:\n  x: 1\nb:\n  y: 2\n", wantErr: "has 2 top-level keys (a, b)"},
		{name: "not yaml", yaml: "\tthis: [is: not", wantErr: "cannot parse course file as YAML"},
		{name: "course body is null", yaml: "onlykey:\n", wantErr: "configuration is empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := DecodeCourse([]byte(tt.yaml))
			if err == nil {
				t.Fatalf("DecodeCourse(%q) = nil error, want error containing %q", tt.yaml, tt.wantErr)
			}
			if !containsAll(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
