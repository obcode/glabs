package cmd

import (
	"github.com/obcode/glabs/v2/config"
	"github.com/spf13/cobra"
)

var (
	urlsCmd = &cobra.Command{
		Use:   "urls course assignment [groups...|students...]",
		Short: "get urls for repositories",
		Long: `get urls for repositories for each student or group in course for assignment.
		You can specify students or groups in order to get an url only for these.`,
		Args: cobra.MinimumNArgs(1), //nolint:gomnd
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 1 {
				config.GetCourseURL(args[0])
			} else {
				assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
				if startercode {
					assignmentConfig.StartercodeURL()
				} else {
					assignmentConfig.Urls(len(args) == 2)
				}
			}
		},
	}
	startercode bool
)

func init() {
	rootCmd.AddCommand(urlsCmd)
	urlsCmd.Flags().BoolVarP(&startercode, "startercode", "s", false, "print url of startercode")

}
