package config

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func release(assignmentKey string) *Release {
	releaseMap := viper.GetStringMap(assignmentKey + ".release")

	if len(releaseMap) == 0 {
		log.Debug().Str("assignmemtKey", assignmentKey).Msg("no release provided")
		return nil
	}

	return &Release{
		MergeRequest: mergeRequest(assignmentKey),
		DockerImages: dockerImages(assignmentKey),
	}
}

func mergeRequest(assignmentKey string) *MergeRequest {
	mergeRequestMap := viper.GetStringMapString(assignmentKey + ".release.mergeRequest")
	if len(mergeRequestMap) == 0 {
		log.Debug().Str("assignmemtKey", assignmentKey).Msg("no release by merge request provided")
		return nil
	}

	sourceBranch := "develop"
	if sB := viper.GetString(assignmentKey + ".release.mergeRequest.source"); len(sB) > 0 {
		sourceBranch = sB
	}

	targetBranch := "main"
	if tB := viper.GetString(assignmentKey + ".release.mergeRequest.target"); len(tB) > 0 {
		targetBranch = tB
	}

	return &MergeRequest{
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		HasPipeline:  viper.GetBool(assignmentKey + ".release.mergeRequest.pipeline"),
	}
}

func dockerImages(assignmentKey string) []string {
	dockerImagesSlice := viper.GetStringSlice(assignmentKey + ".release.dockerImages")
	if len(dockerImagesSlice) == 0 {
		return nil
	}
	return dockerImagesSlice
}
