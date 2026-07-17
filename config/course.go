package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type CourseConfig struct {
	Course   string
	Students []*Student
	Groups   []*Group
}

func GetCourseConfig(course string) (*CourseConfig, error) {
	if !viper.IsSet(course) {
		return nil, fmt.Errorf("configuration for course %s not found", course)
	}

	return &CourseConfig{
		Course:   course,
		Students: students(PerStudent, course, ""),
		Groups:   groups(PerGroup, course, ""),
	}, nil
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
