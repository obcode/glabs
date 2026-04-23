package gitlab

import (
	"fmt"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) ProtectToBranch(assignmentCfg *config.AssignmentConfig) {
	_, err := c.getGroupID(assignmentCfg)
	if err != nil {
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		exitFunc(1)
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.protectToBranchPerGroup(assignmentCfg)
	case config.PerStudent:
		c.protectToBranchPerStudent(assignmentCfg)
	default:
		fmt.Printf("it is only possible to protect the branch for students oder groups, not for %v", per)
		exitFunc(1)
	}
}

func (c *Client) protectBranch(assignmentCfg *config.AssignmentConfig, project *gitlab.Project, spin bool) error {
	if hasProtectedBranches(assignmentCfg.Branches) {
		// var cfg yacspin.Config
		var spinner *yacspin.Spinner
		if spin {
			cfg := yacspin.Config{
				Frequency: 100 * time.Millisecond,
				CharSet:   yacspin.CharSets[69],
				Suffix: aurora.Sprintf(aurora.Cyan(" protect branch for project %s at %s"),
					aurora.Yellow(project.Name),
					aurora.Magenta(assignmentCfg.URL+"/"+project.Name),
				),
				SuffixAutoColon:   true,
				StopCharacter:     "✓",
				StopColors:        []string{"fgGreen"},
				StopFailMessage:   "error",
				StopFailCharacter: "✗",
				StopFailColors:    []string{"fgRed"},
			}
			var err error
			spinner, err = yacspin.New(cfg)
			if err != nil {
				log.Debug().Err(err).Msg("cannot create spinner")
			}
			err = spinner.Start()
			if err != nil {
				log.Debug().Err(err).Msg("cannot start spinner")
			}
		}

		log.Debug().
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Interface("branches", assignmentCfg.Branches).
			Msg("protecting branch")

		for _, branch := range assignmentCfg.Branches {
			if !branch.Protect && !branch.MergeOnly {
				continue
			}

			pushLevel := gitlab.MaintainerPermissions
			mergeLevel := gitlab.MaintainerPermissions
			if branch.MergeOnly {
				pushLevel = gitlab.NoPermissions
				mergeLevel = gitlab.DeveloperPermissions
			}

			err := c.protectSingleBranch(project, branch.Name, pushLevel, mergeLevel)
			if err != nil {
				if spin {
					err := spinner.StopFail()
					if err != nil {
						log.Debug().Err(err).Msg("cannot stop spinner")
					}
				}
				return err
			}
		}

		if spin {
			spinner.StopMessage(aurora.Sprintf(aurora.Green("ok")))
			if err := spinner.Stop(); err != nil {
				log.Debug().Err(err).Msg("cannot stop spinner")
			}
		}
	}

	return nil
}

func hasProtectedBranches(branches []config.BranchRule) bool {
	for _, branch := range branches {
		if branch.Protect || branch.MergeOnly {
			return true
		}
	}

	return false
}

func (c *Client) protectSingleBranch(
	project *gitlab.Project,
	branch string,
	pushAccessLevel gitlab.AccessLevelValue,
	mergeAccessLevel gitlab.AccessLevelValue,
) error {
	existing, _, err := c.ProtectedBranches.GetProtectedBranch(project.ID, branch)
	if err == nil {
		updateOpts := &gitlab.UpdateProtectedBranchOptions{
			AllowedToPush:      replaceBranchPermissions(existing.PushAccessLevels, pushAccessLevel),
			AllowedToMerge:     replaceBranchPermissions(existing.MergeAccessLevels, mergeAccessLevel),
			AllowedToUnprotect: replaceBranchPermissions(existing.UnprotectAccessLevels, gitlab.MaintainerPermissions),
		}

		_, _, err = c.ProtectedBranches.UpdateProtectedBranch(project.ID, branch, updateOpts)
		if err != nil {
			log.Debug().Err(err).
				Str("name", project.Name).
				Str("toURL", project.SSHURLToRepo).
				Str("branch", branch).
				Msg("cannot update protected branch")
			return fmt.Errorf("error while trying to update protected branch %s: %w", branch, err)
		}

		return nil
	}

	if !isProtectedBranchNotFoundError(err) {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", branch).
			Msg("cannot read protected branch")
		return fmt.Errorf("error while trying to read protected branch %s: %w", branch, err)
	}

	opts := &gitlab.ProtectRepositoryBranchesOptions{
		Name:                 gitlab.Ptr(branch),
		PushAccessLevel:      gitlab.Ptr(pushAccessLevel),
		MergeAccessLevel:     gitlab.Ptr(mergeAccessLevel),
		UnprotectAccessLevel: gitlab.Ptr(gitlab.MaintainerPermissions),
	}

	_, _, err = c.ProtectedBranches.ProtectRepositoryBranches(project.ID, opts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", branch).
			Msg("error while protecting branch")
		return fmt.Errorf("error while trying to protect branch %s: %w", branch, err)
	}

	return nil
}

func replaceBranchPermissions(existing []*gitlab.BranchAccessDescription, accessLevel gitlab.AccessLevelValue) *[]*gitlab.BranchPermissionOptions {
	destroy := true
	permissions := make([]*gitlab.BranchPermissionOptions, 0, len(existing)+1)
	for _, level := range existing {
		if level == nil || level.ID <= 0 {
			continue
		}

		id := level.ID
		permissions = append(permissions, &gitlab.BranchPermissionOptions{ID: &id, Destroy: &destroy})
	}

	permissions = append(permissions, &gitlab.BranchPermissionOptions{AccessLevel: gitlab.Ptr(accessLevel)})
	return &permissions
}

func isProtectedBranchNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}

func (c *Client) protectToBranchPerStudent(assignmentCfg *config.AssignmentConfig) {
	if len(assignmentCfg.Students) == 0 {
		fmt.Println("no students in config for assignment found")
		return
	}

	for _, student := range assignmentCfg.Students {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForStudent(student))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			fmt.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		if err := c.protectBranch(assignmentCfg, project, true); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot protect the branch")
		}
	}
}

func (c *Client) protectToBranchPerGroup(assignmentCfg *config.AssignmentConfig) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		projectname := fmt.Sprintf("%s/%s", assignmentCfg.Path, assignmentCfg.RepoNameForGroup(grp))
		project, _, err := c.Projects.GetProject(
			projectname,
			&gitlab.GetProjectOptions{},
		)
		if err != nil {
			fmt.Printf("cannot set access for project %s failed with %s", projectname, err)
			return
		}
		if err := c.protectBranch(assignmentCfg, project, true); err != nil {
			log.Error().Err(err).Str("group", assignmentCfg.Course).Msg("cannot protect the branch")
		}
	}
}
