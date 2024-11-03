package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update course assignment [groups...|students...]",
	Short: "Update repositories with code.",
	Long: `Update repositories with code from the startercode repo.
	USE WITH CARE!
	This can result in merge conflicts, which cannot be handled
	Use only with fresh, i.e. untouched, repositories.`,
	Args: cobra.MinimumNArgs(2), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
		assignmentConfig.Show()
		fmt.Println(aurora.Red("Heads up! Use only with untouched projects."))
		fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
		fmt.Scanln() //nolint:errcheck
		c := gitlab.NewClient()
		c.Update(assignmentConfig)
	},
}
