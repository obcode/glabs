package gitlab

import (
	"fmt"

	"github.com/obcode/glabs/v2/config"
	"github.com/obcode/glabs/v2/git"
)

func (c *Client) Push(assignmentCfg *config.AssignmentConfig, branchname string) error {
	branch, ok := assignmentCfg.DeferredBranches[branchname]
	if !ok {
		return fmt.Errorf("error: no config for deferred branch \"%s\" found\n", branchname)
	}

	sourceRepo, err := git.CloneBranch(branch.URL, branch.FromBranch, branch.Orphan, branch.OrphanMessage)
	if err != nil {
		return err
	}

	names := make([]string, 0)

	switch assignmentCfg.Per {
	case config.PerStudent:
		for _, student := range assignmentCfg.Students {
			names = append(names, assignmentCfg.RepoNameForStudent(student))
		}
	case config.PerGroup:
		for _, grp := range assignmentCfg.Groups {
			names = append(names, assignmentCfg.RepoNameForGroup(grp))
		}
	}

	for _, name := range names {
		projectname := assignmentCfg.Path + "/" + name

		project, err := c.getProjectByName(projectname)
		if err != nil {
			return err
		}

		err = git.PushBranch(assignmentCfg, projectname, sourceRepo, branch.ToBranch, true, project)
		if err != nil {
			return err
		}
	}

	return nil
}
