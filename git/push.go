package git

import (
	"fmt"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v2/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func Push(assignmentCfg *config.AssignmentConfig, projectname string, sourceRepo *SourceRepo, toBranch string, force bool, project *gitlab.Project) error {
	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" pushing branch %s to project %s / branch %s"),
			aurora.Yellow(sourceRepo.Ref.Short()),
			aurora.Magenta(assignmentCfg.URL+"/"+project.Name),
			aurora.Magenta(toBranch),
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

	conf := &gitconfig.RemoteConfig{
		Name: project.Name,
		URLs: []string{project.SSHURLToRepo},
	}

	remote, err := sourceRepo.Repo.CreateRemote(conf)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.SSHURLToRepo).
			Msg("cannot create remote")
		return fmt.Errorf("cannot create remote: %w", err)
	}

	spec := sourceRepo.Ref.String() + ":" + plumbing.NewBranchReferenceName(toBranch).String()
	if force {
		spec = "+" + spec
	}

	pushOpts := &git.PushOptions{
		RemoteName: remote.Config().Name,
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec(spec)},
		Auth:       sourceRepo.Auth,
	}

	err = sourceRepo.Repo.Push(pushOpts)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return err
	}
	err = spinner.Stop()
	if err != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}
	return nil
}
