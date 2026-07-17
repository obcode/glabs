package cmd

import (
	"fmt"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/gitlab"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addgroupguestsCmd)
}

var addgroupguestsCmd = &cobra.Command{
	Use:   "addgroupguests course",
	Short: "Add all students as guests to the course subgroup",
	Long: `Add all students from the course configuration (both individual students and group members) 
as guests to the course subgroup (coursepath/semesterpath). This allows students to use the 
Dependency-Proxy and other group-level features.

The access level is set to 'guest' by default, which is the minimum permission needed for 
accessing the Dependency-Proxy.

Newly created memberships/invitations expire automatically after 1 year.

Example:
	glabs addgroupguests vss    # Adds all students from vss course to the vss/semester/ob-26ss subgroup`,
	Args: cobra.ExactArgs(1), //nolint:gomnd
	Run: func(cmd *cobra.Command, args []string) {
		courseName := args[0]
		if !config.CourseExists(courseName) {
			fmt.Printf("Error: course '%s' not found in configuration\n", courseName)
			return
		}

		courseConfig, err := config.GetCourseConfig(courseName)
		if err != nil {
			er(err)
		}
		subgroupPath := config.GetCourseSubgroupPath(courseName)

		fmt.Printf("Course: %s\n", courseName)
		fmt.Printf("Subgroup: %s\n", subgroupPath)
		fmt.Printf("Individual students: %d\n", len(courseConfig.Students))
		fmt.Printf("Groups: %d\n", len(courseConfig.Groups))

		// Count unique students (deduplicating across groups)
		studentSet := make(map[string]struct{})
		for _, student := range courseConfig.Students {
			key := config.StudentKey(student)
			if key != "" {
				studentSet[key] = struct{}{}
			}
		}
		for _, group := range courseConfig.Groups {
			for _, member := range group.Members {
				key := config.StudentKey(member)
				if key != "" {
					studentSet[key] = struct{}{}
				}
			}
		}
		fmt.Printf("Unique students to add: %d\n", len(studentSet))
		fmt.Println(aurora.Magenta("Add these students as guests to the subgroup? Press 'Enter' to continue or 'Ctrl-C' to stop ..."))
		fmt.Scanln() //nolint:errcheck

		c, err := gitlab.NewClientFromViper()
		if err != nil {
			er(err)
		}
		err = c.AddGroupGuests(courseName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	},
}
