package config

import (
	"fmt"
	"strings"

	"github.com/logrusorgru/aurora/v4"
)

func (cfg *AssignmentConfig) Show() {
	containerRegistry := aurora.Red("disabled")
	if cfg.ContainerRegistry {
		containerRegistry = aurora.Green("enabled")
	}

	startercode := aurora.Sprintf(aurora.Red("not defined"))
	if cfg.Startercode != nil {
		startercode = aurora.Sprintf(aurora.Cyan(`
  URL:                %s
  FromBranch:         %s
  ToBranch:           %s
  DevBranch:          %s
  AdditionalBranches: %s
  ProtectToBranch:    %t`),
			aurora.Yellow(cfg.Startercode.URL),
			aurora.Yellow(cfg.Startercode.FromBranch),
			aurora.Yellow(cfg.Startercode.ToBranch),
			aurora.Yellow(cfg.Startercode.DevBranch),
			aurora.Yellow(cfg.Startercode.AdditionalBranches),
			aurora.Yellow(cfg.Startercode.ProtectToBranch),
		)
	}
	seeding := aurora.Sprintf(aurora.Red("not defined"))
	if cfg.Seeder != nil {
		seeding = aurora.Sprintf(aurora.Cyan(`
  Command:          %s
  Args:             %v
  Author:           %s
  EMail:            %s
  ToBranch:         %s
  ProtectToBranch:  %t`),
			aurora.Yellow(cfg.Seeder.Command),
			aurora.Yellow(cfg.Seeder.Args),
			aurora.Yellow(cfg.Seeder.Name),
			aurora.Yellow(cfg.Seeder.EMail),
			aurora.Yellow(cfg.Seeder.ToBranch),
			aurora.Yellow(cfg.Seeder.ProtectToBranch),
		)
	}
	clone := aurora.Sprintf(aurora.Red("        not defined"))
	if cfg.Clone != nil {
		clone = aurora.Sprintf(aurora.Cyan(`Clone:
  Localpath:        %s
  Branch:           %s
  Force:            %t`),
			aurora.Yellow(cfg.Clone.LocalPath),
			aurora.Yellow(cfg.Clone.Branch),
			aurora.Yellow(cfg.Clone.Force),
		)
	}
	release := aurora.Sprintf(aurora.Red("not defined"))
	if cfg.Release != nil {
		mergeRequest := aurora.Sprintf(aurora.Red("not defined"))
		if cfg.Release.MergeRequest != nil {
			mergeRequest = aurora.Sprintf(`
    %s   %s
    %s   %s
    %s       %t`,
				aurora.Cyan("SourceBranch:"),
				aurora.Yellow(cfg.Release.MergeRequest.SourceBranch),
				aurora.Cyan("TargetBranch:"),
				aurora.Yellow(cfg.Release.MergeRequest.TargetBranch),
				aurora.Cyan("Pipeline:"),
				aurora.Yellow(cfg.Release.MergeRequest.HasPipeline),
			)
		}
		dockerImages := aurora.Sprintf(aurora.Red("not defined"))
		if cfg.Release.DockerImages != nil {
			var images strings.Builder
			images.WriteString(aurora.Sprintf(aurora.Cyan("\n")))
			for _, image := range cfg.Release.DockerImages {
				images.WriteString(aurora.Sprintf(aurora.Cyan("    - ")))
				images.WriteString(aurora.Sprintf(aurora.Yellow(image)))
				images.WriteString("\n")
			}
			dockerImages = images.String()
		}
		release = aurora.Sprintf(aurora.Cyan(`
  %s     %s
  %s     %s`),
			aurora.Cyan("MergeRequest:"),
			mergeRequest,
			aurora.Cyan("DockerImages:"),
			dockerImages,
		)
	}

	var per strings.Builder
	switch cfg.Per {
	case PerStudent:
		per.WriteString(aurora.Sprintf(aurora.Cyan("Students:\n")))
		for _, s := range cfg.Students {
			per.WriteString(aurora.Sprintf(aurora.Cyan("  - ")))
			per.WriteString(aurora.Sprintf(aurora.Yellow(s.Raw)))
			per.WriteString("\n")
		}
	case PerGroup:
		per.WriteString(aurora.Sprintf(aurora.Cyan("Groups:\n")))
		for _, grp := range cfg.Groups {
			per.WriteString(aurora.Sprintf(aurora.Cyan("  - ")))
			per.WriteString(aurora.Sprintf(aurora.Yellow(grp.Name)))
			per.WriteString(aurora.Sprintf(aurora.Cyan(": ")))
			for i, m := range grp.Members {
				per.WriteString(aurora.Sprintf(aurora.Green(m.Raw)))
				if i == len(grp.Members)-1 {
					per.WriteString("\n")
				} else {
					per.WriteString(aurora.Sprintf(aurora.Cyan(", ")))
				}
			}
		}
	}

	groupsOrStudents := per.String()

	fmt.Print(aurora.Sprintf(aurora.Cyan(`
Course:             %s
Assignment:         %s
Per:                %s
Base-URL:           %s
Description:	    %s
AccessLevel:        %s
Container-Registry: %s
Startercode:        %s
Seeding:            %s
Release:            %s
%s
%s
`),
		aurora.Yellow(cfg.Course),
		aurora.Yellow(cfg.Name),
		aurora.Yellow(cfg.Per),
		aurora.Yellow(cfg.URL),
		aurora.Yellow(cfg.Description),
		aurora.Yellow(cfg.AccessLevel.String()),
		containerRegistry,
		aurora.Yellow(startercode),
		aurora.Yellow(seeding),
		aurora.Yellow(release),
		aurora.Yellow(clone),
		aurora.Yellow(groupsOrStudents),
	))
}
