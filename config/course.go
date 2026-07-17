package config

import "fmt"

type CourseConfig struct {
	Course   string
	Students []*Student
	Groups   []*Group
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
