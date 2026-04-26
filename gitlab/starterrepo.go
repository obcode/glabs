package gitlab

import (
	"fmt"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	cfg "github.com/obcode/glabs/v2/config"
	g "github.com/obcode/glabs/v2/git"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
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

	for _, additionalBranch := range assignmentCfg.Startercode.AdditionalBranches {
		if additionalBranch == "" {
			continue
		}

		refSpec := config.RefSpec(fmt.Sprintf("+refs/remotes/origin/%s:refs/heads/%s", additionalBranch, additionalBranch))

		log.Debug().
			Str("refSpec", string(refSpec)).
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", additionalBranch).
			Msg("pushing additional startercode branch")

		pushOpts := &git.PushOptions{
			RemoteName: remote.Config().Name,
			RefSpecs:   []config.RefSpec{refSpec},
			Auth:       from.Auth,
		}
		err = from.Repo.Push(pushOpts)
		if err != nil {
			log.Warn().Err(err).
				Str("branch", additionalBranch).
				Str("refSpec", refSpec.String()).
				Str("name", project.Name).
				Str("url", project.SSHURLToRepo).
				Msg("cannot push additional branch to remote, continuing with other setup steps")
			continue
		}
	}

	return nil
}
