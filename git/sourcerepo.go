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
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/theckman/yacspin"
)

func PrepareSourceRepo(url, fromBranch string, singleCommit bool, commitMessage string) (*SourceRepo, error) {
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
		return nil, err
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
		return nil, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
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
		return nil, err
	}

	if !singleCommit {
		return &SourceRepo{
			Repo: repo,
			Ref:  sourceRef,
			Auth: auth,
		}, nil
	}

	headRef, err := repo.Head()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	tree, err := headCommit.Tree()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	singleCommitBranchName := fmt.Sprintf("orphan-%s-%d", fromBranch, time.Now().UnixNano())
	refName := plumbing.NewBranchReferenceName(singleCommitBranchName)

	committerName := "glabs"
	committerEmail := "glabs-bot@noreply.example.com"

	if viper.IsSet("committer") {
		committerName = viper.GetString("committer.name")
		committerEmail = viper.GetString("committer.email")
	}

	now := time.Now()
	commit := &object.Commit{
		Author: object.Signature{
			Name:  committerName,
			Email: committerEmail,
			When:  now,
		},
		Committer: object.Signature{
			Name:  committerName,
			Email: committerEmail,
			When:  now,
		},
		Message:  commitMessage,
		TreeHash: tree.Hash,
	}

	encoded := repo.Storer.NewEncodedObject()
	if err := commit.Encode(encoded); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	commitHash, err := repo.Storer.SetEncodedObject(encoded)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, commitHash)); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	if err := repo.CreateBranch(&gitconfig.Branch{
		Name:  refName.Short(),
		Merge: refName,
	}); err != nil && err != git.ErrBranchExists {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: refName,
		Force:  true,
	}); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil, err
	}

	if singleCommit {
		spinner.StopMessage(fmt.Sprintf("using branch '%s' with single commit '%s'", refName.Short(), commitMessage))
	}
	errs := spinner.Stop()
	if errs != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}

	return &SourceRepo{
		Repo: repo,
		Ref:  refName,
		Auth: auth,
	}, nil
}
