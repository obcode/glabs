package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Glabs",
	Long:  `All software has versions. This is Glabs'`,
	Run: func(cmd *cobra.Command, args []string) {
		if viper.GetString("Commit") == "none" {
			fmt.Printf("Glabs version %s\n", viper.GetString("Version"))
		} else {
			fmt.Printf("Glabs version %s, commit %s, build date %s, build by %s\n",
				viper.GetString("Version"),
				viper.GetString("Commit"),
				viper.GetString("Date"),
				viper.GetString("BuiltBy"),
			)
		}
	},
}
