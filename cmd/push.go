package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/v2/config"
	"github.com/obcode/glabs/v2/gitlab"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push course assignment branch [groups...|students...]",
	Short: "Push one deferred branch to student/group repos",
	Long: `Push one deferred branch to student/group repos.
	You can specify students or groups in order to push only for these.
	You cannot push all deferred branches at once.`,
	Args: cobra.MinimumNArgs(3), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		course := args[0]
		assignment := args[1]
		branchname := args[2]
		assignmentConfig := config.GetAssignmentConfig(course, assignment, args[3:]...)
		assignmentConfig.Show()
		fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
		fmt.Scanln() //nolint:errcheck
		c := gitlab.NewClient()
		err := c.Push(assignmentConfig, branchname)
		if err != nil {
			fmt.Printf("error: %s", err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
