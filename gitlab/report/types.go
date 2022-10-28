package report

import "time"

type Reports struct {
	Course      string           `json:"course"`
	Assignment  string           `json:"assignment"`
	URL         string           `json:"url"`
	Description string           `json:"description"`
	Projects    []*ProjectReport `json:"projects"`
}

type ProjectReport struct {
	Name            string     `json:"name"`
	IsActive        bool       `json:"active"`
	IsEmpty         bool       `json:"emptyRepo"`
	Commits         int        `json:"commits"`
	LastActivity    *time.Time `json:"lastActvity"`
	OpenIssuesCount int        `json:"openIssuesCount"`
	WebURL          string     `json:"webURL"`
}
