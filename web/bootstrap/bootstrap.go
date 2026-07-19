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
	"github.com/obcode/glabs/v3/web/secrets"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	dbURI   string
	verbose bool
)

// Serve parses flags, loads config, connects to MongoDB and runs the server.
func Serve() error {
	flag.StringVar(&dbURI, "db-uri", "", "override db.uri from the config file")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.BoolVar(&verbose, "v", false, "verbose output (shorthand)")
	flag.Parse()

	setupLogging()

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
	if err := database_.EnsureUserIndexes(ctx); err != nil {
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

	// The KEK for per-user secrets (GitLab PATs). It lives only in the config, never
	// in the database. A malformed key disables token storage (fail-closed); an
	// empty key leaves the sealer nil, so the config editor still works and only
	// token operations are unavailable.
	sealer, err := secrets.NewSealer(viper.GetString("secrets.key"))
	if err != nil {
		log.Error().Err(err).Msg("invalid secrets.key — storing GitLab tokens is disabled until it is fixed")
	}

	a := app.New(database_, sealer, viper.GetString("gitlab.host"))
	if err := seedUsers(ctx, database_); err != nil {
		return err
	}

	graph.StartServer(a, viper.GetString("server.port"))
	return nil
}

func setupLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	if verbose {
		output.FormatLevel = func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
		}
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	log.Logger = zerolog.New(output).With().Caller().Timestamp().Logger()
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
