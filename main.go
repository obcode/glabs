package main

import (
	"github.com/obcode/glabs/cmd"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	viper.Set("Version", version)
	viper.Set("Commit", commit)
	viper.Set("Date", date)
	viper.Set("BuiltBy", builtBy)
	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
