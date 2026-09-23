//go:generate go run github.com/99designs/gqlgen generate --verbose
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/obcode/glabs/v3/web/bootstrap"
	"github.com/obcode/glabs/v3/web/migrate"
	"github.com/spf13/viper"
)

// Build metadata, injected by goreleaser ldflags at release; otherwise filled
// from the VCS info below.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	// Europe/Berlin process-wide, so every naked time.Local — including the
	// Mongo driver's UseLocalTimeZone decoding — is Berlin time. Needs tzdata in
	// the container.
	if loc, err := time.LoadLocation("Europe/Berlin"); err == nil {
		time.Local = loc
	}

	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			var rev, vcsTime string
			var dirty bool
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					rev = s.Value
				case "vcs.time":
					vcsTime = s.Value
				case "vcs.modified":
					dirty = s.Value == "true"
				}
			}
			if rev != "" {
				if len(rev) > 12 {
					rev = rev[:12]
				}
				version = "dev-" + rev
				if vcsTime != "" {
					version += " (" + vcsTime
					if dirty {
						version += ", dirty"
					}
					version += ")"
				}
				commit = rev
				date = vcsTime
			}
		}
	}

	viper.Set("Version", version)
	viper.Set("Commit", commit)
	viper.Set("Date", date)
	viper.Set("BuiltBy", builtBy)

	// One subcommand, and a temporary one: the import from MongoDB. It is here
	// rather than in a tool of its own because the production host has Docker and
	// nothing else -- no Go toolchain, no psql -- so the image that is already
	// there is the only thing that can be run on it. Both this branch and the
	// package it calls go away with the MongoDB store.
	//
	// Hand-rolled rather than cobra: glabs-web has no other subcommand, and the
	// server must keep parsing its flags exactly as before.
	if len(os.Args) > 1 && os.Args[1] == "mongo2pg" {
		if err := migrate.Run(os.Args[2:], bootstrap.InitConfig, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		return
	}

	if err := bootstrap.Serve(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
