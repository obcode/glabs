package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.Flags().BoolVarP(&unarchive, "unarchive", "u", false, "unarchive project")
}

var (
	archiveCmd = &cobra.Command{
		Use:   "archive course assignment [groups...|students...]",
		Short: "Archive or unarchive repositories.",
		Long:  `Archive or unarchive repositories.`,
		Args:  cobra.MinimumNArgs(2), //nolint:gomnd
		Run: func(cmd *cobra.Command, args []string) {
			assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
			assignmentConfig.Show()
			fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
			fmt.Scanln() //nolint:errcheck
			c := gitlab.NewClient()
			c.Archive(assignmentConfig, unarchive)
		},
	}
	unarchive bool
)
