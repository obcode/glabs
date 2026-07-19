package zpa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockZPA serves get_student_info from a map of ask→students, and records the
// Authorization header of the last request.
func mockZPA(t *testing.T, byAsk map[string][]*Student) (*Client, *string) {
	t.Helper()
	lastAuth := new(string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*lastAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/get_student_info" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		ask := r.URL.Query().Get("ask")
		_ = json.NewEncoder(w).Encode(byAsk[ask])
	}))
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, Token: "tok-123"}), lastAuth
}

func TestStudentByEmail_exactMatchAndAuthHeader(t *testing.T) {
	want := &Student{Mtknr: "123", FirstName: "Ada", LastName: "Ciftci", Email: "a.ciftci@hm.edu", Gender: "f", Group: "IF3A"}
	c, lastAuth := mockZPA(t, map[string][]*Student{
		"a.ciftci@hm.edu": {want},
	})

	got, err := c.StudentByEmail(context.Background(), "a.ciftci@hm.edu")
	if err != nil {
		t.Fatalf("StudentByEmail: %v", err)
	}
	if got == nil || got.Mtknr != "123" || got.FirstName != "Ada" || got.Group != "IF3A" {
		t.Fatalf("got %+v, want Ada Ciftci (123)", got)
	}
	if *lastAuth != "Token tok-123" {
		t.Errorf("Authorization = %q, want 'Token tok-123'", *lastAuth)
	}
}

func TestStudentByEmail_fallsBackToLocalPart(t *testing.T) {
	want := &Student{Mtknr: "9", FirstName: "Bo", LastName: "Rueedi", Email: "benjamin.rueedi@hm.edu"}
	// The full-email search returns nothing; the local-part search finds the student.
	c, _ := mockZPA(t, map[string][]*Student{
		"benjamin.rueedi": {want},
	})

	got, err := c.StudentByEmail(context.Background(), "benjamin.rueedi@hm.edu")
	if err != nil {
		t.Fatalf("StudentByEmail: %v", err)
	}
	if got == nil || got.Mtknr != "9" {
		t.Fatalf("got %+v, want the local-part match (9)", got)
	}
}

func TestStudentByEmail_noMatchIsNil(t *testing.T) {
	c, _ := mockZPA(t, map[string][]*Student{}) // ZPA knows nobody

	got, err := c.StudentByEmail(context.Background(), "nobody@hm.edu")
	if err != nil {
		t.Fatalf("StudentByEmail: %v", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil for an unknown email", got)
	}
}

func TestStudentByEmail_ambiguousLocalPartIsNil(t *testing.T) {
	// The local-part search returns two students, neither matching the email exactly
	// → must not guess.
	c, _ := mockZPA(t, map[string][]*Student{
		"mueller": {
			{Mtknr: "1", Email: "a.mueller@hm.edu"},
			{Mtknr: "2", Email: "b.mueller@hm.edu"},
		},
	})

	got, err := c.StudentByEmail(context.Background(), "mueller@hm.edu")
	if err != nil {
		t.Fatalf("StudentByEmail: %v", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil for an ambiguous match", got)
	}
}

func TestStudentByEmail_serverErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	c := New(Config{BaseURL: srv.URL, Token: "bad"})

	if _, err := c.StudentByEmail(context.Background(), "a@hm.edu"); err == nil {
		t.Error("a non-2xx ZPA response should be an error")
	}
}
