package db_test

import (
	"os"
	"testing"
	"time"
)

// TestMain pins time.Local to Europe/Berlin for the whole db test binary, which
// is what cmd/glabs-web/main.go does for the running server.
//
// Without it the timezone part of the contract suite asserts a configuration
// that never exists in production, and the result depends on the host: the dev
// container is on Europe/Berlin and it passes, a GitHub runner is on UTC and it
// fails. Setting it here rather than exporting TZ in the workflow keeps the
// suite correct on any machine.
func TestMain(m *testing.M) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic("cannot load Europe/Berlin: " + err.Error())
	}
	time.Local = loc

	os.Exit(m.Run())
}
