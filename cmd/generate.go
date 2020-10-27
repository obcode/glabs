package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v3"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate course assignment [groups...|students...]",
	Short: "Generate repositories.",
	Long: `Generate repositories for each student or group in course for assignment.
You can specify students or groups in order to generate only for these.
A student needs to exist on GitLab, a group needs to exist in the configuration file.`,
	Args: cobra.MinimumNArgs(2), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
		assignmentConfig.Show()
		fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
		fmt.Scanln()
		c := gitlab.NewClient()
		c.Generate(assignmentConfig)
	},
}
