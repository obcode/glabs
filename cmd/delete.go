package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(deleteCmd)
}

var deleteCmd = &cobra.Command{
	Use:   "delete course assignment [groups...|students...]",
	Short: "Delete repositories.",
	Long: `Delete repositories for each student or group in course for assignment.
You can specify students or groups in order to delete only for these.`,
	Args: cobra.MinimumNArgs(2), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
		assignmentConfig.Show()
		fmt.Println(aurora.Magenta("Do you really want to delete the repos? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
		fmt.Scanln()
		c := gitlab.NewClient()
		c.Delete(assignmentConfig)
	},
}
