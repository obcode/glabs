package gitlab

import (
	"strings"

	"github.com/gookit/color"
	"github.com/obcode/glabs/config"
)

func (c *Client) CheckCourse(cfg *config.CourseConfig) bool {
	noOfErrors := 0
	color.Cyan.Printf("%s:\n", cfg.Course)

	if len(cfg.Students) > 0 {
		color.Cyan.Println("  - students:")

		for _, student := range cfg.Students {
			if !c.checkStudent(student, "") {
				noOfErrors++
			}
		}
	}

	if len(cfg.Groups) > 0 {
		color.Cyan.Println("  - groups:")

		for _, grp := range cfg.Groups {
			color.Cyan.Printf("    - %s:\n", grp.Name)
			for _, student := range grp.Members {
				if !c.checkStudent(student, "  ") {
					noOfErrors++
				}
			}
		}

		color.Cyan.Print("  # checking duplicates in groups")
		foundDup := false

		if studsInMoreGroups := checkDupsInGroups(cfg.Groups); len(studsInMoreGroups) > 0 {

			for student, inGroups := range studsInMoreGroups {
				color.Red.Printf("\n  # %s is in more than one group: %v", student, inGroups)
				foundDup = true
				noOfErrors++
			}

		}

		if !foundDup {
			color.Green.Println("... no duplicate found")
		}
	}

	if noOfErrors > 0 {
		color.Red.Printf("\n# ===> %d error", noOfErrors)
		if noOfErrors == 1 {
			color.Red.Println()
		} else {
			color.Red.Println("s")
		}
		return false
	}
	return true
}

func (c *Client) checkStudent(searchPattern, prefix string) bool {
	user, err := c.getUser(searchPattern)
	if err != nil {
		if strings.Contains(searchPattern, "@") {
			color.Yellow.Printf("%s     - %s # Inviting via email\n", prefix, searchPattern)
			return true
		} else {
			color.Red.Printf("    # %s, error: %v\n", searchPattern, err)
			return false
		}

	}
	color.Cyan.Printf("%s     - %s", prefix, user.Username)
	color.Green.Printf(" # %s (%s)\n", user.Name, searchPattern)
	return true
}

func checkDupsInGroups(groups []*config.Group) map[string][]string {
	studsWithGroups := make(map[string][]string)
	for _, grp := range groups {
		for _, student := range grp.Members {
			_, ok := studsWithGroups[student]
			if !ok {
				studsWithGroups[student] = []string{grp.Name}
			} else {
				studsWithGroups[student] = append(studsWithGroups[student], grp.Name)
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
