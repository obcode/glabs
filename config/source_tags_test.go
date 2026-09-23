package config

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestSourceTagsAreComplete walks CourseSource and everything reachable from it
// and requires every exported field to carry both a bson and a json tag, with
// the same name.
//
// The failure it exists to prevent is silent in both directions. Without a bson
// tag the MongoDB driver lowercases the Go field name, so UseEmailDomainAsSuffix
// is stored as "useemaildomainassuffix" and the value written last week no
// longer reads back. Without a json tag encoding/json capitalises it instead.
// Either way nothing errors: the field simply comes back as its zero value, and
// for a *bool whose absence means true, that flips behaviour rather than losing
// a string.
//
// A test over the types rather than over a fixture is the only version of this
// that keeps working: a field added next year is covered the day it is added,
// without anyone remembering this file exists.
func TestSourceTagsAreComplete(t *testing.T) {
	seen := map[reflect.Type]bool{}

	var walk func(t *testing.T, typ reflect.Type, path string)
	walk = func(t *testing.T, typ reflect.Type, path string) {
		t.Helper()
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true

		for i := range typ.NumField() {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			where := path + "." + f.Name

			bsonName, bsonOK := tagName(f, "bson")
			jsonName, jsonOK := tagName(f, "json")
			switch {
			case !bsonOK:
				t.Errorf("%s has no bson tag — MongoDB would store it under a lowercased field name", where)
			case !jsonOK:
				t.Errorf("%s has no json tag — PostgreSQL would store it under the Go field name", where)
			case bsonName != jsonName:
				t.Errorf("%s is stored as %q in bson but %q in json; the two storage formats must agree",
					where, bsonName, jsonName)
			}
			walk(t, f.Type, where)
		}
	}

	walk(t, reflect.TypeOf(CourseSource{}), "CourseSource")

	// A guard on the guard: if the walk ever stops descending, the loop above
	// silently checks nothing and still passes.
	if len(seen) < 10 {
		t.Errorf("only walked %d types, expected the whole CourseSource tree — the walk stopped descending", len(seen))
	}
}

// tagName returns the stored name for a field in one tag set. The "-" form
// counts as missing on purpose: a field excluded from storage is not a field
// that round-trips, and on CourseSource there is no such field today.
func tagName(f reflect.StructField, key string) (string, bool) {
	tag, ok := f.Tag.Lookup(key)
	if !ok {
		return "", false
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" || name == "-" {
		return "", false
	}
	return name, true
}

// TestSourceSurvivesJSONRoundTrip fills every field in the tree with a distinct
// non-zero value through reflection, encodes it as JSON and decodes it back.
//
// Reflection rather than a hand-written fixture because the fixture is the part
// that goes stale: a field added to source.go is covered here the day it is
// added, whereas a literal keeps passing while quietly saying nothing about the
// new field. Non-zero matters for the same reason — a zero value survives being
// dropped, so a fixture full of them proves nothing.
func TestSourceSurvivesJSONRoundTrip(t *testing.T) {
	// Fixed seed: a failure has to be reproducible, and the values only need to
	// be distinguishable from the zero value, not unpredictable.
	want := &CourseSource{}
	fill(t, reflect.ValueOf(want).Elem(), rand.New(rand.NewSource(1)), 0)

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	got := &CourseSource{}
	if err := json.Unmarshal(encoded, got); err != nil {
		t.Fatalf("cannot decode: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the course source did not survive a JSON round trip (-want +got):\n%s\n"+
			"a field listed here is not being stored — check its json tag", diff)
	}
}

// fill gives every field in v a distinct non-zero value. depth bounds the
// recursion: the type graph is finite but nested, and a map of pointers to
// structs would otherwise be built out further than the test needs.
func fill(t *testing.T, v reflect.Value, rnd *rand.Rand, depth int) {
	t.Helper()
	if depth > 4 {
		return
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(randomString(rnd))
	case reflect.Bool:
		// Always true: false is the zero value and would not show a dropped field.
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(rnd.Intn(1000) + 1))
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(t, v.Elem(), rnd, depth+1)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := range 2 {
			fill(t, s.Index(i), rnd, depth+1)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		for range 2 {
			key := reflect.New(v.Type().Key()).Elem()
			fill(t, key, rnd, depth+1)
			val := reflect.New(v.Type().Elem()).Elem()
			fill(t, val, rnd, depth+1)
			m.SetMapIndex(key, val)
		}
		v.Set(m)
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				fill(t, v.Field(i), rnd, depth+1)
			}
		}
	default:
		t.Fatalf("fill does not know how to populate %s — extend it rather than skipping the field", v.Kind())
	}
}

func randomString(rnd *rand.Rand) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 8)
	for i := range b {
		b[i] = alphabet[rnd.Intn(len(alphabet))]
	}
	return string(b)
}
