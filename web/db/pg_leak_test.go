package db_test

import (
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/web/db"
)

// TestConnectionErrorsDoNotLeakThePassword pins that a database password never
// reaches a log line.
//
// These messages are printed on every restart of a crash loop, and a container
// that cannot reach its database restarts forever. `docker compose logs` is not
// a secret store.
func TestConnectionErrorsDoNotLeakThePassword(t *testing.T) {
	const password = "s3cr3t-do-not-print"

	for _, tt := range []struct {
		name string
		uri  string
	}{
		{"unparseable", "postgres://glabs:" + password + "@host:notaport/glabs"},
		{"unreachable", "postgres://glabs:" + password + "@127.0.0.1:1/glabs?sslmode=disable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.NewPG(t.Context(), tt.uri)
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), password) {
				t.Errorf("the password is in the error message:\n%s", err)
			}
		})
	}
}
