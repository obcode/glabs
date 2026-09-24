package graph

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/obcode/glabs/v3/web/app"
	"github.com/obcode/glabs/v3/web/graph/generated"
	"github.com/vektah/gqlparser/v2/ast"
)

type fakeApproval bool

func (f fakeApproval) IsApproved(context.Context) bool { return bool(f) }

// callGate runs one field through the gate and reports whether the resolver behind it ran.
func callGate(approved bool, object, field string) (ran bool, err error) {
	ctx := graphql.WithFieldContext(context.Background(), &graphql.FieldContext{
		Object: object,
		Field:  graphql.CollectedField{Field: &ast.Field{Name: field}},
	})
	_, err = accessGate(fakeApproval(approved))(ctx, func(context.Context) (any, error) {
		ran = true
		return nil, nil
	})
	return ran, err
}

// Every root field of the real schema: the open ones pass for an unapproved user,
// every other one is refused. Walking the schema rather than naming fields is the
// point -- a field added later is covered without anyone remembering this test.
func TestAccessGate_everyRootFieldOfTheSchema(t *testing.T) {
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: NewResolver(nil)}).Schema()
	seenOpen := map[string]bool{}
	for _, root := range []*ast.Definition{schema.Query, schema.Mutation, schema.Subscription} {
		if root == nil {
			continue
		}
		for _, f := range root.Fields {
			ran, err := callGate(false, root.Name, f.Name)
			if openRootFields[f.Name] {
				seenOpen[f.Name] = true
				if !ran || err != nil {
					t.Errorf("%s.%s is open but was refused: %v", root.Name, f.Name, err)
				}
				continue
			}
			if ran || !errors.Is(err, app.ErrNotApproved) {
				t.Errorf("%s.%s reached its resolver for an unapproved user (err %v)", root.Name, f.Name, err)
			}
			if ran, err := callGate(true, root.Name, f.Name); !ran || err != nil {
				t.Errorf("%s.%s refused an approved user: %v", root.Name, f.Name, err)
			}
		}
	}
	// The open list must name real fields; a typo there would silently close one.
	for _, name := range []string{"me", "serverInfo", "requestAccess"} {
		if !seenOpen[name] {
			t.Errorf("open field %q does not exist in the schema", name)
		}
	}
}

func TestAccessGate_nestedFieldsPass(t *testing.T) {
	if ran, err := callGate(false, "Course", "name"); !ran || err != nil {
		t.Errorf("a nested field must pass (its root was checked): ran=%v err=%v", ran, err)
	}
}

// End to end through the generated executor: proves the gate is hooked where
// gqlgen really calls it and that the object names are the ones it uses.
func TestAccessGate_throughExecutor(t *testing.T) {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: NewResolver(nil)}))
	srv.AddTransport(transport.POST{})
	srv.AroundFields(accessGate(fakeApproval(false)))

	post := func(query string) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"query": query})
		req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("response is not JSON: %q", rec.Body.String())
		}
		return out
	}

	if out := post(`{ serverInfo { version } }`); out["errors"] != nil {
		t.Errorf("serverInfo must be open: %v", out["errors"])
	}
	out := post(`{ courses { name } }`)
	if !strings.Contains(toJSON(out["errors"]), "not approved") {
		t.Errorf("courses must be refused with 'not approved', got %v", out)
	}
	out = post(`mutation { resetUser(email: "x@hm.edu") }`)
	if !strings.Contains(toJSON(out["errors"]), "not approved") {
		t.Errorf("an admin mutation must be refused too, got %v", out)
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
