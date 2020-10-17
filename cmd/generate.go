package cmd

import (
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate [course] [assignment]",
	Short: "Generate repositories for each student.",
	Long:  `Generate repositories for each student in [course] for [assignment]`,
	Args:  cobra.ExactArgs(2), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		c := gitlab.NewClient()
		c.Generate(args[0], args[1])
	},
}
