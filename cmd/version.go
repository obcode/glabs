package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.0"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Glabs",
	Long:  `All software has versions. This is Glabs'`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Glabs version %s\n", version)
	},
}
