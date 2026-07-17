package git

import (
	"fmt"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/reporter"
)

// Clone clones every student/group repository to disk. It is CLI-only — the web
// server never writes to a local working directory. --suppress passes a discard
// reporter so only the machine-readable paths are printed.
func Clone(rep reporter.Reporter, cfg *config.AssignmentConfig) {
	auth, err := GetAuth()
	if err != nil {
		rep.Printf("error: %v", err)
		return
	}

	switch cfg.Per {
	case config.PerStudent:
		for _, stud := range cfg.Students {
			suffix := cfg.RepoSuffix(stud)
			clone(rep, localpath(cfg, suffix), cfg.Clone.Branch, ProjectRepoUrl(cfg, suffix), auth, cfg.Clone.Force)
		}
	case config.PerGroup:
		for _, grp := range cfg.Groups {
			clone(rep, localpath(cfg, grp.Name), cfg.Clone.Branch, ProjectRepoUrl(cfg, grp.Name), auth, cfg.Clone.Force)
		}
	}
}

// ProjectRepoUrl is the HTTPS clone URL for a student's or group's repository.
// cfg.URL is already https://host/coursepath, so the repository URL is just that
// plus the repo name; glabs clones it over HTTPS with the token.
func ProjectRepoUrl(cfg *config.AssignmentConfig, suffix string) string {
	return fmt.Sprintf("%s/%s-%s.git", cfg.URL, cfg.RepoBaseName(), suffix)
}

func localpath(cfg *config.AssignmentConfig, suffix string) string {
	return fmt.Sprintf("%s/%s", cfg.Clone.LocalPath, cfg.RepoNameWithSuffix(suffix))
}

func clone(rep reporter.Reporter, localpath, branch, cloneurl string, auth transport.AuthMethod, force bool) {
	task := rep.Task(aurora.Sprintf(aurora.Cyan(" cloning %s to %s branch %s"),
		aurora.Yellow(cloneurl),
		aurora.Yellow(localpath),
		aurora.Yellow(branch),
	))

	if force {
		task.Update(" trying to remove folder if it exists")
		if err := os.RemoveAll(localpath); err != nil {
			task.Fail(fmt.Sprintf("error when trying to remove %s: %v", localpath, err))
			return
		}
		task.Update(" cloning")
	}

	_, err := git.PlainClone(localpath, false, &git.CloneOptions{
		Auth:          auth,
		URL:           cloneurl,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + branch),
	})
	if err != nil {
		task.Fail(fmt.Sprintf("problem: %v", err))
		return
	}

	task.Done("")
	// Always printed, even under --suppress: this is the machine-readable output
	// meant for piping.
	fmt.Println(localpath)
}
