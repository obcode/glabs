package git

import (
	"fmt"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/logrusorgru/aurora"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
)

func PrepareSourceRepo(url, fromBranch string) (*SourceRepo, error) {
	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" cloning source code from %s, branch %s"),
			aurora.Yellow(url),
			aurora.Yellow(fromBranch),
		),
		SuffixAutoColon:   true,
		StopCharacter:     "✓",
		StopColors:        []string{"fgGreen"},
		StopFailMessage:   "error",
		StopFailCharacter: "✗",
		StopFailColors:    []string{"fgRed"},
	}

	spinner, err := yacspin.New(cfg)
	if err != nil {
		log.Debug().Err(err).Msg("cannot create spinner")
	}
	err = spinner.Start()
	if err != nil {
		log.Debug().Err(err).Msg("cannot start spinner")
	}

	auth, err := GetAuth()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		Auth:          auth,
		URL:           url,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + fromBranch),
	})

	errs := spinner.Stop()
	if errs != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}

	if err != nil {
		return nil, fmt.Errorf("error while cloning repo (wrong URL or no rights?): %w", err)
	}

	return &SourceRepo{
		Repo: r,
		Ref:  plumbing.ReferenceName("refs/heads/" + fromBranch),
		Auth: auth,
	}, nil
}
