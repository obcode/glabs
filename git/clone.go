package git

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
)

func Clone(cfg *config.AssignmentConfig) {
	auth, err := getAuth()
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	switch cfg.Per {
	case config.PerStudent:
		for _, suffix := range cfg.Students {
			clone(localpath(cfg, suffix), cfg.Clone.Branch, cloneurl(cfg, suffix), auth)
		}
	case config.PerGroup:
		for _, grp := range cfg.Groups {
			clone(localpath(cfg, grp.Name), cfg.Clone.Branch, cloneurl(cfg, grp.Name), auth)
		}
	}
}

func cloneurl(cfg *config.AssignmentConfig, suffix string) string {
	return fmt.Sprintf("%s/%s-%s",
		strings.Replace(strings.Replace(cfg.URL, "https://", "git@", 1), "/", ":", 1),
		cfg.Name, suffix)
}

func localpath(cfg *config.AssignmentConfig, suffix string) string {
	return fmt.Sprintf("%s/%s-%s", cfg.Clone.LocalPath, cfg.Name, suffix)
}

func clone(localpath, branch, cloneurl string, auth ssh.AuthMethod) {
	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" cloning %s to %s branch %s"),
			aurora.Yellow(cloneurl),
			aurora.Yellow(localpath),
			aurora.Yellow(branch),
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

	_, err = git.PlainClone(localpath, false, &git.CloneOptions{
		Auth:          auth,
		URL:           cloneurl,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + branch),
	})

	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return
	}

	errs := spinner.Stop()
	if errs != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}
}
