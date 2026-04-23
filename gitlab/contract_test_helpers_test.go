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

	// Use a plain http.Client without the retryable wrapper so that 5xx
	// responses from mock servers do not trigger retries and cause test hangs.
	apiClient, err := gitlabapi.NewClient("token",
		gitlabapi.WithBaseURL(server.URL+"/api/v4"),
		gitlabapi.WithHTTPClient(&http.Client{}),
		gitlabapi.WithoutRetries(),
	)
	if err != nil {
		t.Fatalf("creating gitlab test client failed: %v", err)
	}

	return &Client{apiClient}
}
