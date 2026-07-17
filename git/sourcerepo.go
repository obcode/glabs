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
	"github.com/obcode/glabs/v3/reporter"
	"github.com/spf13/viper"
)

func PrepareSourceRepo(rep reporter.Reporter, url, fromBranch string, singleCommit bool, commitMessage string) (*SourceRepo, error) {
	task := rep.Task(aurora.Sprintf(aurora.Cyan(" cloning source code from %s, branch %s"),
		aurora.Yellow(url),
		aurora.Yellow(fromBranch),
	))
	fail := func(err error) (*SourceRepo, error) {
		task.Fail(fmt.Sprintf("problem: %v", err))
		return nil, err
	}

	// The starter-code (or deferred-branch) URL may be written in SSH notation
	// (git@host:path.git); glabs clones over HTTPS with the token, so normalize
	// it first, then pick the credential based on its host.
	cloneURL, err := config.HTTPSCloneURL(url)
	if err != nil {
		return fail(err)
	}

	auth, err := AuthForURL(cloneURL)
	if err != nil {
		return fail(err)
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
		return fail(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fail(err)
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: sourceRef,
		Force:  true,
	}); err != nil {
		return fail(err)
	}

	if !singleCommit {
		task.Done("")
		return &SourceRepo{
			Repo: repo,
			Ref:  sourceRef,
			Auth: auth,
		}, nil
	}

	headRef, err := repo.Head()
	if err != nil {
		return fail(err)
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return fail(err)
	}

	tree, err := headCommit.Tree()
	if err != nil {
		return fail(err)
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
		return fail(err)
	}

	commitHash, err := repo.Storer.SetEncodedObject(encoded)
	if err != nil {
		return fail(err)
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, commitHash)); err != nil {
		return fail(err)
	}

	if err := repo.CreateBranch(&gitconfig.Branch{
		Name:  refName.Short(),
		Merge: refName,
	}); err != nil && err != git.ErrBranchExists {
		return fail(err)
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: refName,
		Force:  true,
	}); err != nil {
		return fail(err)
	}

	task.Done(fmt.Sprintf("using branch '%s' with single commit '%s'", refName.Short(), commitMessage))

	return &SourceRepo{
		Repo: repo,
		Ref:  refName,
		Auth: auth,
	}, nil
}
