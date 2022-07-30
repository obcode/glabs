package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v3"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(setaccessCmd)
}

var setaccessCmd = &cobra.Command{
	Use:   "setaccess course assignment [groups...|students...]",
	Short: "Set access level for exisiting repositories.",
	Long:  `Set access level for exisiting repositories.`,
	Args:  cobra.MinimumNArgs(2), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
		assignmentConfig.Show()
		fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
		fmt.Scanln()
		c := gitlab.NewClient()
		c.Setaccess(assignmentConfig)
	},
}
