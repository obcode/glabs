package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/obcode/glabs/v2/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	Verbose bool
	rootCmd = &cobra.Command{
		Use:   "glabs",
		Short: "Manage GitLab for student assignments",
		Long:  `Manage GitLab for student assignments`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

			output := zerolog.ConsoleWriter{Out: os.Stdout}
			if Verbose {
				output.FormatLevel = func(i interface{}) string {
					return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
				}
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			} else {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			}
			log.Logger = zerolog.New(output).With().Caller().Timestamp().Logger()
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.glabs.yml)")
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose output")
}

func er(msg interface{}) {
	fmt.Println("Error:", msg)
	os.Exit(1)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			er(err)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".glabs")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		er(fmt.Errorf("cannot read the main config file: %w", err))
	}

	// Course files are loaded into config's own registry rather than merged into
	// viper. Merging them put course names in the same flat namespace as
	// gitlab.host and coursesfilepath — a course named `courses` would have
	// collided with the course list — and left resolution reading, and writing,
	// global state.
	for _, course := range viper.GetStringSlice("courses") {
		path, err := findCourseFile(course)
		if err != nil {
			er(err)
		}
		if _, err := config.LoadCourseFile(path); err != nil {
			er(err)
		}
	}
}

// findCourseFile locates a course file by name under coursesfilepath, accepting
// the extensions viper used to search for.
func findCourseFile(course string) (string, error) {
	dir := viper.GetString("coursesfilepath")
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, course+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("cannot find a course file for %q: looked for %s.yaml and %s.yml in %q",
		course, course, course, dir)
}
