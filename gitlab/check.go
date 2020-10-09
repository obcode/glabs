package gitlab

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

func (c *Client) Check(group string) bool {
	noOfErrors := 0
	header(fmt.Sprintf("> checking usernames config of %s\n", group))
	header(fmt.Sprintln(">> checking students"))

	for _, student := range viper.GetStringSlice(group + ".students") {
		if !c.checkStudent(student) {
			noOfErrors++
		}
	}

	header(fmt.Sprintln(">> checking students in groups"))

	for subgroup, members := range viper.GetStringMapStringSlice(group + ".groups") {
		header(fmt.Sprintf(">>> checking %s\n", subgroup))
		for _, student := range members {
			if !c.checkStudent(student) {
				noOfErrors++
			}
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
	} else {
		success(fmt.Sprintf("   > %s exists\n", name))
		return true
	}
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
