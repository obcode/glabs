package git

import (
	"fmt"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/reporter"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func Push(rep reporter.Reporter, assignmentCfg *config.AssignmentConfig, projectname string, sourceRepo *SourceRepo, toBranch string, force bool, project *gitlab.Project) error {
	task := rep.Task(aurora.Sprintf(aurora.Cyan(" pushing branch %s to project %s / branch %s"),
		aurora.Yellow(sourceRepo.Ref.Short()),
		aurora.Magenta(assignmentCfg.URL+"/"+project.Name),
		aurora.Magenta(toBranch),
	))

	conf := &gitconfig.RemoteConfig{
		Name: project.Name,
		URLs: []string{project.HTTPURLToRepo},
	}

	remote, err := sourceRepo.Repo.CreateRemote(conf)
	if err != nil {
		task.Fail(fmt.Sprintf("problem: %v", err))
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.HTTPURLToRepo).
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

	if err := sourceRepo.Repo.Push(pushOpts); err != nil {
		task.Fail(fmt.Sprintf("problem: %v", err))
		return err
	}
	task.Done("")
	return nil
}
