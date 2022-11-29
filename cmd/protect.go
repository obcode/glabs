package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(protectCmd)
	protectCmd.Flags().StringVarP(&ProtectBranch, "branch", "b", "", "protect branch (overrides config file)")
}

var (
	protectCmd = &cobra.Command{
		Use:   "protect course assignment [groups...|students...]",
		Short: "Protect branch for exisiting repositories.",
		Long:  `Protect branch for exisiting repositories.`,
		Args:  cobra.MinimumNArgs(2), //nolint:gomnd
		Run: func(cmd *cobra.Command, args []string) {
			assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
			if len(ProtectBranch) > 0 {
				assignmentConfig.SetProtectToBranch(ProtectBranch)
			}
			assignmentConfig.Show()
			fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
			fmt.Scanln()
			c := gitlab.NewClient()
			c.ProtectToBranch(assignmentConfig)
		},
	}
	ProtectBranch string
)
