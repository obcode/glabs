package obs

import (
	"fmt"
	"strings"
	"time"

	sentry "github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/rs/zerolog"
)

// flushTimeout bounds how long a shutdown, or a log.Fatal, waits for pending
// events to reach the backend. Long enough for a slow round trip, short enough
// that a dead monitoring host cannot hold up a restart.
const flushTimeout = 5 * time.Second

// Config is the error-reporting configuration. The DSN comes from the
// environment, everything else from the build info and .glabs-web.yaml.
type Config struct {
	// DSN is the ingest URL. Empty — the default, and the whole of local
	// development — disables reporting: Init then does nothing at all and
	// returns a nil writer.
	DSN string
	// Environment separates production from a test installation in the UI.
	Environment string
	// Release is the version this binary was built as, so an issue points at a
	// release rather than at "some time in the last three months".
	Release string
	// IgnoreErrors drops events whose message matches one of these patterns.
	// Shipped empty on purpose: filling it before a week of real traffic is
	// guessing at which noise exists.
	IgnoreErrors []string
	// Debug makes the SDK log what it does with every event.
	Debug bool

	// transport replaces the HTTP transport. Unexported, so only this package
	// can set it: the tests use it to run a real Init and read back what would
	// have gone out, which is the only way to check the whole chain
	// (writer → BeforeSend → transport) rather than the scrubber alone.
	transport sentry.Transport
}

// enabled reports whether Init actually brought up a client.
var enabled bool

// Enabled reports whether error reporting is configured.
func Enabled() bool { return enabled }

// Init starts error reporting and returns the zerolog writer that feeds it.
//
// The writer is the whole capture path: glabs handles its errors by logging
// them, so a log line at Error level or above IS the error report. The caller
// hangs it into the logger with zerolog.MultiLevelWriter; a nil return means
// "not configured" and the logger stays exactly as it was.
//
// Note what that means here: the writer sits on the GLOBAL zerolog logger, so it
// also captures the shared gitlab, config and reporter packages that glabs-web
// calls into. That is intended — a GitLab call failing is exactly what one wants
// to hear about — and it is why scrub.go's allow list is built around what those
// packages log, not around what web/ logs.
//
// One client, not two. sentryzerolog.New would build a second one from its own
// ClientOptions — with a second BeforeSend and its own buffer — so a Flush on
// either would miss half the events and the scrubber would have to be installed
// twice. NewWithHub binds the writer to the client sentry.Init just created.
func Init(cfg Config) (zerolog.LevelWriter, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, nil
	}

	scrub := scrubber{}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:          cfg.DSN,
		Environment:  cfg.Environment,
		Release:      cfg.Release,
		IgnoreErrors: cfg.IgnoreErrors,
		Debug:        cfg.Debug,
		BeforeSend:   scrub.scrub,
		Transport:    cfg.transport,

		// This server manages one faculty's lab assignments; there is no traffic
		// volume to trace and no budget question to answer with metrics. Both
		// off means less that can carry data out by accident.
		EnableTracing:  false,
		DisableLogs:    true,
		DisableMetrics: true,

		// Events from the writer get their (useless) stack from sentryzerolog.
		AttachStacktrace: false,

		// The writer adds none (WithBreadcrumbs is off below), so the only ones
		// that can exist are deliberate. A small ceiling keeps a long-running
		// request from carrying a day's worth into an unrelated failure.
		MaxBreadcrumbs: 20,

		// SendDefaultPII stays false, and DataCollection then says the same
		// thing in the SDK's newer, finer-grained vocabulary. Deprecated and set
		// anyway: the SDK's fallback for a NIL DataCollection is derived from
		// SendDefaultPII, so a future refactor that drops the block below would
		// otherwise silently turn collection on.
		//nolint:staticcheck // SA1019: kept as the safe fallback, see above
		SendDefaultPII: false,
		DataCollection: &sentry.DataCollection{
			UserInfo:    sentry.Set(false),
			HTTPBodies:  []sentry.BodyType{},
			Cookies:     &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff},
			QueryParams: &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff},
			HTTPHeaders: &sentry.HeaderCollectionConfig{
				Request: &sentry.KeyValueCollectionBehavior{
					Mode:  sentry.CollectionAllowList,
					Terms: allowedHeaderTerms(),
				},
				Response: &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot initialise error reporting: %w", err)
	}

	writer, err := sentryzerolog.NewWithHub(sentry.CurrentHub(), sentryzerolog.Options{
		// Warnings are ordinary weather here (a missing smtp host, a course
		// without members); reporting them would bury the errors.
		Levels: []zerolog.Level{
			zerolog.ErrorLevel,
			zerolog.FatalLevel,
			zerolog.PanicLevel,
		},
		// Would turn every Info line into a breadcrumb on the next error, each
		// one carrying its unfiltered fields.
		WithBreadcrumbs: false,
		FlushTimeout:    flushTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create the error-reporting log writer: %w", err)
	}

	enabled = true
	return writer, nil
}

// Flush waits for pending events. Always safe to call, including when reporting
// was never configured.
func Flush() {
	if enabled {
		sentry.Flush(flushTimeout)
	}
}
