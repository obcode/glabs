package gitlab

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

func newContractClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	apiClient, err := gitlabapi.NewClient("token", gitlabapi.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("creating gitlab test client failed: %v", err)
	}

	return &Client{apiClient}
}
