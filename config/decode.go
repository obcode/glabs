package config

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

// Decoding a course file happens in two hops:
//
//	bytes --yaml.v3--> map[string]any --mapstructure--> *CourseSource
//
// yaml.v3 only ever sees generic maps, so its case-sensitive tag matching never
// applies to field names. mapstructure does the struct mapping and folds case
// (MatchName defaults to strings.EqualFold), which reproduces viper's key
// handling exactly — viper decodes with mapstructure internally, which is why
// `frombranch` and `fromBranch` are interchangeable today.
//
// Assignments are decoded one at a time rather than in a single pass over the
// course. mapstructure's Metadata cannot name a key inside a ",remain" map — it
// reports `[<interface {} Value>].clone.clone` — and `glabs config lint` is
// only useful if it can say *which* assignment carries the stray key.

// DecodeResult carries what a decode learned about the input beyond the values
// themselves: keys nobody claimed, and deprecated spellings that were accepted.
// `glabs config lint` reports these; ordinary loading ignores them.
type DecodeResult struct {
	// UnknownKeys are dotted paths present in the file that no field matches.
	// They are silently ignored by the loader, which is exactly why they are
	// worth reporting: `clone.clone` and `release.mergeRequest.dockerImages`
	// both look effective and are not.
	UnknownKeys []string
	// LegacyKeys are dotted paths that were accepted via a deprecated alias,
	// with an explanation.
	LegacyKeys []LegacyKey
}

type LegacyKey struct {
	Path string
	Hint string
}

// legacyAliases maps deprecated key spellings to their canonical field name.
// Lookup folds case, which matters: viper lowercases keys before its own alias
// table sees them, which is why `approvalsRequired` never worked there. Here it
// does — see TestDecodeAcceptsAliasViperMissed.
var legacyAliases = map[string]string{
	"allow_force_push":             "allowForcePush",
	"code_owner_approval_required": "codeOwnerApprovalRequired",
	"required_approvals":           "requiredApprovals",
	"approvalsrequired":            "requiredApprovals",
}

// courseSourceRaw mirrors CourseSource but keeps the assignments unparsed, so
// each can be decoded separately with its own metadata.
type courseSourceRaw struct {
	CoursePath             string              `mapstructure:"coursepath"`
	SemesterPath           string              `mapstructure:"semesterpath"`
	UseCoursenameAsPrefix  bool                `mapstructure:"useCoursenameAsPrefix"`
	UseEmailDomainAsSuffix *bool               `mapstructure:"useEmailDomainAsSuffix"`
	Students               []string            `mapstructure:"students"`
	Groups                 map[string][]string `mapstructure:"groups"`
	Assignments            map[string]any      `mapstructure:",remain"`
}

// DecodeCourse decodes a course file. The file must have exactly one top-level
// key — the course name — whose value is the course mapping.
func DecodeCourse(data []byte) (*CourseSource, *DecodeResult, error) {
	var top map[string]any
	if err := yaml.Unmarshal(data, &top); err != nil {
		return nil, nil, fmt.Errorf("cannot parse course file as YAML: %w", err)
	}

	switch len(top) {
	case 1:
	case 0:
		return nil, nil, fmt.Errorf("course file is empty: expected a single top-level key naming the course")
	default:
		names := make([]string, 0, len(top))
		for name := range top {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, nil, fmt.Errorf("course file has %d top-level keys (%s): expected exactly one, naming the course",
			len(top), strings.Join(names, ", "))
	}

	var name string
	var body any
	for k, v := range top {
		name, body = k, v
	}

	return DecodeCourseBody(name, body)
}

// DecodeCourseBody decodes an already-parsed course mapping. The web server
// uses this for documents coming from MongoDB rather than from a file.
func DecodeCourseBody(name string, body any) (*CourseSource, *DecodeResult, error) {
	if body == nil {
		return nil, nil, fmt.Errorf("course %s: configuration is empty", name)
	}

	normalized, legacy := normalizeLegacyKeys(body, name)

	var raw courseSourceRaw
	var md mapstructure.Metadata
	if err := decodeInto(&raw, normalized, &md); err != nil {
		return nil, nil, fmt.Errorf("course %s: cannot decode course settings: %w", name, err)
	}

	course := &CourseSource{
		Name:                   name,
		CoursePath:             raw.CoursePath,
		SemesterPath:           raw.SemesterPath,
		UseCoursenameAsPrefix:  raw.UseCoursenameAsPrefix,
		UseEmailDomainAsSuffix: raw.UseEmailDomainAsSuffix,
		Students:               raw.Students,
		Groups:                 raw.Groups,
	}

	result := &DecodeResult{LegacyKeys: legacy}
	result.UnknownKeys = append(result.UnknownKeys, prefixPaths(name, md.Unused)...)

	if len(raw.Assignments) > 0 {
		course.Assignments = make(map[string]*AssignmentSource, len(raw.Assignments))
	}
	for assignmentName, assignmentBody := range raw.Assignments {
		path := name + "." + assignmentName
		if assignmentBody == nil {
			return nil, nil, fmt.Errorf("%s: assignment configuration is empty", path)
		}

		var assignment AssignmentSource
		var amd mapstructure.Metadata
		if err := decodeInto(&assignment, assignmentBody, &amd); err != nil {
			return nil, nil, fmt.Errorf("%s: cannot decode assignment: %w", path, err)
		}
		course.Assignments[assignmentName] = &assignment
		result.UnknownKeys = append(result.UnknownKeys, prefixPaths(path, amd.Unused)...)
	}

	sort.Strings(result.UnknownKeys)
	return course, result, nil
}

func decodeInto(target any, input any, md *mapstructure.Metadata) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           target,
		Metadata:         md,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			approvalsDecodeHook,
		),
	})
	if err != nil {
		return fmt.Errorf("cannot create decoder: %w", err)
	}
	return decoder.Decode(input)
}

func prefixPaths(prefix string, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		full := prefix + "." + p
		if _, ok := seen[full]; ok {
			continue
		}
		seen[full] = struct{}{}
		out = append(out, full)
	}
	return out
}

// approvalsDecodeHook accepts the legacy bare-list form of
// mergeRequest.approvals by rewriting it into the modern {rules: [...]} shape
// before the struct decode sees it (see config/assignment.go:333-344 for the
// behaviour this reproduces).
func approvalsDecodeHook(from reflect.Type, to reflect.Type, data any) (any, error) {
	if to != reflect.TypeOf(ApprovalsSource{}) {
		return data, nil
	}
	switch from.Kind() {
	case reflect.Slice, reflect.Array:
		return map[string]any{"rules": data}, nil
	default:
		return data, nil
	}
}

// normalizeLegacyKeys rewrites deprecated key spellings to their canonical form
// and records where it did so. It walks the raw tree because the aliases are
// key names, not values, and mapstructure has no per-key alias facility.
func normalizeLegacyKeys(value any, path string) (any, []LegacyKey) {
	var found []LegacyKey

	var walk func(any, string) any
	walk = func(v any, p string) any {
		switch typed := v.(type) {
		case map[string]any:
			out := make(map[string]any, len(typed))
			for key, item := range typed {
				childPath := p + "." + key
				if canonical, ok := legacyAliases[strings.ToLower(key)]; ok {
					found = append(found, LegacyKey{
						Path: childPath,
						Hint: fmt.Sprintf("%q is a deprecated spelling of %q", key, canonical),
					})
					key = canonical
				}
				out[key] = walk(item, childPath)
			}
			return out
		case []any:
			out := make([]any, len(typed))
			for i, item := range typed {
				out[i] = walk(item, fmt.Sprintf("%s[%d]", p, i))
			}
			return out
		default:
			return v
		}
	}

	normalized := walk(value, path)
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return normalized, found
}
