package gitlab

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	cfg "github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

type starterrepo struct {
	repo       *git.Repository
	publickeys *ssh.PublicKeys
}

func prepareStartercodeRepo(assignmentCfg *cfg.AssignmentConfig) *starterrepo {
	if assignmentCfg.Startercode == nil {
		log.Debug().
			Str("course", assignmentCfg.Course).
			Str("assignment", assignmentCfg.Name).
			Msg("no startercode provided")
		return nil
	}

	privateKeyFile := fmt.Sprintf("%s/.ssh/id_rsa", os.Getenv("HOME"))
	// if pkf := startercode["privatekeyfile"]; pkf != "" {
	// 	privateKeyFile = pkf
	// }

	log.Debug().Str("privatekeyfile", privateKeyFile).Msg("using private key from file")

	log.Debug().Str("url", assignmentCfg.Startercode.URL).Msg("using startercode from url")

	_, err := os.Stat(privateKeyFile)
	if err != nil {
		log.Error().Err(err).Str("file", privateKeyFile).Msg("read file failed")
		return nil
	}

	publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		log.Error().Err(err).Str("file", privateKeyFile).Msg("generate publickeys failed")
		return nil
	}

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		Auth:          publicKeys,
		URL:           assignmentCfg.Startercode.URL,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + assignmentCfg.Startercode.FromBranch),
		Progress:      os.Stdout,
	})

	if err != nil {
		log.Error().Err(err).
			Msg("error while preparing starterrepo")
	}

	return &starterrepo{
		repo:       r,
		publickeys: publicKeys,
	}
}

func (c *Client) pushStartercode(assignmentCfg *cfg.AssignmentConfig, from *starterrepo, project *gitlab.Project) {
	conf := &config.RemoteConfig{
		Name: project.Name,
		URLs: []string{project.SSHURLToRepo},
	}

	remote, err := from.repo.CreateRemote(conf)
	if err != nil {
		log.Fatal().Err(err).
			Str("name", project.Name).Str("url", project.SSHURLToRepo).
			Msg("cannot create remote")
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
		Auth:       from.publickeys,
	}
	err = from.repo.Push(pushOpts)
	if err != nil {
		log.Fatal().Err(err).
			Str("name", project.Name).Str("url", project.SSHURLToRepo).
			Msg("cannot push to remote")
	}

	c.protectBranch(assignmentCfg, project)
}

func (c *Client) protectBranch(assignmentCfg *cfg.AssignmentConfig, project *gitlab.Project) {
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
			log.Error().Err(err).
				Str("name", project.Name).
				Str("toURL", project.SSHURLToRepo).
				Str("branch", assignmentCfg.Startercode.ToBranch).
				Msg("error while protecting branch")
		}
	}
}
