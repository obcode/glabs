package gitlab

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

// projectJSONStr returns a minimal project JSON body.
func projectJSONStr(id int64, name, pathNS string) string {
	return fmt.Sprintf(`{"id":%d,"name":%q,"path_with_namespace":%q,"ssh_url_to_repo":"git@gitlab.example.org:%s.git"}`,
		id, name, pathNS, pathNS)
}

// -- Archive ------------------------------------------------------------------

func TestArchive_GroupNotFound_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Group Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Path:        "mpd/ss26/blatt-01",
		URL:         "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:         config.PerStudent,
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	assertExitCode(t, 1, func() { client.Archive(cfg, false) })
}

func TestArchive_InvalidPer_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Path:        "mpd/ss26/blatt-01",
		Per:         config.PerFailed,
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	assertExitCode(t, 1, func() { client.Archive(cfg, false) })
}

// -- archivePerStudent --------------------------------------------------------

func TestArchivePerStudent_NoStudents(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Name:        "blatt01",
		Path:        "mpd/ss26/blatt-01",
		Per:         config.PerStudent,
		Students:    []*config.Student{},
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	// No students → returns immediately without calling API
	client.archivePerStudent(cfg, false)
}

func TestArchivePerStudent_GetProjectFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
		Startercode:           &config.Startercode{ToBranch: "main"},
	}
	// GetProject fails → prints error, returns
	client.archivePerStudent(cfg, false)
}

func TestArchivePerStudent_Success_Archive(t *testing.T) {
	pj := `{"id":1,"name":"mpd-blatt01-alice","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-alice","ssh_url_to_repo":"git@gitlab.example.org:mpd/ss26/blatt-01/mpd-blatt01-alice.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-alice"):
			_, _ = w.Write([]byte(pj))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/archive"):
			_, _ = w.Write([]byte(pj))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
		Startercode:           &config.Startercode{ToBranch: "main"},
	}
	client.archivePerStudent(cfg, false)
}

func TestArchivePerStudent_Success_Unarchive(t *testing.T) {
	pj := `{"id":1,"name":"mpd-blatt01-alice","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-alice","ssh_url_to_repo":"git@gitlab.example.org:mpd/ss26/blatt-01/mpd-blatt01-alice.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-alice"):
			_, _ = w.Write([]byte(pj))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/unarchive"):
			_, _ = w.Write([]byte(pj))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
		Startercode:           &config.Startercode{ToBranch: "main"},
	}
	client.archivePerStudent(cfg, true) // unarchive=true
}

// -- archivePerGroup ----------------------------------------------------------

func TestArchivePerGroup_NoGroups(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Name:        "blatt01",
		Path:        "mpd/ss26/blatt-01",
		Per:         config.PerGroup,
		Groups:      []*config.Group{},
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	client.archivePerGroup(cfg, false)
}

func TestArchivePerGroup_GetProjectFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:                   config.PerGroup,
		UseCoursenameAsPrefix: true,
		Groups:                []*config.Group{{Name: "team1"}},
		Startercode:           &config.Startercode{ToBranch: "main"},
	}
	client.archivePerGroup(cfg, false)
}

func TestArchivePerGroup_Success(t *testing.T) {
	pj := `{"id":2,"name":"mpd-blatt01-team1","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-team1","ssh_url_to_repo":"git@gitlab.example.org:mpd/ss26/blatt-01/mpd-blatt01-team1.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-team1"):
			_, _ = w.Write([]byte(pj))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/archive"):
			_, _ = w.Write([]byte(pj))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	alice := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		URL:                   "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:                   config.PerGroup,
		UseCoursenameAsPrefix: true,
		Groups: []*config.Group{
			{Name: "team1", Members: []*config.Student{{Username: &alice, Raw: "alice"}}},
		},
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	client.archivePerGroup(cfg, false)
}

// -- archive (low-level) ------------------------------------------------------

func TestArchive_ArchiveFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/archive") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"500 Internal Server Error"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	err := client.archive(cfg, &gitlabapi.Project{ID: 3, Name: "myrepo"}, false, false)
	if err == nil {
		t.Fatal("archive() expected error on 500, got nil")
	}
}

func TestArchive_UnarchiveFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/unarchive") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	err := client.archive(cfg, &gitlabapi.Project{ID: 4, Name: "myrepo"}, false, true)
	if err == nil {
		t.Fatal("archive() unarchive expected error on 403, got nil")
	}
}

func TestArchive_Success(t *testing.T) {
	pj := projectJSONStr(5, "myrepo", "mpd/ss26/myrepo")
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/archive") {
			_, _ = w.Write([]byte(pj))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Startercode: &config.Startercode{ToBranch: "main"},
	}
	err := client.archive(cfg, &gitlabapi.Project{ID: 5, Name: "myrepo"}, false, false)
	if err != nil {
		t.Fatalf("archive() error = %v", err)
	}
}
