package cmd

import (
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showInfoCmd)
}

var showInfoCmd = &cobra.Command{
	Use:   "show-info [group]",
	Short: "Show info for a group",
	Long:  `Show info for a group`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c := gitlab.NewClient()
		c.GetGroupInfo(args[0])
	},
}
