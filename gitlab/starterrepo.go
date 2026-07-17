package gitlab

import (
	"fmt"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	cfg "github.com/obcode/glabs/v3/config"
	g "github.com/obcode/glabs/v3/git"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) pushStartercode(assignmentCfg *cfg.AssignmentConfig, from *g.SourceRepo, project *gitlab.Project) error {
	conf := &config.RemoteConfig{
		Name: project.Name,
		URLs: []string{project.HTTPURLToRepo},
	}

	remote, err := from.Repo.CreateRemote(conf)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.HTTPURLToRepo).
			Msg("cannot create remote")
		return fmt.Errorf("cannot create remote: %w", err)
	}

	refSpec := config.RefSpec(
		fmt.Sprintf("+refs/heads/%s:refs/heads/%s",
			from.Ref.Short(),
			assignmentCfg.Startercode.ToBranch),
	)

	log.Debug().
		Str("refSpec", string(refSpec)).
		Str("name", project.Name).
		Str("toURL", project.HTTPURLToRepo).
		Str("fromBranch", from.Ref.Short()).
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
			Str("name", project.Name).Str("url", project.HTTPURLToRepo).
			Msg("cannot push to remote")
		return fmt.Errorf("cannot push to remote: %w", err)
	}

	tagName := strings.TrimSpace(assignmentCfg.Startercode.Tag)
	if tagName != "" {
		sourceRef, err := from.Repo.Reference(from.Ref, true)
		if err != nil {
			return fmt.Errorf("cannot resolve source reference for tag %q: %w", tagName, err)
		}

		tagRef := plumbing.NewTagReferenceName(tagName)
		if err := from.Repo.Storer.SetReference(plumbing.NewHashReference(tagRef, sourceRef.Hash())); err != nil {
			return fmt.Errorf("cannot set local tag %q: %w", tagName, err)
		}

		tagRefSpec := config.RefSpec(fmt.Sprintf("+%s:%s", tagRef.String(), tagRef.String()))
		log.Debug().
			Str("refSpec", string(tagRefSpec)).
			Str("name", project.Name).
			Str("toURL", project.HTTPURLToRepo).
			Str("tag", tagName).
			Msg("pushing startercode tag")

		err = from.Repo.Push(&git.PushOptions{
			RemoteName: remote.Config().Name,
			RefSpecs:   []config.RefSpec{tagRefSpec},
			Auth:       from.Auth,
		})
		if err != nil {
			return fmt.Errorf("cannot push startercode tag %q to remote: %w", tagName, err)
		}
	}

	for _, additionalBranch := range assignmentCfg.Startercode.AdditionalBranches {
		if additionalBranch == "" {
			continue
		}

		refSpec := config.RefSpec(fmt.Sprintf("+refs/remotes/origin/%s:refs/heads/%s", additionalBranch, additionalBranch))

		log.Debug().
			Str("refSpec", string(refSpec)).
			Str("name", project.Name).
			Str("toURL", project.HTTPURLToRepo).
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
				Str("url", project.HTTPURLToRepo).
				Msg("cannot push additional branch to remote, continuing with other setup steps")
			continue
		}
	}

	return nil
}
