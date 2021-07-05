package git

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
)

type Starterrepo struct {
	Repo *git.Repository
	Auth ssh.AuthMethod
}

func PrepareStartercodeRepo(assignmentCfg *config.AssignmentConfig) (*Starterrepo, error) {
	if assignmentCfg.Startercode == nil {
		log.Debug().
			Str("course", assignmentCfg.Course).
			Str("assignment", assignmentCfg.Name).
			Msg("no startercode provided")
		return nil, nil
	}

	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" cloning startercode from %s, branch %s"),
			aurora.Yellow(assignmentCfg.Startercode.URL),
			aurora.Yellow(assignmentCfg.Startercode.FromBranch),
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

	return &Starterrepo{
		Repo: r,
		Auth: auth,
	}, nil
}
