package gitlab

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/obcode/glabs/v2/config"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

func newTestClient(t *testing.T, idUsers map[int]string, searchUsers map[string][]string) *Client {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			search := r.URL.Query().Get("search")
			users, ok := searchUsers[search]
			if !ok {
				_, _ = w.Write([]byte("[]"))
				return
			}
			resp := "["
			for i, username := range users {
				if i > 0 {
					resp += ","
				}
				resp += fmt.Sprintf(`{"id":%d,"name":"%s","username":"%s"}`, i+100, username, username)
			}
			resp += "]"
			_, _ = w.Write([]byte(resp))
			return

		case r.Method == http.MethodGet && len(r.URL.Path) > len("/api/v4/users/") && r.URL.Path[:14] == "/api/v4/users/":
			var id int
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v4/users/%d", &id)
			username, ok := idUsers[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"404 User Not Found"}`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%d,"name":"%s","username":"%s"}`, id, username, username)))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	apiClient, err := gitlabapi.NewClient("token", gitlabapi.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("creating gitlab test client failed: %v", err)
	}

	return &Client{apiClient}
}

func TestCheckDupsInGroups_NoDuplicates(t *testing.T) {
	groups := []*config.Group{
		{Name: "g1", Members: []*config.Student{{Raw: "alice"}, {Raw: "bob"}}},
		{Name: "g2", Members: []*config.Student{{Raw: "carol"}}},
	}

	dups := checkDupsInGroups(groups)
	if len(dups) != 0 {
		t.Fatalf("expected no duplicates, got %#v", dups)
	}
}

func TestCheckDupsInGroups_WithDuplicates(t *testing.T) {
	groups := []*config.Group{
		{Name: "g1", Members: []*config.Student{{Raw: "alice"}, {Raw: "bob"}}},
		{Name: "g2", Members: []*config.Student{{Raw: "bob"}, {Raw: "dave"}}},
		{Name: "g3", Members: []*config.Student{{Raw: "alice"}}},
	}

	dups := checkDupsInGroups(groups)

	if len(dups) != 2 {
		t.Fatalf("expected two duplicate entries, got %#v", dups)
	}

	if !reflect.DeepEqual(dups["alice"], []string{"g1", "g3"}) {
		t.Fatalf("alice groups = %#v", dups["alice"])
	}
	if !reflect.DeepEqual(dups["bob"], []string{"g1", "g2"}) {
		t.Fatalf("bob groups = %#v", dups["bob"])
	}
}

func TestCheckCourseReturnsTrueForResolvableStudents(t *testing.T) {
	id := 1001
	username := "alice"
	email := "new.user@example.org"

	client := newTestClient(t,
		map[int]string{1001: "id-user"},
		map[string][]string{
			"alice":                {"alice"},
			"new.user@example.org": {},
		},
	)

	cfg := &config.CourseConfig{
		Course: "course",
		Students: []*config.Student{
			{Id: &id, Raw: "1001"},
			{Username: &username, Raw: "alice"},
			{Email: &email, Raw: email},
		},
	}

	if ok := client.CheckCourse(cfg); !ok {
		t.Fatal("CheckCourse() = false, want true")
	}
}

func TestCheckCourseReturnsFalseOnMissingUserAndDuplicate(t *testing.T) {
	missing := "missinguser"

	client := newTestClient(t,
		map[int]string{},
		map[string][]string{},
	)

	cfg := &config.CourseConfig{
		Course: "course",
		Groups: []*config.Group{
			{
				Name: "g1",
				Members: []*config.Student{
					{Username: &missing, Raw: "missinguser"},
					{Raw: "dup"},
				},
			},
			{
				Name: "g2",
				Members: []*config.Student{
					{Raw: "dup"},
				},
			},
		},
	}

	if ok := client.CheckCourse(cfg); ok {
		t.Fatal("CheckCourse() = true, want false")
	}
}
