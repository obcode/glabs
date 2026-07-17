package gitlab

import (
	"github.com/gookit/color"
	"github.com/obcode/glabs/v3/config"
	"github.com/rs/zerolog/log"
)

func (c *Client) CheckCourse(cfg *config.CourseConfig) bool {
	noOfErrors := 0
	c.rep.Printf("%s", color.Cyan.Sprintf("%s:\n", cfg.Course))

	if len(cfg.Students) > 0 {
		c.rep.Println(color.Cyan.Sprint("  - students:"))

		for _, student := range cfg.Students {
			log.Debug().Interface("student", *student).Msg("checking student")
			if !c.checkStudent(student, "") {
				noOfErrors++
			}
		}
	}

	if len(cfg.Groups) > 0 {
		c.rep.Println(color.Cyan.Sprint("  - groups:"))

		for _, grp := range cfg.Groups {
			c.rep.Printf("%s", color.Cyan.Sprintf("    - %s:\n", grp.Name))
			for _, student := range grp.Members {
				log.Debug().Interface("student", *student).Msg("checking student")
				if !c.checkStudent(student, "  ") {
					noOfErrors++
				}
			}
		}

		c.rep.Printf("%s", color.Cyan.Sprint("  # checking duplicates in groups"))
		foundDup := false

		if studsInMoreGroups := checkDupsInGroups(cfg.Groups); len(studsInMoreGroups) > 0 {
			for student, inGroups := range studsInMoreGroups {
				c.rep.Printf("%s", color.Red.Sprintf("\n  # %s is in more than one group: %v", student, inGroups))
				foundDup = true
				noOfErrors++
			}
		}

		if !foundDup {
			c.rep.Println(color.Green.Sprint("... no duplicate found (but checked only the raw input)"))
		}
	}

	if noOfErrors > 0 {
		c.rep.Printf("%s", color.Red.Sprintf("\n# ===> %d error", noOfErrors))
		if noOfErrors == 1 {
			c.rep.Println()
		} else {
			c.rep.Println(color.Red.Sprint("s"))
		}
		return false
	}
	return true
}

func (c *Client) checkStudent(student *config.Student, prefix string) bool {
	user, err := c.getUser(student)
	if err != nil {
		if student.Email != nil {
			c.rep.Printf("%s", color.Yellow.Sprintf("%s     - %s # cannot get user info, inviting via email\n", prefix, *student.Email))
			return true
		}
		c.rep.Printf("%s", color.Red.Sprintf("    # %+v, error: %v\n", student, err))
		return false
	}

	if student.Id != nil {
		c.rep.Printf("%s", color.Green.Sprintf("%s     - %d # %s (@%s) specified via ID\n", prefix, user.ID, user.Name, user.Username))
	}
	if student.Username != nil {
		c.rep.Printf("%s", color.Red.Sprintf("%s     # please consider changing to UserID:\n", prefix))
		c.rep.Printf("%s", color.Red.Sprintf("%s     - %d # %s (@%s) specified via Username\n", prefix, user.ID, user.Name, user.Username))
	}
	return true
}

func checkDupsInGroups(groups []*config.Group) map[string][]string {
	studsWithGroups := make(map[string][]string)
	for _, grp := range groups {
		for _, student := range grp.Members {
			_, ok := studsWithGroups[student.Raw]
			if !ok {
				studsWithGroups[student.Raw] = []string{grp.Name}
			} else {
				studsWithGroups[student.Raw] = append(studsWithGroups[student.Raw], grp.Name)
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
