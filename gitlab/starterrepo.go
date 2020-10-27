package gitlab

import (
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/logrusorgru/aurora"
	cfg "github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	"github.com/xanzy/go-gitlab"
)

type starterrepo struct {
	repo       *git.Repository
	publickeys *ssh.PublicKeys
}

func prepareStartercodeRepo(assignmentCfg *cfg.AssignmentConfig) (*starterrepo, error) {
	if assignmentCfg.Startercode == nil {
		log.Debug().
			Str("course", assignmentCfg.Course).
			Str("assignment", assignmentCfg.Name).
			Msg("no startercode provided")
		return nil, nil
	}

	privateKeyFile := fmt.Sprintf("%s/.ssh/id_rsa", os.Getenv("HOME"))
	// if pkf := startercode["privatekeyfile"]; pkf != "" {
	// 	privateKeyFile = pkf
	// }

	_, err := os.Stat(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read ssh key from file: %w", err)
	}

	publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		return nil, fmt.Errorf("cannot generate publickeys from file %s:  %w", privateKeyFile, err)
	}

	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" cloning startercode from %s, branch %s"),
			aurora.Yellow(assignmentCfg.Startercode.URL),
			aurora.Yellow(assignmentCfg.Startercode.FromBranch),
		),
		SuffixAutoColon: true,
		StopCharacter:   "✓",
		StopColors:      []string{"fgGreen"},
	}

	spinner, err := yacspin.New(cfg)
	if err != nil {
		log.Debug().Err(err).Msg("cannot create spinner")
	}
	err = spinner.Start()
	if err != nil {
		log.Debug().Err(err).Msg("cannot start spinner")
	}

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		Auth:          publicKeys,
		URL:           assignmentCfg.Startercode.URL,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + assignmentCfg.Startercode.FromBranch),
	})

	errs := spinner.Stop()
	if errs != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}

	if err != nil {
		return nil, fmt.Errorf("error while cloning repo (wrong URL or no rights?): %w", err)
	}

	return &starterrepo{
		repo:       r,
		publickeys: publicKeys,
	}, nil
}

func (c *Client) pushStartercode(assignmentCfg *cfg.AssignmentConfig, from *starterrepo, project *gitlab.Project) error {
	conf := &config.RemoteConfig{
		Name: project.Name,
		URLs: []string{project.SSHURLToRepo},
	}

	remote, err := from.repo.CreateRemote(conf)
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
		Auth:       from.publickeys,
	}
	err = from.repo.Push(pushOpts)
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
