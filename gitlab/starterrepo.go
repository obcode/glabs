package gitlab

import (
	"fmt"

	"github.com/go-git/go-git/v5"
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

	refSpec := config.RefSpec("refs/heads/" + assignmentCfg.Startercode.FromBranch +
		":refs/heads/" + assignmentCfg.Startercode.ToBranch)

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
		return c.devBranch(assignmentCfg, project)
	}

	if assignmentCfg.Startercode.ProtectToBranch {
		return c.protectBranch(assignmentCfg, project)
	}

	return nil
}

func (c *Client) devBranch(assignmentCfg *cfg.AssignmentConfig, project *gitlab.Project) error {
	if assignmentCfg.Startercode.ProtectToBranch {
		log.Debug().
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", assignmentCfg.Startercode.DevBranch).
			Msg("switching to development branch")

		opts := &gitlab.CreateBranchOptions{
			Branch: gitlab.String(assignmentCfg.Startercode.DevBranch),
			Ref:    gitlab.String(assignmentCfg.Startercode.ToBranch),
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
			DefaultBranch: gitlab.String(assignmentCfg.Startercode.DevBranch),
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
	}

	return nil
}

func (c *Client) protectBranch(assignmentCfg *cfg.AssignmentConfig, project *gitlab.Project) error {
	if assignmentCfg.Startercode.ProtectToBranch {
		log.Debug().
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", assignmentCfg.Startercode.ToBranch).
			Msg("protecting branch")

		opts := &gitlab.ProtectRepositoryBranchesOptions{
			Name: gitlab.String(assignmentCfg.Startercode.ToBranch),
		}

		_, _, err := c.ProtectedBranches.ProtectRepositoryBranches(project.ID, opts)
		if err != nil {
			log.Debug().Err(err).
				Str("name", project.Name).
				Str("toURL", project.SSHURLToRepo).
				Str("branch", assignmentCfg.Startercode.ToBranch).
				Msg("error while protecting branch")
			return fmt.Errorf("error while trying to protect branch: %w", err)
		}
	}

	return nil
}
