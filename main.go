package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/obcode/glabs/v2/cmd"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			version = info.Main.Version
		}
	}
	viper.Set("Version", version)
	viper.Set("Commit", commit)
	viper.Set("Date", date)
	viper.Set("BuiltBy", builtBy)
	err := cmd.Execute()
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error:", err)
		if err != nil {
			panic(err)
		}
		os.Exit(-1)
	}
}
