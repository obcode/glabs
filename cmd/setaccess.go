package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(setaccessCmd)
	setaccessCmd.Flags().StringVarP(&Level, "level", "l", "", "accesslevel (overrides config file)")
}

var (
	setaccessCmd = &cobra.Command{
		Use:   "setaccess course assignment [groups...|students...]",
		Short: "Set access level for exisiting repositories.",
		Long:  `Set access level for exisiting repositories.`,
		Args:  cobra.MinimumNArgs(2), //nolint:gomnd
		Run: func(cmd *cobra.Command, args []string) {
			assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
			if len(Level) > 0 {
				assignmentConfig.SetAccessLevel(Level)
			}
			assignmentConfig.Show()
			fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
			fmt.Scanln() //nolint:errcheck
			c := gitlab.NewClient()
			c.Setaccess(assignmentConfig)
		},
	}
	Level string
)
