package config

import "github.com/ProtonMail/go-crypto/openpgp"

type Student struct {
	Id       *int
	Username *string
	Email    *string
	Raw      string
}

type AssignmentConfig struct {
	Course            string
	Name              string
	Path              string
	URL               string
	Per               Per
	Description       string
	ContainerRegistry bool
	AccessLevel       AccessLevel
	Students          []*Student
	Groups            []*Group
	Startercode       *Startercode
	Clone             *Clone
	Release           *Release
	Seeder            *Seeder
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
	DevBranch          string
	AdditionalBranches []string
	ProtectToBranch    bool
}

type Clone struct {
	LocalPath string
	Branch    string
	Force     bool
}

type Release struct {
	MergeRequest *MergeRequest
	DockerImages []string
}

type MergeRequest struct {
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
