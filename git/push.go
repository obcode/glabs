package git

import (
	"fmt"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v2/config"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func CloneBranch(url, fromBranch string, orphan bool, orphanMessage string) (*git.Repository, plumbing.ReferenceName, error) {
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
		fmt.Printf("error: %v", err)
		return nil, "", err
	}

	storer := memory.NewStorage()
	fs := memfs.New()

	sourceRef := plumbing.NewBranchReferenceName(fromBranch)

	repo, err := git.Clone(storer, fs, &git.CloneOptions{
		URL:           url,
		ReferenceName: sourceRef,
		SingleBranch:  true,
		Auth:          auth,
	})
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	wt, err := repo.Worktree()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: sourceRef,
		Force:  true,
	}); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	if !orphan {
		return repo, sourceRef, nil
	}

	headRef, err := repo.Head()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	tree, err := headCommit.Tree()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	orphanBranchName := fmt.Sprintf("orphan-%s-%d", fromBranch, time.Now().UnixNano())
	orphanRef := plumbing.NewBranchReferenceName(orphanBranchName)

	now := time.Now()
	commit := &object.Commit{
		Author: object.Signature{
			Name:  "glabs",
			Email: "noreply@example.com",
			When:  now,
		},
		Committer: object.Signature{
			Name:  "glabs",
			Email: "noreply@example.com",
			When:  now,
		},
		Message:  orphanMessage,
		TreeHash: tree.Hash,
	}

	encoded := repo.Storer.NewEncodedObject()
	if err := commit.Encode(encoded); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	commitHash, err := repo.Storer.SetEncodedObject(encoded)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(orphanRef, commitHash)); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	if err := repo.CreateBranch(&gitconfig.Branch{
		Name:  orphanRef.Short(),
		Merge: orphanRef,
	}); err != nil && err != git.ErrBranchExists {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: orphanRef,
		Force:  true,
	}); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, "", err
	}

	if orphan {
		spinner.StopMessage(fmt.Sprintf("using branch '%s' with single commit '%s'", orphanRef.Short(), orphanMessage))
	}
	errs := spinner.Stop()
	if errs != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}

	return repo, orphanRef, nil
}

func PushBranch(assignmentCfg *config.AssignmentConfig, projectname string, repo *git.Repository, localRef plumbing.ReferenceName, toBranch string, force bool, project *gitlab.Project) error {
	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" pushing branch %s to project %s / branch %s"),
			aurora.Yellow(localRef.Short()),
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

	remote, err := repo.CreateRemote(conf)
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

	auth, err := GetAuth()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		fmt.Printf("error: %v", err)
		return err
	}

	spec := localRef.String() + ":" + plumbing.NewBranchReferenceName(toBranch).String()
	if force {
		spec = "+" + spec
	}

	pushOpts := &git.PushOptions{
		RemoteName: remote.Config().Name,
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec(spec)},
		Auth:       auth,
	}

	err = repo.Push(pushOpts)
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
