package gitlab

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

func (c *Client) Check(course string) bool {
	if !viper.IsSet(course) {
		log.Error().
			Str("course", course).
			Msg("configuration for course does not exist")
		return false
	}

	noOfErrors := 0
	header(fmt.Sprintf("> checking usernames config of %s\n", course))
	header(fmt.Sprintln(">> checking students"))

	for _, student := range viper.GetStringSlice(course + ".students") {
		if !c.checkStudent(student) {
			noOfErrors++
		}
	}

	header(fmt.Sprintln(">> checking students in groups"))

	groups := viper.GetStringMapStringSlice(course + ".groups")

	for subgroup, members := range groups {
		header(fmt.Sprintf(">>> checking %s\n", subgroup))
		for _, student := range members {
			if !c.checkStudent(student) {
				noOfErrors++
			}
		}
	}

	header(fmt.Sprintln(">>> checking duplicates in groups"))
	if studsInMoreGroups := checkDupsInGroups(groups); len(studsInMoreGroups) > 0 {
		for student, inGroups := range studsInMoreGroups {
			failure(fmt.Sprintf("   > %s is in more than one group: %v\n", student, inGroups))
			noOfErrors++
		}
	}

	if noOfErrors > 0 {
		failure(fmt.Sprintf("===> %d error(s)\n", noOfErrors))
		return false
	}
	return true
}

func (c *Client) checkStudent(name string) bool {
	if _, err := c.getUserID(name); err != nil {
		failure(fmt.Sprintf("   > %s, error: %v\n", name, err))
		return false
	}
	success(fmt.Sprintf("   > %s exists\n", name))
	return true
}

func checkDupsInGroups(groups map[string][]string) map[string][]string {
	studsWithGroups := make(map[string][]string)
	for subgroup, members := range groups {
		for _, student := range members {
			_, ok := studsWithGroups[student]
			if !ok {
				studsWithGroups[student] = []string{subgroup}
			} else {
				studsWithGroups[student] = append(studsWithGroups[student], subgroup)
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
