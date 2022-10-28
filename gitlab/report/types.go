package report

import (
	"time"

	"github.com/xanzy/go-gitlab"
)

type Reports struct {
	Course      string           `json:"course"`
	Assignment  string           `json:"assignment"`
	URL         string           `json:"url"`
	Description string           `json:"description"`
	Projects    []*ProjectReport `json:"projects"`
	Generated   *time.Time       `json:"generated"`
}

type ProjectReport struct {
	Name                   string                  `json:"name"`
	IsActive               bool                    `json:"active"`
	IsEmpty                bool                    `json:"emptyRepo"`
	Commits                int                     `json:"commits"`
	CreatedAt              *time.Time              `json:"createdAt"`
	LastActivity           *time.Time              `json:"lastActivity"`
	LastCommit             *Commit                 `json:"commit"`
	OpenIssuesCount        int                     `json:"openIssuesCount"`
	OpenMergeRequestsCount int                     `json:"openMergeRequestsCount"`
	WebURL                 string                  `json:"webURL"`
	Members                []*gitlab.ProjectMember `json:"members"`
}

type Commit struct {
	Title         string     `json:"title"`
	CommitterName string     `json:"committerName"`
	CommittedDate *time.Time `json:"committedDate"`
	WebURL        string     `json:"webURL"`
}
