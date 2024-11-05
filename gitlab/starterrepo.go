package gitlab

import (
	"fmt"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	cfg "github.com/obcode/glabs/config"
	g "github.com/obcode/glabs/git"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) pushStartercode(assignmentCfg *cfg.AssignmentConfig, from *g.Starterrepo, project *gitlab.Project) error {
	conf := &config.RemoteConfig{
		Name: project.Name,
		URLs: []string{project.SSHURLToRepo},
	}

	remote, err := from.Repo.CreateRemote(conf)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.SSHURLToRepo).
			Msg("cannot create remote")
		return fmt.Errorf("cannot create remote: %w", err)
	}

	refSpec := config.RefSpec(
		fmt.Sprintf("+refs/heads/%s:refs/heads/%s",
			assignmentCfg.Startercode.FromBranch,
			assignmentCfg.Startercode.ToBranch),
	)

	log.Debug().
		Str("refSpec", string(refSpec)).
		Str("name", project.Name).
		Str("toURL", project.SSHURLToRepo).
		Str("fromBranch", assignmentCfg.Startercode.FromBranch).
		Str("toBranch", assignmentCfg.Startercode.ToBranch).
		Str("devBranch", assignmentCfg.Startercode.DevBranch).
		Msg("pushing starter code")

	pushOpts := &git.PushOptions{
		RemoteName: remote.Config().Name,
		RefSpecs:   []config.RefSpec{refSpec},
		Auth:       from.Auth,
	}
	err = from.Repo.Push(pushOpts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.SSHURLToRepo).
			Msg("cannot push to remote")
		return fmt.Errorf("cannot push to remote: %w", err)
	}

	if assignmentCfg.Startercode.DevBranch != assignmentCfg.Startercode.ToBranch {
		if err := c.devBranch(assignmentCfg, project); err != nil {
			log.Debug().Err(err).
				Str("name", project.Name).
				Msg("cannot set dev branch")
		}
	}

	if assignmentCfg.Startercode.ProtectToBranch {
		if err := c.protectBranch(assignmentCfg, project, false); err != nil {
			log.Debug().Err(err).
				Str("name", project.Name).
				Msg("cannot protect to branch")
		}
	}

	for _, additionalBranch := range assignmentCfg.Startercode.AdditionalBranches {
		log.Debug().Str("branch", additionalBranch).Msg("pushing additional branch")

		// worktree, err := from.Repo.Worktree()
		// if err != nil {
		// 	log.Debug().Err(err).
		// 		Str("branch", additionalBranch).
		// 		Str("name", project.Name).Str("url", project.SSHURLToRepo).
		// 		Msg("cannot get worktree")
		// 	return fmt.Errorf("cannot get worktree: %w", err)
		// }

		// worktree.Checkout(&git.CheckoutOptions{
		// 	Branch: plumbing.ReferenceName(additionalBranch),
		// })

		refSpec := config.RefSpec(fmt.Sprintf("+refs/remotes/origin/%s:refs/heads/%s", additionalBranch, additionalBranch))

		pushOpts := &git.PushOptions{
			RemoteName: remote.Config().Name,
			RefSpecs:   []config.RefSpec{refSpec},
			Auth:       from.Auth,
		}
		err = from.Repo.Push(pushOpts)
		if err != nil {
			log.Debug().Err(err).
				Str("branch", additionalBranch).
				Str("refspec", refSpec.String()).
				Str("name", project.Name).Str("url", project.SSHURLToRepo).
				Msg("cannot push to remote")
			return fmt.Errorf("cannot push to remote: %w", err)
		}

	}

	return nil
}

func (c *Client) devBranch(assignmentCfg *cfg.AssignmentConfig, project *gitlab.Project) error {
	log.Debug().
		Str("name", project.Name).
		Str("toURL", project.SSHURLToRepo).
		Str("branch", assignmentCfg.Startercode.DevBranch).
		Msg("switching to development branch")

	opts := &gitlab.CreateBranchOptions{
		Branch: gitlab.Ptr(assignmentCfg.Startercode.DevBranch),
		Ref:    gitlab.Ptr(assignmentCfg.Startercode.ToBranch),
	}

	_, _, err := c.Branches.CreateBranch(project.ID, opts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", assignmentCfg.Startercode.DevBranch).
			Msg("error creating development branch")
		return fmt.Errorf("error while trying to create development branch: %w", err)
	}

	projectOpts := &gitlab.EditProjectOptions{
		DefaultBranch: gitlab.Ptr(assignmentCfg.Startercode.DevBranch),
	}

	_, _, err = c.Projects.EditProject(project.ID, projectOpts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", assignmentCfg.Startercode.DevBranch).
			Msg("error switching default to development branch")
		return fmt.Errorf("error while switching default to development branch: %w", err)
	}

	return nil
}
