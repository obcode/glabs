package report

import (
	"time"

	"github.com/xanzy/go-gitlab"
)

type Reports struct {
	Course                 string           `json:"course"`
	Assignment             string           `json:"assignment"`
	URL                    string           `json:"url"`
	Description            string           `json:"description"`
	Projects               []*ProjectReport `json:"projects"`
	Generated              *time.Time       `json:"generated"`
	HasReleaseMergeRequest bool             `json:"hasReleaseMergeRequest"`
	HasReleaseDockerImages bool             `json:"hasReleaseDockerImages"`
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
	Release                *Release                `json:"release"`
}

type Commit struct {
	Title         string     `json:"title"`
	CommitterName string     `json:"committerName"`
	CommittedDate *time.Time `json:"committedDate"`
	WebURL        string     `json:"webURL"`
}

type MergeRequest struct {
	Found          bool   `json:"found"`
	WebURL         string `json:"webURL"`
	PipelineStatus string `json:"pipelineStatus"`
}

type DockerImages struct {
	Status string         `json:"status"`
	Images []*DockerImage `json:"images"`
}

type DockerImage struct {
	Wanted string  `json:"wanted"`
	Image  *string `json:"image"`
}
type Release struct {
	MergeRequest *MergeRequest `json:"mergeRequest"`
	DockerImages *DockerImages `json:"dockerImages"`
}
