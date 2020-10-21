package cmd

import (
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check [course]",
	Short: "check course config",
	Long:  `Check config of a course, especially if all student names exist`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("show-success", true)
		c := gitlab.NewClient()
		if len(args) == 1 {
			cfg := config.GetCourseConfig(args[0])
			if cfg != nil {
				c.CheckCourse(cfg)
			}
		}
	},
}
