package config

import (
	"sort"
	"strconv"
	"strings"

	"github.com/go-email-validator/go-email-validator/pkg/ev"
	"github.com/go-email-validator/go-email-validator/pkg/ev/evmail"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
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

func accessLevel(assignmentKey string) AccessLevel {
	accesslevelIdentifier := viper.GetString(assignmentKey + ".accesslevel")

	switch accesslevelIdentifier {
	case "guest":
		return Guest
	case "reporter":
		return Reporter
	case "maintainer":
		return Maintainer
	}

	return Developer
}

func students(per Per, course, assignment string, onlyForStudentsOrGroups ...string) []*Student {
	if per == PerGroup {
		return nil
	}

	studs := viper.GetStringSlice(course + "." + assignment + ".students")
	studsGlobal := viper.GetStringSlice(course + ".students")

	if len(studs) == 0 {
		studs = studsGlobal
	} else {
		studs = append(studs, studsGlobal...)
	}

	if len(onlyForStudentsOrGroups) > 0 {
		onlyForStudents := make([]string, 0, len(onlyForStudentsOrGroups))
		for _, onlyStudent := range onlyForStudentsOrGroups {
			for _, student := range studs {
				if onlyStudent == student {
					onlyForStudents = append(onlyForStudents, onlyStudent)
				}
			}
		}
		studs = onlyForStudents
	}

	log.Debug().Interface("students", students).Msg("found students")
	sort.Strings(studs)
	students := mkStudents(studs)
	return students
}

func mkStudents(studs []string) []*Student {
	students := make([]*Student, 0, len(studs))

	for _, stud := range studs {
		rawStud := stud
		student := &Student{Raw: rawStud}
		// E-Mail?
		if ev.NewSyntaxValidator().Validate(ev.NewInput(evmail.FromString(stud))).IsValid() {
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

func groups(per Per, course, assignment string, onlyForStudentsOrGroups ...string) []*Group {
	if per == PerStudent {
		return nil
	}

	groupsMapAssignmemt := viper.GetStringMapStringSlice(course + "." + assignment + ".groups")
	groupsMap := viper.GetStringMapStringSlice(course + ".groups")

	if len(groupsMapAssignmemt) > 0 {
		for k, v := range groupsMapAssignmemt {
			groupsMap[k] = v
		}
	}

	if len(onlyForStudentsOrGroups) > 0 {
		onlyTheseGroups := make(map[string][]string)
		for _, onlyGroup := range onlyForStudentsOrGroups {
			for groupname, students := range groupsMap {
				if onlyGroup == groupname {
					onlyTheseGroups[groupname] = students
				}
			}
		}
		groupsMap = onlyTheseGroups
	}

	keys := make([]string, 0, len(groupsMap))
	for k := range groupsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	groups := make([]*Group, 0, len(groupsMap))
	for _, groupname := range keys {
		members := groupsMap[groupname]
		sort.Strings(members)
		studentMembers := mkStudents(members)
		groups = append(groups, &Group{
			Name:    groupname,
			Members: studentMembers,
		})
	}

	return groups
}
