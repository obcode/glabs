package obs

import (
	"errors"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

// TestLiveIngest sends real events to a real collector. Skipped unless
// SENTRY_SMOKE_DSN is set, so it never runs in CI or on a laptop by accident:
//
//	SENTRY_SMOKE_DSN='https://<key>@glitchtip.example.edu/5' \
//	    go test ./web/obs/ -run TestLiveIngest -v
//
// Expected in the UI afterwards: THREE events in TWO issues — the two lines that
// share a call site share an issue — and on every one of them a `course` tag but
// NO `project` and NO `student` tag, and [email] in place of both the ordinary
// address and the _at_ spelling glabs generates. That is the scrubber checked
// against the deployment rather than against a unit test's idea of it.
func TestLiveIngest(t *testing.T) {
	dsn := os.Getenv("SENTRY_SMOKE_DSN")
	if dsn == "" {
		t.Skip("SENTRY_SMOKE_DSN not set")
	}

	writer, err := Init(Config{DSN: dsn, Environment: "smoketest", Release: "smoketest"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if writer == nil {
		t.Fatal("Init returned no writer for a non-empty DSN")
	}
	defer Flush()

	// Same as bootstrap does — without it the caller, and therefore the fingerprint,
	// would carry this machine's absolute path.
	zerolog.CallerMarshalFunc = RepoRelativeCaller

	logger := zerolog.New(zerolog.MultiLevelWriter(zerolog.NewTestWriter(t), writer)).
		With().Caller().Timestamp().Logger()

	logger.Info().Msg("must not be reported")
	for _, assignment := range []string{"blatt01", "blatt02"} {
		logger.Error().
			Str("course", "mpd").
			Str("job", "smoke-"+assignment).
			Str("student", "alice"). // must be scrubbed away: identifies a person
			Str("project", "mpd-"+assignment+"-alice_at_example.org").
			Msg("cannot archive the project")
	}
	logger.Error().
		Err(errors.New("404 Project Not Found: mpd-blatt01-alice_at_example.org, owner oliver.braun@hm.edu")).
		Msg("cannot reach GitLab")

	t.Log("sent; expect 3 events in 2 issues, no student/project tag, [email] in the text")
}
