package config

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type CourseConfig struct {
	Course   string
	Students []*Student
	Groups   []*Group
}

func GetCourseConfig(course string) *CourseConfig {
	if !viper.IsSet(course) {
		log.Fatal().
			Str("course", course).
			Msg("configuration for course not found")
		return nil
	}

	return &CourseConfig{
		Course:   course,
		Students: students(PerStudent, course, ""),
		Groups:   groups(PerGroup, course, ""),
	}
}

// CourseExists checks if a course configuration exists
func CourseExists(course string) bool {
	return viper.IsSet(course)
}

// StudentKey generates a unique key for a student based on available identifiers
func StudentKey(student *Student) string {
	if student.Email != nil {
		return *student.Email
	}
	if student.Username != nil {
		return *student.Username
	}
	if student.Id != nil {
		return fmt.Sprintf("id:%d", *student.Id)
	}
	return ""
}
