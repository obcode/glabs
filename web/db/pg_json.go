package db

import (
	"encoding/json"
	"fmt"
)

// The helpers below exist for one reason: keeping "absent" absent.
//
// In MongoDB an `omitempty` field that is empty is simply not in the document,
// and reads back as a nil map or a nil slice. Several callers depend on that
// distinction -- ScheduledJob.OnlyFor nil means "the whole assignment" while an
// empty slice would mean "nobody" -- so it has to survive the move.
//
// json.Marshal of a nil map produces the four bytes `null`, which jsonb happily
// stores as a JSON null. That is not the same as SQL NULL: the column would be
// non-null, containing a null, and every `params is null` written later would be
// wrong. So a nil map becomes a nil []byte and therefore SQL NULL.

// marshalStringMap encodes a map for a jsonb column, keeping nil as SQL NULL.
//
// An EMPTY but non-nil map is also stored as NULL, and comes back nil. That is a
// deliberate small lie: Mongo's `omitempty` could not tell the two apart either,
// so the round trip already collapsed them, and the store contract suite pins
// the collapsed behaviour for both implementations.
func marshalStringMap(m map[string]string) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("cannot encode params: %w", err)
	}
	return b, nil
}

// unmarshalStringMap decodes a jsonb column, keeping SQL NULL as a nil map.
func unmarshalStringMap(b []byte) (map[string]string, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("cannot decode params: %w", err)
	}
	if len(m) == 0 {
		return nil, nil
	}
	return m, nil
}

// emptyToNil normalises an empty slice to nil.
//
// pgx decodes an empty text[] as an empty non-nil slice, and `emit_empty_slices`
// makes sqlc do the same for result sets. Neither is wrong; both differ from
// what the Mongo store returned, and OnlyFor is the field where that difference
// changes which repositories an operation touches.
func emptyToNil[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return s
}
