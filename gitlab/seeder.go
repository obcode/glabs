package gitlab

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/logrusorgru/aurora"
	cfg "github.com/obcode/glabs/v3/config"
	g "github.com/obcode/glabs/v3/git"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func localpath(cfg *cfg.AssignmentConfig, project string) string {
	return fmt.Sprintf("%s/%s", cfg.Clone.LocalPath, project)
}

func (c *Client) runSeeder(assignmentCfg *cfg.AssignmentConfig, project *gitlab.Project) error {

	// Deprecated. The seeder runs an arbitrary command from the config file; in a
	// shared/web context that is remote code execution, so it will not be offered
	// there and is slated for removal from the CLI too. No course file uses it.
	fmt.Fprintln(os.Stderr, aurora.Yellow(
		"warning: the seeder is deprecated and will be removed in a future release"))

	path := localpath(assignmentCfg, project.Name)

	err := os.Mkdir(path, 0755)
	if err != nil {
		log.Debug().Err(err).
			Msg("cannot create new directory for seeding")
		return err
	}
	path, _ = filepath.Abs(path)

	// Copy rather than write back into assignmentCfg.Seeder.Args: the same config
	// is reused for every project in the per-student/per-group loop, and
	// substituting %s in place would consume the placeholder after the first one.
	args := make([]string, len(assignmentCfg.Seeder.Args))
	for i, item := range assignmentCfg.Seeder.Args {
		if strings.Count(item, "%s") == 1 {
			args[i] = fmt.Sprintf(item, path)
		} else {
			args[i] = item
		}
	}

	cmd := exec.Command(assignmentCfg.Seeder.Command, args...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()

	log.Debug().Msg(fmt.Sprintf("seeder returned: %v ", string(out)))
	if err != nil {
		log.Debug().Err(err)
		return fmt.Errorf("running seeding application %s failed: %v", assignmentCfg.Seeder.Command, err)
	}

	_, err = git.PlainInit(path, false)
	if err != nil {
		log.Debug().Err(err).
			Msg("cannot initalize repository for seeding")
		return err
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		log.Debug().Err(err)
		return err
	}

	wtree, err := repo.Worktree()
	if err != nil {
		log.Debug().Err(err)
		return err
	}

	err = addAndCommit(wtree, assignmentCfg)
	if err != nil {
		return err
	}

	err = push(assignmentCfg, repo, wtree, project)
	if err != nil {
		return err
	}

	if assignmentCfg.Seeder.ProtectToBranch {
		opts := &gitlab.ProtectRepositoryBranchesOptions{
			Name: gitlab.Ptr(assignmentCfg.Seeder.ToBranch),
		}

		_, _, err = c.ProtectedBranches.ProtectRepositoryBranches(project.ID, opts)
		if err != nil {
			log.Debug().Err(err)
			return err
		}
	}
	err = os.RemoveAll(path)
	if err != nil {
		return err
	}
	return nil
}

func addAndCommit(wtree *git.Worktree, assignmentCfg *cfg.AssignmentConfig) error {
	err := wtree.AddWithOptions(&git.AddOptions{All: true, Glob: "*"})
	if err != nil {
		log.Debug().Err(err).
			Msg("cannot stage changes")
		return fmt.Errorf("cannot stage changes: %w", err)
	}

	_, err = wtree.Commit("Seeded repository using glabs",
		&git.CommitOptions{Author: &object.Signature{Name: assignmentCfg.Seeder.Name,
			Email: assignmentCfg.Seeder.EMail, When: time.Now()},
			SignKey: assignmentCfg.Seeder.SignKey})

	if err != nil {
		log.Debug().Err(err).
			Msg("cannot commit changes")
		return fmt.Errorf("cannot commit changes: %w", err)
	}
	return nil
}

func push(assignmentCfg *cfg.AssignmentConfig, repo *git.Repository, wtree *git.Worktree, project *gitlab.Project) error {
	auth, err := g.GetAuth()
	if err != nil {
		log.Debug().Err(err)
		return err
	}
	branch := fmt.Sprintf("refs/heads/%s", assignmentCfg.Seeder.ToBranch)
	b := plumbing.ReferenceName(branch)

	create := false
	_, err = repo.Branch(branch)
	if err != nil {
		create = true
	}

	err = wtree.Checkout(&git.CheckoutOptions{Create: create, Branch: b})
	if err != nil {
		log.Debug().Err(err).
			Str("branch", branch).
			Msg("cannot checkout branch")
		return fmt.Errorf("cannot checkout branch: %w", err)
	}

	conf := &config.RemoteConfig{
		Name: project.Name,
		URLs: []string{project.HTTPURLToRepo},
	}

	remote, err := repo.CreateRemote(conf)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.HTTPURLToRepo).
			Msg("cannot create remote")
		return fmt.Errorf("cannot create remote: %w", err)
	}

	refSpec := config.RefSpec("refs/heads/" + assignmentCfg.Seeder.ToBranch +
		":refs/heads/" + assignmentCfg.Seeder.ToBranch)

	log.Debug().
		Str("refSpec", string(refSpec)).
		Str("name", project.Name).
		Str("toURL", project.HTTPURLToRepo).
		Str("toBranch", assignmentCfg.Seeder.ToBranch).
		Msg("pushing seeded repository")

	pushOpts := &git.PushOptions{
		RemoteName: remote.Config().Name,
		RefSpecs:   []config.RefSpec{refSpec},
		Auth:       auth,
	}
	err = repo.Push(pushOpts)
	if err != nil {
		log.Debug().Err(err).
			Str("name", project.Name).Str("url", project.HTTPURLToRepo).
			Msg("cannot push to remote")
		return fmt.Errorf("cannot push to remote: %w", err)
	}
	return nil
}
