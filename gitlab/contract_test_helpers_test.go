package gitlab

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newContractClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	// Disable retries so a mocked 5xx returns immediately instead of retrying
	// with backoff. This goes through the real constructor now — driving it with
	// injected options is exactly why the DI exists.
	client, err := NewClient(
		WithHost(server.URL+"/api/v4"),
		WithToken("token"),
		WithHTTPClient(&http.Client{}),
		WithoutRetries(),
	)
	if err != nil {
		t.Fatalf("creating gitlab test client failed: %v", err)
	}

	return client
}
