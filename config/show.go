package config

import (
	"fmt"
	"strings"

	"github.com/logrusorgru/aurora/v4"
)

func (cfg *AssignmentConfig) Show() {
	var out strings.Builder

	const (
		topLabelWidth     = 19
		sectionLabelWidth = 28
	)

	writeTopField := func(label string, value any) {
		fmt.Fprintf(&out, "%-*v %v\n", topLabelWidth, aurora.Cyan(label+":"), aurora.Yellow(value))
	}

	writeSectionHeader := func(name string) {
		fmt.Fprintf(&out, "%s\n", aurora.Cyan(name+":"))
	}

	writeSectionField := func(label string, value any) {
		fmt.Fprintf(&out, "  %-*v %v\n", sectionLabelWidth, aurora.Cyan(label+":"), aurora.Yellow(value))
	}

	writeSectionNotDefined := func() {
		fmt.Fprintf(&out, "  %s\n", aurora.Red("not defined"))
	}

	mergeMethod := MergeCommit
	squashOption := SquashDefaultOff
	pipelineMustSucceed := false
	skippedPipelinesAreSuccessful := false
	allThreadsMustBeResolved := false
	statusChecksMustSucceed := false
	if cfg.MergeRequest != nil {
		mergeMethod = cfg.MergeRequest.MergeMethod
		squashOption = cfg.MergeRequest.SquashOption
		pipelineMustSucceed = cfg.MergeRequest.PipelineMustSucceed
		skippedPipelinesAreSuccessful = cfg.MergeRequest.SkippedPipelinesAreSuccessful
		allThreadsMustBeResolved = cfg.MergeRequest.AllThreadsMustBeResolved
		statusChecksMustSucceed = cfg.MergeRequest.StatusChecksMustSucceed
	}

	containerRegistry := aurora.Red("disabled")
	if cfg.ContainerRegistry {
		containerRegistry = aurora.Green("enabled")
	}

	writeTopField("Course", cfg.Course)
	writeTopField("Assignment", cfg.Name)
	writeTopField("Coursename-Prefix", cfg.UseCoursenameAsPrefix)
	writeTopField("Per", cfg.Per)
	writeTopField("Base-URL", cfg.URL)
	writeTopField("Description", cfg.Description)
	writeTopField("AccessLevel", cfg.AccessLevel.String())
	writeTopField("Container-Registry", containerRegistry)

	writeSectionHeader("MergeRequest")
	writeSectionField("MergeMethod", mergeMethod)
	writeSectionField("SquashOption", squashOption)
	writeSectionField("PipelineMustSucceed", pipelineMustSucceed)
	writeSectionField("SkippedPipelinesAreSuccessful", skippedPipelinesAreSuccessful)
	writeSectionField("AllThreadsMustBeResolved", allThreadsMustBeResolved)
	writeSectionField("StatusChecksMustSucceed", statusChecksMustSucceed)

	writeSectionHeader("Startercode")
	if cfg.Startercode == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("URL", cfg.Startercode.URL)
		writeSectionField("FromBranch", cfg.Startercode.FromBranch)
		writeSectionField("ToBranch", cfg.Startercode.ToBranch)
		writeSectionField("AdditionalBranches", cfg.Startercode.AdditionalBranches)
	}

	writeSectionHeader("Branches")
	if len(cfg.Branches) == 0 {
		writeSectionNotDefined()
	} else {
		for _, branch := range cfg.Branches {
			fmt.Fprintf(
				&out,
				"  - %-20v (%v=%-5v, %v=%-5v, %v=%-5v, %v=%-5v, %v=%-5v)\n",
				aurora.Yellow(branch.Name),
				aurora.Cyan("protect"), aurora.Yellow(branch.Protect),
				aurora.Cyan("mergeOnly"), aurora.Yellow(branch.MergeOnly),
				aurora.Cyan("default"), aurora.Yellow(branch.Default),
				aurora.Cyan("allowForcePush"), aurora.Yellow(branch.AllowForcePush),
				aurora.Cyan("codeOwnerApprovalRequired"), aurora.Yellow(branch.CodeOwnerApprovalRequired),
			)
		}
	}

	writeSectionHeader("Issues")
	if cfg.Issues == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("ReplicateFromStartercode", cfg.Issues.ReplicateFromStartercode)
		if cfg.Issues.ReplicateFromStartercode {
			writeSectionField("IssueNumbers", cfg.Issues.IssueNumbers)
		} else {
			writeSectionField("IssueNumbers", "not used")
		}
	}

	writeSectionHeader("Seeding")
	if cfg.Seeder == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("Command", cfg.Seeder.Command)
		writeSectionField("Args", cfg.Seeder.Args)
		writeSectionField("Author", cfg.Seeder.Name)
		writeSectionField("EMail", cfg.Seeder.EMail)
		writeSectionField("ToBranch", cfg.Seeder.ToBranch)
		writeSectionField("ProtectToBranch", cfg.Seeder.ProtectToBranch)
	}

	writeSectionHeader("Release")
	if cfg.Release == nil {
		writeSectionNotDefined()
	} else {
		if cfg.Release.MergeRequest == nil {
			writeSectionField("MergeRequest", "not defined")
		} else {
			writeSectionField("MergeRequest.SourceBranch", cfg.Release.MergeRequest.SourceBranch)
			writeSectionField("MergeRequest.TargetBranch", cfg.Release.MergeRequest.TargetBranch)
			writeSectionField("MergeRequest.Pipeline", cfg.Release.MergeRequest.HasPipeline)
		}
		if len(cfg.Release.DockerImages) == 0 {
			writeSectionField("DockerImages", "not defined")
		} else {
			writeSectionField("DockerImages", cfg.Release.DockerImages)
		}
	}

	writeSectionHeader("Clone")
	if cfg.Clone == nil {
		writeSectionNotDefined()
	} else {
		writeSectionField("Localpath", cfg.Clone.LocalPath)
		writeSectionField("Branch", cfg.Clone.Branch)
		writeSectionField("Force", cfg.Clone.Force)
	}

	switch cfg.Per {
	case PerStudent:
		writeSectionHeader("Students")
		if len(cfg.Students) == 0 {
			writeSectionNotDefined()
		} else {
			for _, s := range cfg.Students {
				fmt.Fprintf(&out, "  - %s\n", aurora.Yellow(s.Raw))
			}
		}
	case PerGroup:
		writeSectionHeader("Groups")
		if len(cfg.Groups) == 0 {
			writeSectionNotDefined()
		} else {
			for _, grp := range cfg.Groups {
				members := make([]string, 0, len(grp.Members))
				for _, m := range grp.Members {
					members = append(members, m.Raw)
				}
				fmt.Fprintf(&out, "  - %s: %s\n", aurora.Yellow(grp.Name), aurora.Green(strings.Join(members, ", ")))
			}
		}
	}

	fmt.Println(out.String())
}
