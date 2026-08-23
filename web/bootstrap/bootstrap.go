// Package bootstrap wires glabs-web together: flags, config, database, then the
// GraphQL server. It is the server's equivalent of cmd/root.go for the CLI.
package bootstrap

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/obcode/glabs/v3/web/app"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph"
	"github.com/obcode/glabs/v3/web/mail"
	"github.com/obcode/glabs/v3/web/obs"
	"github.com/obcode/glabs/v3/web/secrets"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	dbURI   string
	verbose bool
)

// The error-reporting configuration comes from the environment, not from
// .glabs-web.yaml: it has to be up before the config file is read, since a
// config file that will not load is exactly the kind of startup failure worth
// hearing about. The DSN is a credential and belongs in the host's .env.
const (
	EnvSentryDSN         = "SENTRY_DSN"
	EnvSentryEnvironment = "SENTRY_ENVIRONMENT"
)

// Serve parses flags, loads config, connects to MongoDB and runs the server.
func Serve() error {
	flag.StringVar(&dbURI, "db-uri", "", "override db.uri from the config file")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.BoolVar(&verbose, "v", false, "verbose output (shorthand)")
	flag.Parse()

	reporter, flushReports := setupReporting()
	defer flushReports()

	setupLogging(reporter)

	if err := initConfig(); err != nil {
		return err
	}

	uri := viper.GetString("db.uri")
	if dbURI != "" {
		uri = dbURI
	}
	database := viper.GetString("db.database")
	if database == "" {
		database = "glabs"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	database_, err := db.Connect(ctx, uri, database)
	if err != nil {
		return err
	}
	if err := database_.EnsureCourseIndexes(ctx); err != nil {
		return err
	}
	if err := database_.EnsureUserSecretIndexes(ctx); err != nil {
		return err
	}
	if err := database_.EnsureActivityIndexes(ctx); err != nil {
		return err
	}
	if err := database_.EnsureJobIndexes(ctx); err != nil {
		return err
	}
	if err := database_.EnsureEventIndexes(ctx); err != nil {
		return err
	}

	// The KEK for per-user secrets (GitLab PATs). It lives only in the config, never
	// in the database. A malformed key disables token storage (fail-closed); an
	// empty key leaves the sealer nil, so the config editor still works and only
	// token operations are unavailable.
	sealer, err := secrets.NewSealer(viper.GetString("secrets.key"))
	if err != nil {
		log.Error().Err(err).Msg("invalid secrets.key — storing GitLab tokens is disabled until it is fixed")
	}

	// SMTP for job notifications is optional: without smtp.host, scheduling and
	// running still work — only the emails are skipped.
	var mailer app.Mailer
	if smtpHost := viper.GetString("smtp.host"); smtpHost != "" {
		mailer = mail.NewSender(mail.Config{
			Host:                  smtpHost,
			Port:                  viper.GetInt("smtp.port"),
			Username:              viper.GetString("smtp.username"),
			Password:              viper.GetString("smtp.password"),
			From:                  viper.GetString("smtp.from"),
			Hostname:              viper.GetString("smtp.hostname"),
			TLSInsecureSkipVerify: viper.GetBool("smtp.tlsInsecureSkipVerify"),
			TestRecipient:         viper.GetString("smtp.testRecipient"),
		})
		log.Info().Str("host", smtpHost).Bool("dryRun", viper.GetBool("smtp.dryRun")).Msg("SMTP configured; job notifications enabled")
	} else {
		log.Warn().Msg("no smtp.host configured; scheduled-job notifications are disabled")
	}

	// admins may see the platform-wide monitoring page and receive the nightly
	// summary. It is the ONLY privilege above ordinary owner-scoped access.
	admins := viper.GetStringSlice("admins")

	a := app.New(database_, sealer, viper.GetString("gitlab.host"), mailer, viper.GetBool("smtp.dryRun"), admins)

	// The scheduled-job runner polls in the background for the life of the process
	// (a background context, since StartServer blocks and never returns here).
	go a.StartJobRunner(context.Background())

	// The nightly admin summary is optional: it needs a mailer, at least one
	// recipient, and summary.enabled. Recipients default to the admins list.
	if viper.GetBool("summary.enabled") {
		recipients := []string{}
		if r := strings.TrimSpace(viper.GetString("summary.recipient")); r != "" {
			recipients = append(recipients, r)
		} else {
			recipients = admins
		}
		hour := 5
		if viper.IsSet("summary.hour") {
			hour = viper.GetInt("summary.hour")
		}
		a.ConfigureSummary(hour, recipients)
		go a.StartSummaryMailer(context.Background())
	} else {
		log.Info().Msg("nightly admin summary disabled (summary.enabled not set)")
	}

	graph.StartServer(a, viper.GetString("server.port"))
	return nil
}

// setupLogging configures the global logger, and hangs the error reporter into
// it when there is one.
//
// The reporter has to be carried over from setupReporting: this replaces the
// writer, and without it every error from here on would only be printed.
func setupLogging(reporter zerolog.LevelWriter) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	// Not cosmetic: the reporter groups issues by the caller field, and the
	// compiler'"'"'s absolute path would make the same line read differently in the
	// container than it does here. See web/obs/caller.go.
	zerolog.CallerMarshalFunc = obs.RepoRelativeCaller

	output := zerolog.ConsoleWriter{Out: os.Stdout}
	if verbose {
		output.FormatLevel = func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
		}
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	var out zerolog.LevelWriter = zerolog.MultiLevelWriter(output)
	if reporter != nil {
		out = zerolog.MultiLevelWriter(output, reporter)
	}
	log.Logger = zerolog.New(out).With().Caller().Timestamp().Logger()
}

// setupReporting starts error reporting from the environment and returns the
// writer for setupLogging to keep, plus a flush to defer.
//
// Everything that decides what may leave this host lives in web/obs — read
// web/obs/scrub.go before adding anything that reports. Note that the writer
// sits on the GLOBAL logger, so it also captures the shared gitlab, config and
// reporter packages this server calls into; that is what the allow list there is
// built around. Both returns are safe when reporting is off: the writer is nil
// and the flush does nothing.
//
// A collector that will not start is a reason to run unmonitored, not a reason
// to refuse to serve.
func setupReporting() (zerolog.LevelWriter, func()) {
	dsn := os.Getenv(EnvSentryDSN)
	if dsn == "" {
		return nil, func() {}
	}

	environment := os.Getenv(EnvSentryEnvironment)
	if environment == "" {
		environment = "production"
	}

	reporter, err := obs.Init(obs.Config{
		DSN:         dsn,
		Environment: environment,
		// Set by cmd/glabs-web before Serve; viper.Set outranks the config file,
		// which has not been read yet anyway.
		Release: viper.GetString("Version"),
		// Deliberately empty: filling the ignore list before a week of real
		// traffic is guessing at which noise exists.
		IgnoreErrors: nil,
	})
	if err != nil {
		log.Warn().Err(err).Msg("error reporting is off")
		return nil, func() {}
	}
	if reporter == nil {
		return nil, func() {}
	}

	// Attach it straight away, before setupLogging runs: the one log.Fatal that
	// can happen in between is a config file that will not load.
	log.Logger = log.Output(zerolog.MultiLevelWriter(os.Stderr, reporter)).
		With().Caller().Logger()

	return reporter, obs.Flush
}

func initConfig() error {
	home, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	viper.SetConfigName(".glabs-web")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath(home)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return fmt.Errorf("config '.glabs-web.yaml' not found (searched in: ., %s)", home)
		}
		return fmt.Errorf("cannot read config '.glabs-web.yaml': %w", err)
	}
	return nil
}
