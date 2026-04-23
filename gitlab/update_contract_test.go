package gitlab

import (
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
)

// ---- Update (top-level) -----------------------------------------------------

func TestUpdate_GroupNotFound_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerStudent,
	}
	assertExitCode(t, 1, func() { client.Update(cfg) })
}

func TestUpdate_InvalidPer_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerFailed,
	}
	assertExitCode(t, 1, func() { client.Update(cfg) })
}

// ---- updatePerStudent -------------------------------------------------------

func TestUpdatePerStudent_NoStudents(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:   "mpd",
		Name:     "blatt01",
		Path:     "mpd/ss26/blatt-01",
		Per:      config.PerStudent,
		Students: []*config.Student{},
	}
	client.updatePerStudent(cfg, nil)
}

func TestUpdatePerStudent_GetProjectFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
	}
	// GetProject fails → prints error, returns
	client.updatePerStudent(cfg, nil)
}

func TestUpdatePerStudent_NoStartercode_Success(t *testing.T) {
	// No starterrepo → update function just starts/stops spinner, no actual API calls
	pj := `{"id":20,"name":"mpd-blatt01-alice","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-alice","ssh_url_to_repo":"git@example.com:p.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-alice") {
			_, _ = w.Write([]byte(pj))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
	}
	// starterrepo=nil → update() just logs, no push
	client.updatePerStudent(cfg, nil)
}

// ---- updatePerGroup ---------------------------------------------------------

func TestUpdatePerGroup_NoGroups(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerGroup,
		Groups: []*config.Group{},
	}
	client.updatePerGroup(cfg, nil)
}

func TestUpdatePerGroup_GetProjectFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	alice := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerGroup,
		UseCoursenameAsPrefix: true,
		Groups: []*config.Group{
			{Name: "team1", Members: []*config.Student{{Username: &alice, Raw: "alice"}}},
		},
	}
	client.updatePerGroup(cfg, nil)
}

func TestUpdatePerGroup_NoStartercode_Success(t *testing.T) {
	pj := `{"id":21,"name":"mpd-blatt01-team1","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-team1","ssh_url_to_repo":"git@example.com:p.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-team1") {
			_, _ = w.Write([]byte(pj))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	alice := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerGroup,
		UseCoursenameAsPrefix: true,
		Groups: []*config.Group{
			{Name: "team1", Members: []*config.Student{{Username: &alice, Raw: "alice"}}},
		},
	}
	client.updatePerGroup(cfg, nil)
}
