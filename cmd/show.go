package cmd

import (
	"fmt"

	"github.com/obcode/glabs/v3/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showConfigCmd)
}

var showConfigCmd = &cobra.Command{
	Use:   "show course assignment [groups...|students...]",
	Short: "Show config of an assignment",
	Long:  `Show config of an assignment`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
		if err != nil {
			er(err)
		}
		fmt.Println(cfg.Show())
	},
}
