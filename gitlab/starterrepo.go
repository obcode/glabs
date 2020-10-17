package gitlab

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

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

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		Auth:     publicKeys,
		URL:      url,
		Progress: os.Stdout,
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

func pushStartercode(from *starterrepo, toName, toURL string) {
	conf := &config.RemoteConfig{
		Name: toName,
		URLs: []string{toURL},
	}

	remote, err := from.repo.CreateRemote(conf)
	if err != nil {
		log.Fatal().Err(err).
			Str("name", toName).Str("url", toURL).
			Msg("cannot create remote")
	}

	pushOpts := &git.PushOptions{
		RemoteName: remote.Config().Name,
		Auth:       from.publickeys,
	}
	err = from.repo.Push(pushOpts)
	if err != nil {
		log.Fatal().Err(err).
			Str("name", toName).Str("url", toURL).
			Msg("cannot push to remote")
	}
}
