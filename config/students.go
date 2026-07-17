package config

import (
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func (cfg *AssignmentConfig) SetAccessLevel(level string) {
	accesslevel := Developer
	switch level {
	case "guest":
		accesslevel = Guest
	case "reporter":
		accesslevel = Reporter
	case "maintainer":
		accesslevel = Maintainer
	}

	cfg.AccessLevel = accesslevel
}

// matchesPattern reports whether value matches the regexp pattern. An invalid
// pattern matches nothing rather than erroring: these come from the CLI's
// positional arguments, where a plain name is the common case and a typo should
// simply select no repositories.
func matchesPattern(pattern, value string) bool {
	ok, err := regexp.MatchString(pattern, value)
	return ok && err == nil
}

func mkStudents(studs []string) []*Student {
	students := make([]*Student, 0, len(studs))

	for _, stud := range studs {
		rawStud := stud
		student := &Student{Raw: rawStud}

		if strings.HasPrefix(stud, "-") {
			log.Warn().Str("student in config", stud).Msg("student identifier starts with '-', did you miss the space after hyphen?")
		}

		// E-Mail?
		if _, err := mail.ParseAddress(stud); err == nil {
			log.Debug().Str("student in config", stud).Msg("is valid email")
			student.Email = &rawStud
		} else {
			// ID?
			userID, err := strconv.Atoi(rawStud)
			if strings.HasPrefix(rawStud, "0") || err != nil {
				log.Debug().Str("student in config", rawStud).Msg("must be username")
				student.Username = &student.Raw
			} else {
				log.Debug().Str("student in config", rawStud).Msg("is user id")
				student.Id = &userID
			}
		}
		students = append(students, student)
		log.Debug().Interface("student", student).Msg("added student")
	}

	return students
}
