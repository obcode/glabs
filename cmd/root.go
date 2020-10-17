package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
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

	if err := viper.ReadInConfig(); err == nil {
		viper.AddConfigPath(viper.GetString("coursesfilepath"))
		for _, course := range viper.GetStringSlice("courses") {
			viper.SetConfigName(course)
			err = viper.MergeInConfig()
			if err != nil {
				panic(fmt.Errorf("%s: should be %s.yml", err, course))
			}
		}
	} else {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}
}
