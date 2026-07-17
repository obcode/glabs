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
	"github.com/obcode/glabs/v3/config"
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

	cloneSucceeded := false
	defer func() {
		if spinner == nil {
			return
		}

		if cloneSucceeded {
			errs := spinner.Stop()
			if errs != nil {
				log.Debug().Err(errs).Msg("cannot stop spinner")
			}
			return
		}

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
	}()

	// The starter-code (or deferred-branch) URL may be written in SSH notation
	// (git@host:path.git); glabs clones over HTTPS with the token, so normalize
	// it first, then pick the credential based on its host.
	cloneURL, err := config.HTTPSCloneURL(url)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	auth, err := AuthForURL(cloneURL)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		fmt.Printf("error: %v", err)
		return nil, err
	}

	storer := memory.NewStorage()
	fs := memfs.New()

	sourceRef := plumbing.NewBranchReferenceName(fromBranch)

	repo, err := git.Clone(storer, fs, &git.CloneOptions{
		URL:           cloneURL,
		ReferenceName: sourceRef,
		SingleBranch:  true,
		Auth:          auth,
	})
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: sourceRef,
		Force:  true,
	}); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	if !singleCommit {
		cloneSucceeded = true
		return &SourceRepo{
			Repo: repo,
			Ref:  sourceRef,
			Auth: auth,
		}, nil
	}

	headRef, err := repo.Head()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	tree, err := headCommit.Tree()
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	singleCommitBranchName := fmt.Sprintf("orphan-%s-%d", sourceRef.Short(), time.Now().UnixNano())
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
		return nil, err
	}

	commitHash, err := repo.Storer.SetEncodedObject(encoded)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, commitHash)); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	if err := repo.CreateBranch(&gitconfig.Branch{
		Name:  refName.Short(),
		Merge: refName,
	}); err != nil && err != git.ErrBranchExists {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: refName,
		Force:  true,
	}); err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	if singleCommit {
		spinner.StopMessage(fmt.Sprintf("using branch '%s' with single commit '%s'", refName.Short(), commitMessage))
	}
	cloneSucceeded = true

	return &SourceRepo{
		Repo: repo,
		Ref:  refName,
		Auth: auth,
	}, nil
}
