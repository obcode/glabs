package gitlab

import (
	"fmt"
	"sort"

	"github.com/gookit/color"
	"github.com/obcode/glabs/v3/config"
	"github.com/rs/zerolog/log"
)

// StudentCheckStatus classifies how a roster entry resolved against GitLab.
type StudentCheckStatus string

const (
	// CheckOK: the user resolved (pinned by ID, or matched uniquely by email).
	CheckOK StudentCheckStatus = "ok"
	// CheckInvite: no GitLab user found, but there is an email → invite by email.
	CheckInvite StudentCheckStatus = "invite"
	// CheckDeprecated: resolved by username — works, but pinning by ID is safer.
	CheckDeprecated StudentCheckStatus = "deprecated"
	// CheckError: cannot resolve and there is no email to fall back to.
	CheckError StudentCheckStatus = "error"
)

// StudentCheck is the classification of one roster entry.
type StudentCheck struct {
	Input   string // the raw roster entry, as written
	Status  StudentCheckStatus
	Message string
}

// GroupCheck is the classification of one group's members.
type GroupCheck struct {
	Name    string
	Members []*StudentCheck
}

// DuplicateCheck is one student that appears in more than one group.
type DuplicateCheck struct {
	Student string
	Groups  []string
}

// CheckResult is the data behind CheckCourse: per-student classification for the
// course's students and groups, the students that appear in more than one group,
// and whether the whole course checks out.
type CheckResult struct {
	Course     string
	Students   []*StudentCheck
	Groups     []*GroupCheck
	Duplicates []*DuplicateCheck
	Errors     int
	OK         bool
}

// classifyStudent resolves one roster entry against GitLab and classifies it. It
// is the data-returning counterpart of checkStudent (which prints for the CLI).
func (c *Client) classifyStudent(student *config.Student) *StudentCheck {
	sc := &StudentCheck{Input: student.Raw}
	user, err := c.getUser(student)
	if err != nil {
		if student.Email != nil {
			sc.Status = CheckInvite
			sc.Message = fmt.Sprintf("%s — no GitLab user yet, will invite by email", *student.Email)
			return sc
		}
		sc.Status = CheckError
		sc.Message = fmt.Sprintf("cannot resolve %q: %v", student.Raw, err)
		return sc
	}
	switch {
	case student.Id != nil:
		sc.Status = CheckOK
		sc.Message = fmt.Sprintf("%s (@%s) via ID %d", user.Name, user.Username, user.ID)
	case student.Username != nil:
		sc.Status = CheckDeprecated
		sc.Message = fmt.Sprintf("%s (@%s) via username — consider pinning by ID %d", user.Name, user.Username, user.ID)
	default:
		sc.Status = CheckOK
		sc.Message = fmt.Sprintf("%s (@%s) via email", user.Name, user.Username)
	}
	return sc
}

// CheckCourseData resolves every roster entry of the course against GitLab and
// returns the classification, reporting progress per student through c.rep (so a
// streaming reporter can forward it live). It never prints; the CLI's CheckCourse
// keeps its own colored rendering.
func (c *Client) CheckCourseData(cfg *config.CourseConfig) *CheckResult {
	task := c.rep.Task(fmt.Sprintf("checking %s", cfg.Course))
	res := &CheckResult{Course: cfg.Course}
	errors := 0

	for _, student := range cfg.Students {
		task.Update(fmt.Sprintf("checking student %s", student.Raw))
		sc := c.classifyStudent(student)
		if sc.Status == CheckError {
			errors++
		}
		res.Students = append(res.Students, sc)
	}

	for _, grp := range cfg.Groups {
		gc := &GroupCheck{Name: grp.Name}
		for _, student := range grp.Members {
			task.Update(fmt.Sprintf("checking %s / %s", grp.Name, student.Raw))
			sc := c.classifyStudent(student)
			if sc.Status == CheckError {
				errors++
			}
			gc.Members = append(gc.Members, sc)
		}
		res.Groups = append(res.Groups, gc)
	}

	for student, groups := range checkDupsInGroups(cfg.Groups) {
		res.Duplicates = append(res.Duplicates, &DuplicateCheck{Student: student, Groups: groups})
		errors++
	}
	sort.Slice(res.Duplicates, func(i, j int) bool { return res.Duplicates[i].Student < res.Duplicates[j].Student })

	task.Done("")
	res.Errors = errors
	res.OK = errors == 0
	return res
}

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
