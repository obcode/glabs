package cmd

import (
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check [group]",
	Short: "check group config",
	Long:  `Check config of a group, especially if all student names exist`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("show-success", true)
		c := gitlab.NewClient()
		c.Check(args[0])
	},
}
