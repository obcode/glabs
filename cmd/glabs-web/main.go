//go:generate go run github.com/99designs/gqlgen generate --verbose
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/obcode/glabs/v3/web/bootstrap"
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

	if err := bootstrap.Serve(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
