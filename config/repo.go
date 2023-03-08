package config

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

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

	fromBranch := "main"
	if fB := viper.GetString(assignmentKey + ".startercode.fromBranch"); len(fB) > 0 {
		fromBranch = fB
	}

	toBranch := "main"
	if tB := viper.GetString(assignmentKey + ".startercode.toBranch"); len(tB) > 0 {
		toBranch = tB
	}

	devBranch := toBranch
	if dB := viper.GetString(assignmentKey + ".startercode.devBranch"); len(dB) > 0 {
		devBranch = dB
	}

	additionalBranches := []string{}
	if addB := viper.GetStringSlice(assignmentKey + ".startercode.additionalBranches"); len(addB) > 0 {
		additionalBranches = addB
	}

	return &Startercode{
		URL:                url,
		FromBranch:         fromBranch,
		ToBranch:           toBranch,
		DevBranch:          devBranch,
		AdditionalBranches: additionalBranches,
		ProtectToBranch:    viper.GetBool(assignmentKey + ".startercode.protectToBranch"),
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
		branch = "main"
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

func (cfg *AssignmentConfig) SetProtectToBranch(branch string) {
	if branch != "" {
		cfg.Startercode.ToBranch = branch
	}
	cfg.Startercode.ProtectToBranch = true
}

func (cfg *AssignmentConfig) SetLocalpath(localpath string) {
	cfg.Clone.LocalPath = localpath
}

func (cfg *AssignmentConfig) SetForce() {
	cfg.Clone.Force = true
}
