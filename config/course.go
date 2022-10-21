package config

import (
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
