package config

import (
	"fmt"
	"sort"
	"strings"
	"syscall"

	"github.com/logrusorgru/aurora"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/term"
)

type AssignmentConfig struct {
	Course            string
	Name              string
	Path              string
	URL               string
	Per               Per
	Description       string
	ContainerRegistry bool
	AccessLevel       AccessLevel
	Students          []string
	Groups            []*Group
	Startercode       *Startercode
	Clone             *Clone
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
	URL             string
	FromBranch      string
	ToBranch        string
	ProtectToBranch bool
}

type Clone struct {
	LocalPath string
	Branch    string
	Force     bool
}

type Group struct {
	Name    string
	Members []string
}

type AccessLevel int

const (
	Guest      AccessLevel = 10
	Reporter   AccessLevel = 20
	Developer  AccessLevel = 30
	Maintainer AccessLevel = 40
)

func (ac AccessLevel) String() string {
	if ac == 10 {
		return "guest"
	}
	if ac == 20 {
		return "reporter"
	}
	if ac == 30 {
		return "developer"
	}
	return "maintainer"
}

func GetAssignmentConfig(course, assignment string, onlyForStudentsOrGroups ...string) *AssignmentConfig {
	if !viper.IsSet(course) {
		log.Fatal().
			Str("course", course).
			Msg("configuration for course not found")
	}

	if !viper.IsSet(course + "." + assignment) {
		log.Fatal().
			Str("course", course).
			Str("assignment", assignment).
			Msg("configuration for assignment not found")
	}

	assignmentKey := course + "." + assignment
	per := per(assignmentKey)

	path := assignmentPath(course, assignment)
	url := viper.GetString("gitlab.host") + "/" + path

	assignmentConfig := &AssignmentConfig{
		Course:            course,
		Name:              assignment,
		Path:              path,
		URL:               url,
		Per:               per,
		Description:       description(assignmentKey),
		ContainerRegistry: viper.GetBool(assignmentKey + ".containerRegistry"),
		AccessLevel:       accessLevel(assignmentKey),
		Students:          students(per, course, onlyForStudentsOrGroups...),
		Groups:            groups(per, course, onlyForStudentsOrGroups...),
		Startercode:       startercode(assignmentKey),
		Clone:             clone(assignmentKey),
		Seeder:            seeder(assignmentKey),
	}

	return assignmentConfig
}

func assignmentPath(course, assignment string) string {
	path := viper.GetString(course + ".coursepath")
	if semesterpath := viper.GetString(course + ".semesterpath"); len(semesterpath) > 0 {
		path += "/" + semesterpath
	}

	assignmentpath := path
	if group := viper.GetString(course + "." + assignment + ".assignmentpath"); len(group) > 0 {
		assignmentpath += "/" + group
	}

	return assignmentpath
}

func per(assignmentKey string) Per {
	if per := viper.GetString(assignmentKey + ".per"); per == "group" {
		return PerGroup
	}
	return PerStudent
}

func description(assignmentKey string) string {
	description := "generated by glabs"

	if desc := viper.GetString(assignmentKey + ".description"); desc != "" {
		description = desc
	}

	return description
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

func students(per Per, course string, onlyForStudentsOrGroups ...string) []string {
	if per == PerGroup {
		return nil
	}
	students := viper.GetStringSlice(course + ".students")
	if len(onlyForStudentsOrGroups) > 0 {
		onlyForStudents := make([]string, 0, len(onlyForStudentsOrGroups))
		for _, onlyStudent := range onlyForStudentsOrGroups {
			for _, student := range students {
				if onlyStudent == student {
					onlyForStudents = append(onlyForStudents, onlyStudent)
				}
			}
		}
		students = onlyForStudents
	}

	log.Debug().Interface("students", students).Msg("found students")
	sort.Strings(students)
	return students
}

func groups(per Per, course string, onlyForStudentsOrGroups ...string) []*Group {
	if per == PerStudent {
		return nil
	}

	groupsMap := viper.GetStringMapStringSlice(course + ".groups")
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
		groups = append(groups, &Group{
			Name:    groupname,
			Members: members,
		})
	}

	return groups
}

func startercode(assignmentKey string) *Startercode {
	startercodeMap := viper.GetStringMapString(assignmentKey + ".startercode")

	if len(startercodeMap) == 0 {
		log.Debug().Str("assignmemtKey", assignmentKey).Msg("no startercode provided")
		return nil
	}

	url, ok := startercodeMap["url"]
	if !ok {
		log.Fatal().Str("assignmemtKey", assignmentKey).Msg("startercode provided without url")
		return nil
	}

	fromBranch := "master"
	if fB := viper.GetString(assignmentKey + ".startercode.fromBranch"); len(fB) > 0 {
		fromBranch = fB
	}

	toBranch := "master"
	if tB := viper.GetString(assignmentKey + ".startercode.toBranch"); len(tB) > 0 {
		toBranch = tB
	}

	return &Startercode{
		URL:             url,
		FromBranch:      fromBranch,
		ToBranch:        toBranch,
		ProtectToBranch: viper.GetBool(assignmentKey + ".startercode.protectToBranch"),
	}
}

func seeder(assignmentKey string) *Seeder {
	seederMap := viper.GetStringMapString(assignmentKey + ".seeder")

	if len(seederMap) == 0 {
		log.Debug().Str("assignmemtKey", assignmentKey).Msg("no seeder provided")
		return nil
	}

	cmd, ok := seederMap["cmd"]
	if !ok {
		log.Fatal().Str("assignmemtKey", assignmentKey).Msg("seeder provided without cmd")
		return nil
	}

	toBranch := "master"
	if tB := viper.GetString(assignmentKey + ".seeder.toBranch"); len(tB) > 0 {
		toBranch = tB
	}

	privKeyString := viper.GetString(assignmentKey + ".seeder.signKey")
	var entity *openpgp.Entity
	entity = nil
	if privKeyString != "" {
		entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privKeyString))
		if err != nil {
			log.Fatal()
		}
		if entities[0].PrivateKey.Encrypted {
			fmt.Println(aurora.Blue("Passphrase for signing key is required. Please enter it now:"))
			passphrase, _ := term.ReadPassword(int(syscall.Stdin))
			err = entities[0].PrivateKey.Decrypt(passphrase)
			if err != nil {
				log.Fatal()
			}
		}
		entity = entities[0]

	}

	return &Seeder{
		Command:         cmd,
		Args:            viper.GetStringSlice(assignmentKey + ".seeder.args"),
		Name:            viper.GetString(assignmentKey + ".seeder.name"),
		SignKey:         entity,
		EMail:           viper.GetString(assignmentKey + ".seeder.email"),
		ToBranch:        toBranch,
		ProtectToBranch: viper.GetBool(assignmentKey + ".seeder.protectToBranch"),
	}
}

func clone(assignmentKey string) *Clone {
	cloneMap := viper.GetStringMapString(assignmentKey + ".clone")

	localpath, ok := cloneMap["localpath"]
	if !ok {
		localpath = "."
	}

	branch, ok := cloneMap["branch"]
	if !ok {
		branch = "master"
	}

	force := viper.GetBool(assignmentKey + ".clone.force")

	return &Clone{
		LocalPath: localpath,
		Branch:    branch,
		Force:     force,
	}
}

func (cfg *AssignmentConfig) SetBranch(branch string) {
	cfg.Clone.Branch = branch
}

func (cfg *AssignmentConfig) SetLocalpath(localpath string) {
	cfg.Clone.LocalPath = localpath
}

func (cfg *AssignmentConfig) SetForce() {
	cfg.Clone.Force = true
}
