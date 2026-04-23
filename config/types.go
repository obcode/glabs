package config

import "github.com/ProtonMail/go-crypto/openpgp"

type Student struct {
	Id       *int
	Username *string
	Email    *string
	Raw      string
}

type AssignmentConfig struct {
	Course                string
	Name                  string
	UseCoursenameAsPrefix bool
	Path                  string
	URL                   string
	Per                   Per
	Description           string
	ContainerRegistry     bool
	AccessLevel           AccessLevel
	MergeRequest          *MergeRequest
	Branches              []BranchRule
	Issues                *IssueReplication
	Students              []*Student
	Groups                []*Group
	Startercode           *Startercode
	Clone                 *Clone
	Release               *Release
	Seeder                *Seeder
}

type Per string

const (
	PerStudent Per = "student"
	PerGroup   Per = "group"
	PerFailed  Per = "could not happen"
)

type Seeder struct {
	Command         string
	Args            []string
	Name            string
	EMail           string
	SignKey         *openpgp.Entity
	ToBranch        string
	ProtectToBranch bool
}

type Startercode struct {
	URL                string
	FromBranch         string
	ToBranch           string
	AdditionalBranches []string
}

type BranchRule struct {
	Name      string `mapstructure:"name"`
	Protect   bool   `mapstructure:"protect"`
	MergeOnly bool   `mapstructure:"mergeOnly"`
	Default   bool   `mapstructure:"default"`
}

type IssueReplication struct {
	ReplicateFromStartercode bool
	IssueNumbers             []int
}

type Clone struct {
	LocalPath string
	Branch    string
	Force     bool
}

type Release struct {
	MergeRequest *ReleaseMergeRequest
	DockerImages []string
}

type MergeRequest struct {
	MergeMethod                   MergeMethod
	SquashOption                  SquashOption
	PipelineMustSucceed           bool
	SkippedPipelinesAreSuccessful bool
	AllThreadsMustBeResolved      bool
	StatusChecksMustSucceed       bool
}

type ReleaseMergeRequest struct {
	SourceBranch string
	TargetBranch string
	HasPipeline  bool
}

type Group struct {
	Name    string
	Members []*Student
}

type AccessLevel int

const (
	Guest      AccessLevel = 10
	Reporter   AccessLevel = 20
	Developer  AccessLevel = 30
	Maintainer AccessLevel = 40
)

// MergeMethod represents the merge strategy for GitLab projects.
// Values correspond to glabs config format, not the GitLab API directly.
type MergeMethod string

const (
	// MergeCommit creates a merge commit for every merge (GitLab default).
	MergeCommit MergeMethod = "merge"
	// SemiLinearHistory requires linear history: rebase before creating merge commit.
	SemiLinearHistory MergeMethod = "semi_linear"
	// FastForward only allows fast-forward merges; no merge commits.
	FastForward MergeMethod = "ff"
)

// SquashOption represents the squash-on-merge setting for GitLab projects.
type SquashOption string

const (
	// SquashNever disables squashing for all merge requests.
	SquashNever SquashOption = "never"
	// SquashAlways squashes all merge requests automatically.
	SquashAlways SquashOption = "always"
	// SquashDefaultOff lets users opt in to squash per MR (default off).
	SquashDefaultOff SquashOption = "default_off"
	// SquashDefaultOn lets users opt out of squash per MR (default on).
	SquashDefaultOn SquashOption = "default_on"
)
