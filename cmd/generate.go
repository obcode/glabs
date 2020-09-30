package cmd

import (
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
)

var (
	Startercode string
)

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&Startercode, "startercode", "c", "",
		"repo which should be used as starter code template")
}

var generateCmd = &cobra.Command{
	Use:   "generate [group] [assignment]",
	Short: "Generate repositories for each student.",
	Long:  `Generate repositories for each student in [group] for [assignment]`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		c := gitlab.NewClient()
		c.Generate(args[0], args[1], "")
	},
}
