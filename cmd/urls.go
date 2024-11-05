package cmd

import (
	"github.com/obcode/glabs/config"
	"github.com/spf13/cobra"
)

var (
	urlsCmd = &cobra.Command{
		Use:   "urls course assignment [groups...|students...]",
		Short: "get urls for repositories",
		Long: `get urls for repositories for each student or group in course for assignment.
		You can specify students or groups in order to get an url only for these.`,
		Args: cobra.MinimumNArgs(2), //nolint:gomnd
		Run: func(cmd *cobra.Command, args []string) {
			assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
			assignmentConfig.Urls(len(args) == 2)
		},
	}
)

func init() {
	rootCmd.AddCommand(urlsCmd)
}
