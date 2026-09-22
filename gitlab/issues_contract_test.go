package gitlab

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/config"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

func TestGetStartercodeProject_ParseSSHURL(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v4/projects/") {
			_, _ = w.Write([]byte(`{"id":42,"path_with_namespace":"mpd/startercode/blatt-01"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "git@gitlab.example.org:mpd/startercode/blatt-01.git"},
	}

	project, err := client.getStartercodeProject(cfg)
	if err != nil {
		t.Fatalf("getStartercodeProject() error = %v", err)
	}
	if project == nil || project.ID != 42 {
		t.Fatalf("unexpected project: %#v", project)
	}
}

func TestGetStartercodeProject_ParseHTTPSURLWithoutGitSuffix(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v4/projects/") {
			_, _ = w.Write([]byte(`{"id":43,"path_with_namespace":"mpd/startercode/blatt-02"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "https://gitlab.example.org/mpd/startercode/blatt-02"},
	}

	project, err := client.getStartercodeProject(cfg)
	if err != nil {
		t.Fatalf("getStartercodeProject() error = %v", err)
	}
	if project == nil || project.ID != 43 {
		t.Fatalf("unexpected project: %#v", project)
	}
}

func TestGetStartercodeProject_InvalidURL(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "not-a-valid-url"},
	}

	_, err := client.getStartercodeProject(cfg)
	if err == nil {
		t.Fatal("expected parse error for invalid startercode URL")
	}
}

func TestGetStartercodeProject_ProjectLookupFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "https://gitlab.example.org/mpd/startercode/blatt-03.git"},
	}

	_, err := client.getStartercodeProject(cfg)
	if err == nil {
		t.Fatal("expected project lookup error")
	}
}

func TestReplicateIssue_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"Fix tests","description":"Please fix tests"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			_, _ = w.Write([]byte(`{"id":9901,"iid":99}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2, PathWithNamespace: "mpd/ss26/blatt-01/team1"}

	if _, err := client.replicateIssue(source, target, 7, false); err != nil {
		t.Fatalf("replicateIssue() error = %v", err)
	}
}

func TestReplicateIssue_GetIssueFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2}

	_, err := client.replicateIssue(source, target, 7, false)
	if err == nil {
		t.Fatal("expected error when loading issue fails")
	}
}

func TestReplicateIssue_CreateIssueFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"Fix tests","description":"Please fix tests"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2}

	_, err := client.replicateIssue(source, target, 7, false)
	if err == nil {
		t.Fatal("expected error when creating issue fails")
	}
}

func TestResolveIssueNumbersForReplication_WithChildTasks(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v4/projects/1/issues/") {
			if strings.HasSuffix(r.URL.Path, "/2") {
				_, _ = w.Write([]byte(`{"id":2002,"iid":2,"title":"Aufgabenstellung","description":"Root"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/5") {
				_, _ = w.Write([]byte(`{"id":2005,"iid":5,"title":"Teilaufgabe 1","description":"Child 1"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/6") {
				_, _ = w.Write([]byte(`{"id":2006,"iid":6,"title":"Teilaufgabe 2","description":"Child 2"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost || r.URL.Path != "/api/graphql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		iid, _ := req.Variables["iid"].(string)
		switch iid {
		case "2":
			_, _ = w.Write([]byte(`{"data":{"project":{"issue":{"workItem":{"widgets":[{"children":{"nodes":[{"iid":"5"},{"iid":"6"}]}}]}}}}}`))
			return
		case "5", "6":
			_, _ = w.Write([]byte(`{"data":{"project":{"issue":{"workItem":{"widgets":[]}}}}}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-07"}

	numbers, err := client.resolveIssueNumbersForReplication(source, []int{2}, true)
	if err != nil {
		t.Fatalf("resolveIssueNumbersForReplication() error = %v", err)
	}

	if len(numbers) != 3 || numbers[0] != 2 || numbers[1] != 5 || numbers[2] != 6 {
		t.Fatalf("resolved issue numbers = %#v, want [2 5 6]", numbers)
	}
}

// Issue #151: labels, assignees and the closed state travel with a replicated issue.
func TestReplicateIssue_CopiesLabelsAssigneesAndClosedState(t *testing.T) {
	var (
		createBody   map[string]any
		updateBody   map[string]any
		createdLabel map[string]any
	)

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"Fix tests","description":"d",
				"state":"closed","labels":["bug","docs"],
				"assignees":[{"id":11,"username":"student"},{"id":22,"username":"lecturer"}]}`))
		// The target knows "bug" already; "docs" has to be created.
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/2/labels":
			_, _ = w.Write([]byte(`[{"id":501,"name":"bug","color":"#ff0000"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/labels":
			_, _ = w.Write([]byte(`[{"id":301,"name":"bug","color":"#ff0000"},
				{"id":302,"name":"docs","color":"#00ff00","description":"documentation"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/labels":
			if err := json.NewDecoder(r.Body).Decode(&createdLabel); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":502,"name":"docs","color":"#00ff00"}`))
		// Only the student is a member of the target project.
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/2/members/all":
			_, _ = w.Write([]byte(`[{"id":11,"username":"student"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":9901,"iid":99}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v4/projects/2/issues/99":
			if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":9901,"iid":99,"state":"closed"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2, PathWithNamespace: "mpd/ss26/blatt-01/team1"}

	if _, err := client.replicateIssue(source, target, 7, false); err != nil {
		t.Fatalf("replicateIssue() error = %v", err)
	}

	// LabelOptions marshals as ONE comma-joined string, not a JSON array — that is the REST
	// convention GitLab expects (see LabelOptions.MarshalJSON in the client library).
	if createBody["labels"] != "bug,docs" {
		t.Errorf("labels = %v, want \"bug,docs\"", createBody["labels"])
	}

	// The lecturer (22) is not a member of the target and must be dropped, otherwise GitLab
	// rejects the whole create call.
	assignees, _ := createBody["assignee_ids"].([]any)
	if len(assignees) != 1 || assignees[0].(float64) != 11 {
		t.Errorf("assignee_ids = %v, want [11]", createBody["assignee_ids"])
	}

	// The missing label is created with the SOURCE colour, so it does not look different in
	// every student repository.
	if createdLabel["color"] != "#00ff00" {
		t.Errorf("created label colour = %v, want #00ff00", createdLabel["color"])
	}
	if createdLabel["description"] != "documentation" {
		t.Errorf("created label description = %v, want documentation", createdLabel["description"])
	}

	if updateBody["state_event"] != "close" {
		t.Errorf("state_event = %v, want close — a closed source issue must end up closed", updateBody["state_event"])
	}
}

// An open source issue must not be closed, and must not trigger an update call at all.
func TestReplicateIssue_OpenIssueIsNotClosed(t *testing.T) {
	updateCalled := false

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"t","description":"d","state":"opened"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			_, _ = w.Write([]byte(`{"id":9901,"iid":99}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v4/projects/2/issues/99":
			updateCalled = true
			_, _ = w.Write([]byte(`{"id":9901,"iid":99}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2, PathWithNamespace: "mpd/ss26/blatt-01/team1"}

	if _, err := client.replicateIssue(source, target, 7, false); err != nil {
		t.Fatalf("replicateIssue() error = %v", err)
	}
	if updateCalled {
		t.Error("an open issue must not be updated after creation")
	}
}

// Metadata is best-effort: losing a label or the member lookup is not worth losing the issue.
func TestReplicateIssue_MetadataFailuresDoNotFailReplication(t *testing.T) {
	var createBody map[string]any

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"t","description":"d",
				"state":"opened","labels":["bug"],"assignees":[{"id":11,"username":"student"}]}`))
		case r.URL.Path == "/api/v4/projects/2/members/all":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		case r.URL.Path == "/api/v4/projects/2/labels":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"500"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":9901,"iid":99}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2, PathWithNamespace: "mpd/ss26/blatt-01/team1"}

	if _, err := client.replicateIssue(source, target, 7, false); err != nil {
		t.Fatalf("replicateIssue() must survive metadata failures, got error = %v", err)
	}

	if _, ok := createBody["assignee_ids"]; ok {
		t.Errorf("assignee_ids must be absent when the member lookup failed, got %v", createBody["assignee_ids"])
	}
	// The label name still goes along — GitLab binds an existing one and otherwise invents a
	// colour, which is strictly better than dropping the label.
	if createBody["labels"] != "bug" {
		t.Errorf("labels = %v, want \"bug\"", createBody["labels"])
	}
}

// If GitLab refuses the metadata, the issue must still be replicated. The mocked contract
// tests cannot prove GitLab accepts what we send, and the integration suite never reaches
// issue replication — so this fallback is what keeps the feature from being a regression.
func TestReplicateIssue_FallsBackToBareIssueWhenMetadataIsRejected(t *testing.T) {
	var bodies []map[string]any

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"t","description":"d",
				"state":"opened","labels":["bug"],"assignees":[{"id":11,"username":"student"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/2/labels":
			_, _ = w.Write([]byte(`[{"id":501,"name":"bug","color":"#ff0000"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/2/members/all":
			_, _ = w.Write([]byte(`[{"id":11,"username":"student"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			bodies = append(bodies, body)
			// Reject anything carrying metadata, accept the bare retry.
			if _, hasLabels := body["labels"]; hasLabels {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":9901,"iid":99}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2, PathWithNamespace: "mpd/ss26/blatt-01/team1"}

	iid, err := client.replicateIssue(source, target, 7, false)
	if err != nil {
		t.Fatalf("replicateIssue() must fall back to a bare issue, got error = %v", err)
	}
	if iid != 99 {
		t.Errorf("iid = %d, want 99", iid)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected two create attempts, got %d", len(bodies))
	}
	if _, hasLabels := bodies[1]["labels"]; hasLabels {
		t.Error("the retry must not carry labels")
	}
	if _, hasAssignees := bodies[1]["assignee_ids"]; hasAssignees {
		t.Error("the retry must not carry assignees")
	}
}

// A failure that has nothing to do with metadata must still surface as an error.
func TestReplicateIssue_BareCreateFailureStillFails(t *testing.T) {
	attempts := 0

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"t","description":"d",
				"state":"opened","labels":["bug"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/2/labels":
			_, _ = w.Write([]byte(`[{"id":501,"name":"bug","color":"#ff0000"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			attempts++
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2, PathWithNamespace: "mpd/ss26/blatt-01/team1"}

	if _, err := client.replicateIssue(source, target, 7, false); err == nil {
		t.Fatal("expected an error when the bare retry fails too")
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2 (metadata, then bare)", attempts)
	}
}
