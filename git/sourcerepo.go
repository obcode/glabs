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
)

// Committer identifies who authors the starter-code commit. Empty fields fall
// back to the glabs bot identity, so a caller that does not care (the common
// case) can pass the zero value. The web server passes the acting user.
type Committer struct {
	Name  string
	Email string
}

func (c Committer) signature(when time.Time) object.Signature {
	name, email := c.Name, c.Email
	if name == "" {
		name = "glabs"
	}
	if email == "" {
		email = "glabs-bot@noreply.example.com"
	}
	return object.Signature{Name: name, Email: email, When: when}
}

// PrepareSourceRepo clones the starter code into memory and returns it ready to
// push. auth resolves the clone credential from an explicit host+token (no
// viper), and committer authors the squashed single commit — both injected so
// the web server can act as a specific user.
func PrepareSourceRepo(rep reporter.Reporter, auth TokenAuth, committer Committer, url, fromBranch string, singleCommit bool, commitMessage string) (*SourceRepo, error) {
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

	resolvedAuth, err := auth.forURL(cloneURL)
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
		Auth:          resolvedAuth,
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
			Auth: resolvedAuth,
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

	now := time.Now()
	sig := committer.signature(now)
	commit := &object.Commit{
		Author:    sig,
		Committer: sig,
		Message:   commitMessage,
		TreeHash:  tree.Hash,
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
		Auth: resolvedAuth,
	}, nil
}
