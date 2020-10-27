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
		Msg("pushing starter code")

	pushOpts := &git.PushOptions{
		RemoteName: remote.Config().Name,
		RefSpecs:   []config.RefSpec{refSpec},
		Auth:       from.Publickeys,
	}
	err = from.Repo.Push(pushOpts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.SSHURLToRepo).
			Msg("cannot push to remote")
		return fmt.Errorf("cannot push to remote: %w", err)
	}

	if assignmentCfg.Startercode.ProtectToBranch {
		return c.protectBranch(assignmentCfg, project)
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
