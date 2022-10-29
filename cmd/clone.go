package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/git"
	"github.com/spf13/cobra"
)

var (
	cloneCmd = &cobra.Command{
		Use:   "clone course assignment [groups...|students...]",
		Short: "Clone repositories.",
		Long: `Clone repositories for each student or group in course for assignment.
		You can specify students or groups in order to clone only for these.`,
		Args: cobra.MinimumNArgs(2), //nolint:gomnd
		Run: func(cmd *cobra.Command, args []string) {
			assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
			if len(Branch) > 0 {
				assignmentConfig.SetBranch(Branch)
			}
			if len(Localpath) > 0 {
				assignmentConfig.SetLocalpath(Localpath)
			}
			if Force {
				assignmentConfig.SetForce()
			}
			assignmentConfig.Show()
			fmt.Println(aurora.Magenta("Config okay? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
			fmt.Scanln()

			git.Clone(assignmentConfig)
		},
	}
	Localpath string
	Branch    string
	Force     bool
)

func init() {
	rootCmd.AddCommand(cloneCmd)
	cloneCmd.Flags().StringVarP(&Localpath, "path", "p", "", "clone in this directory")
	cloneCmd.Flags().StringVarP(&Branch, "branch", "b", "", "checkout branch after cloning")
	cloneCmd.Flags().BoolVarP(&Force, "force", "f", false, "remove directory if it already exists")
}
