package config

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// inheritKey is the assignment config field used to inherit configuration from
// another assignment within the same course (OOP-style single inheritance).
//
//	blatt10:
//	  extends: blatt09     # inherit everything from blatt09 ...
//	  assignmentpath: blatt-10   # ... and override only what differs
const inheritKey = "extends"

// abstractKey marks an assignment as an abstract base that exists only to be
// inherited from via `extends`. Abstract assignments cannot be operated on
// directly (generate, protect, clone, …). The flag itself is never inherited.
const abstractKey = "abstract"

// assignmentIsAbstract reports whether the assignment declares `abstract: true`
// on itself. It reads the assignment's own value and must be called before
// resolveAssignmentInheritance writes the merged config back into viper, so an
// inherited abstract flag never makes a concrete child abstract.
func assignmentIsAbstract(course, assignment string) bool {
	return viper.GetBool(course + "." + assignment + "." + abstractKey)
}

// resolveAssignmentInheritance resolves the `extends` chain for the given
// assignment and writes the merged, effective configuration back into viper at
// the assignment key. After this call the rest of the config loading reads the
// inherited values transparently via the usual viper.Get* calls.
//
// Inheritance semantics (child overrides parent):
//   - maps (e.g. mergeRequest, startercode, deferredBranches) are deep-merged,
//     so a child can override a single nested field while keeping the rest;
//   - scalars and slices (e.g. branches, issueNumbers) are replaced wholesale.
//
// Parents may themselves extend other assignments; chains are resolved
// recursively. Cycles and missing parents are fatal.
func resolveAssignmentInheritance(course, assignment string) {
	assignmentKey := course + "." + assignment
	if !viper.IsSet(assignmentKey + "." + inheritKey) {
		return
	}

	merged := mergedAssignmentMap(course, assignment, map[string]bool{})
	// Meta keys must never leak into the effective config or be inherited.
	delete(merged, inheritKey)
	delete(merged, abstractKey)
	viper.Set(assignmentKey, merged)
}

// mergedAssignmentMap returns the assignment's configuration map with all parent
// configuration (via `extends`) merged in. The child's own values win.
func mergedAssignmentMap(course, assignment string, seen map[string]bool) map[string]any {
	if seen[assignment] {
		log.Fatal().
			Str("course", course).
			Str("assignment", assignment).
			Msg("cyclic 'extends' inheritance detected in assignment configuration")
	}
	seen[assignment] = true

	own := viper.GetStringMap(course + "." + assignment)

	parentRaw, ok := own[inheritKey]
	if !ok {
		return own
	}

	parent, ok := parentRaw.(string)
	if !ok || strings.TrimSpace(parent) == "" {
		log.Fatal().
			Str("course", course).
			Str("assignment", assignment).
			Msg("'extends' must be the name of another assignment in the same course")
	}
	parent = strings.TrimSpace(parent)

	if !viper.IsSet(course + "." + parent) {
		log.Fatal().
			Str("course", course).
			Str("assignment", assignment).
			Str("extends", parent).
			Msg("assignment referenced by 'extends' not found")
	}

	parentMap := mergedAssignmentMap(course, parent, seen)

	return deepMerge(parentMap, own)
}

// deepMerge returns a new map with child merged onto parent. Nested maps are
// merged recursively; all other values (scalars, slices) are replaced by the
// child's value.
func deepMerge(parent, child map[string]any) map[string]any {
	out := make(map[string]any, len(parent)+len(child))
	for k, v := range parent {
		out[k] = v
	}
	for k, childVal := range child {
		if parentVal, ok := out[k]; ok {
			parentMap, parentIsMap := asStringMap(parentVal)
			childMap, childIsMap := asStringMap(childVal)
			if parentIsMap && childIsMap {
				out[k] = deepMerge(parentMap, childMap)
				continue
			}
		}
		out[k] = childVal
	}
	return out
}

// asStringMap normalizes the YAML/viper map representations (map[string]any or
// map[any]any) into map[string]any, reporting whether the value was a map.
func asStringMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}
