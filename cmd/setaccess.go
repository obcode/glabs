package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/gitlab"
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
			assignmentConfig, err := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
			if err != nil {
				er(err)
			}
			if len(Level) > 0 {
				assignmentConfig.SetAccessLevel(Level)
			}
			fmt.Println(assignmentConfig.Show())
			fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
			fmt.Scanln() //nolint:errcheck
			c, err := gitlab.NewClientFromViper()
			if err != nil {
				er(err)
			}
			c.Setaccess(assignmentConfig)
		},
	}
	Level string
)
