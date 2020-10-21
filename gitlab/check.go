package gitlab

import (
	"fmt"

	"github.com/obcode/glabs/config"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

func (c *Client) CheckCourse(cfg *config.CourseConfig) bool {
	noOfErrors := 0
	header(fmt.Sprintf("%s:\n", cfg.Course))

	if len(cfg.Students) > 0 {
		header(fmt.Sprintln("  - students:"))

		for _, student := range cfg.Students {
			if !c.checkStudent(student, "") {
				noOfErrors++
			}
		}
	}

	if len(cfg.Groups) > 0 {
		header(fmt.Sprintln("  - groups:"))

		for _, grp := range cfg.Groups {
			header(fmt.Sprintf("    - %s:\n", grp.GroupName))
			for _, student := range grp.Members {
				if !c.checkStudent(student, "  ") {
					noOfErrors++
				}
			}
		}

		header("  # checking duplicates in groups\n")
		foundDup := false

		if studsInMoreGroups := checkDupsInGroups(cfg.Groups); len(studsInMoreGroups) > 0 {

			for student, inGroups := range studsInMoreGroups {
				failure(fmt.Sprintf("   # %s is in more than one group: %v\n", student, inGroups))
				foundDup = true
				noOfErrors++
			}

		}

		if !foundDup {
			header("    # no duplicate found\n")
		}
	}

	if noOfErrors > 0 {
		failure(fmt.Sprintf("# ===> %d error(s)\n", noOfErrors))
		return false
	}
	return true
}

func (c *Client) checkStudent(name, prefix string) bool {
	user, err := c.getUser(name)
	if err != nil {
		failure(fmt.Sprintf("    # %s, error: %v\n", name, err))
		return false
	}
	success(fmt.Sprintf("%s     - %s # %s\n", prefix, user.Username, user.Name))
	return true
}

func checkDupsInGroups(groups []*config.Group) map[string][]string {
	studsWithGroups := make(map[string][]string)
	for _, grp := range groups {
		for _, student := range grp.Members {
			_, ok := studsWithGroups[student]
			if !ok {
				studsWithGroups[student] = []string{grp.GroupName}
			} else {
				studsWithGroups[student] = append(studsWithGroups[student], grp.GroupName)
			}
		}
	}

	problems := make(map[string][]string)
	for student, inGroups := range studsWithGroups {
		if len(inGroups) > 1 {
			problems[student] = inGroups
		}
	}

	return problems
}

func header(str string) {
	if viper.GetBool("show-success") {
		fmt.Print(chalk.Blue, chalk.Bold, str, chalk.Reset)
	}
}

func success(str string) {
	if viper.GetBool("show-success") {
		fmt.Print(chalk.Green, str, chalk.Reset)
	}
}

func failure(str string) {
	fmt.Print(chalk.Red, str, chalk.Reset)
}
