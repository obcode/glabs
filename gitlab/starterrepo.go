package gitlab

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/xanzy/go-gitlab"
)

const master = "master"

type starterrepo struct {
	repo       *git.Repository
	publickeys *ssh.PublicKeys
}

func prepareStartercodeRepo(course, assignment string) *starterrepo {
	startercodeKey := course + "." + assignment + ".startercode"
	startercode := viper.GetStringMapString(startercodeKey)

	if len(startercode) == 0 {
		log.Debug().Str("course", course).Str("assignment", assignment).Msg("no startercode provided")
		return nil
	}

	privateKeyFile := fmt.Sprintf("%s/.ssh/id_rsa", os.Getenv("HOME"))
	if pkf := startercode["privatekeyfile"]; pkf != "" {
		privateKeyFile = pkf
	}

	log.Debug().Str("privatekeyfile", privateKeyFile).Msg("using private key from file")

	url, ok := startercode["url"]
	if !ok {
		log.Fatal().Err(errors.New("url for startercode not set")).
			Str("course", course).Str("assignment", assignment).
			Msg("url for startercode missing")
	}

	log.Debug().Str("url", url).Msg("using startercode from url")

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

	fromBranch := master
	if fB := viper.GetString(course + "." + assignment + ".startercode.fromBranch"); len(fB) > 0 {
		fromBranch = fB
	}

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		Auth:          publicKeys,
		URL:           url,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + fromBranch),
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

func (c *Client) pushStartercode(course, assignment string, from *starterrepo, project *gitlab.Project) {
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

	fromBranch := master
	if fB := viper.GetString(course + "." + assignment + ".startercode.fromBranch"); len(fB) > 0 {
		fromBranch = fB
	}

	toBranch := master
	if tB := viper.GetString(course + "." + assignment + ".startercode.toBranch"); len(tB) > 0 {
		toBranch = tB
	}

	refSpec := config.RefSpec("refs/heads/" + fromBranch + ":refs/heads/" + toBranch)

	log.Debug().
		Str("refSpec", string(refSpec)).
		Str("name", project.Name).
		Str("toURL", project.SSHURLToRepo).
		Str("fromBranch", fromBranch).
		Str("toBranch", toBranch).
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

	c.protectBranch(course, assignment, toBranch, project)
}

func (c *Client) protectBranch(course, assignment, toBranch string, project *gitlab.Project) {
	if viper.GetBool(course + "." + assignment + ".startercode.protectToBranch") {
		log.Debug().
			Str("name", project.Name).
			Str("toURL", project.SSHURLToRepo).
			Str("branch", toBranch).
			Msg("protecting branch")

		opts := &gitlab.ProtectRepositoryBranchesOptions{
			Name: gitlab.String(toBranch),
		}

		_, _, err := c.ProtectedBranches.ProtectRepositoryBranches(project.ID, opts)
		if err != nil {
			log.Error().Err(err).
				Str("name", project.Name).
				Str("toURL", project.SSHURLToRepo).
				Str("branch", toBranch).
				Msg("error while protecting branch")
		}
	}
}
