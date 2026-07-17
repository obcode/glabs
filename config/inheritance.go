package config

import "fmt"

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
