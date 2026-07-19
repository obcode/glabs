package gitlab

import (
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/git"
	"github.com/rs/zerolog/log"
)

func (c *Client) Generate(assignmentCfg *config.AssignmentConfig) error {
	assignmentGitLabGroupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		// try to create group if it does not exist, otherwise return an error
		assignmentGitLabGroupID, err = c.createGroup(assignmentCfg)
		if err != nil {
			log.Error().Err(err).
				Str("course", assignmentCfg.Course).
				Str("assignmentpath", assignmentCfg.Path).
				Msg("error while creating group for assignment")
			return fmt.Errorf("cannot create GitLab group for assignment, please create the group %s", assignmentCfg.URL)
		}
	}

	var starterrepo *git.SourceRepo

	if assignmentCfg.Startercode != nil {
		starterrepo, err = git.PrepareSourceRepo(c.rep, c.gitAuth(), c.committer,
			assignmentCfg.Startercode.URL,
			assignmentCfg.Startercode.FromBranch,
			assignmentCfg.Startercode.Template,
			assignmentCfg.Startercode.TemplateMessage,
		)
		if err != nil {
			return err
		}
	}

	switch per := assignmentCfg.Per; per {
	case config.PerGroup:
		c.generatePerGroup(assignmentCfg, assignmentGitLabGroupID, starterrepo)
	case config.PerStudent:
		c.generatePerStudent(assignmentCfg, assignmentGitLabGroupID, starterrepo)
	default:
		return fmt.Errorf("it is only possible to generate for students or groups, not for %v", per)
	}
	return nil
}

func (c *Client) generate(assignmentCfg *config.AssignmentConfig, assignmentGroupID int64,
	projectname string, members []*config.Student, starterrepo *git.SourceRepo) {

	task := c.rep.Task(aurora.Sprintf(aurora.Cyan(" generating project %s at %s"),
		aurora.Yellow(projectname),
		aurora.Magenta(assignmentCfg.URL+"/"+projectname),
	))
	task.Update("generating project on host")

	project, generated, err := c.generateProject(assignmentCfg, projectname, assignmentGroupID)
	if err != nil {
		task.Fail(fmt.Sprintf("problem: %v", err))
		return
	}
	if !generated {
		task.Done(aurora.Sprintf(aurora.Red("project already exists")))
	} else {
		task.Done("")
	}

	if starterrepo != nil {
		if !generated {
			c.rep.Println(aurora.Red("    ↪ not trying to push startercode to existing project"))
		} else {
			task := c.rep.Task(aurora.Sprintf(aurora.Cyan(" ↪ pushing startercode")))
			if err := c.pushStartercode(assignmentCfg, starterrepo, project); err != nil {
				task.Fail(fmt.Sprintf("problem: %v", err))
				return
			}
			task.Done("")
		}
	} else if assignmentCfg.Seeder != nil {
		if !generated {
			c.rep.Println(aurora.Red("    ↪ not running seeder for existing project"))
		} else {
			task := c.rep.Task(aurora.Sprintf(aurora.Cyan(" ↪ seeding project %s using %s"),
				aurora.Magenta(projectname),
				aurora.Magenta(assignmentCfg.Seeder.Command),
			))
			if err := c.runSeeder(assignmentCfg, project); err != nil {
				task.Fail(fmt.Sprintf("problem: %v", err))
				return
			}
			task.Done("")
		}
	}

	if generated && len(assignmentCfg.Branches) > 0 {
		baseBranch := defaultBranchName(assignmentCfg.Branches, "main")
		if assignmentCfg.Startercode != nil {
			baseBranch = assignmentCfg.Startercode.ToBranch
		} else if assignmentCfg.Seeder != nil {
			baseBranch = assignmentCfg.Seeder.ToBranch
		}

		if err := c.syncConfiguredBranches(assignmentCfg, project, baseBranch, len(members)); err != nil {
			log.Error().Err(err).Str("project", project.Name).Msg("cannot apply configured branch rules")
			c.rep.Printf("error: cannot apply branch/approval rules for project %s: %v\n", project.Name, err)
		}
	}

	// Replicate issues from startercode repo if configured
	if generated && assignmentCfg.Startercode != nil && assignmentCfg.Issues != nil && assignmentCfg.Issues.ReplicateFromStartercode {
		starterProject, starterProjectErr := c.getStartercodeProject(assignmentCfg)
		issueNumbers := assignmentCfg.Issues.IssueNumbers
		parentByChild := make(map[int]int)

		if starterProjectErr == nil {
			plan, planErr := c.resolveIssuePlanForReplication(
				starterProject,
				assignmentCfg.Issues.IssueNumbers,
				assignmentCfg.Issues.IncludeChildTasks,
			)
			if planErr != nil {
				starterProjectErr = planErr
			} else {
				issueNumbers = plan.OrderedIssues
				parentByChild = plan.ParentByChild
			}
		}

		createdIssueMap := make(map[int]int, len(issueNumbers))

		for _, issueNumber := range issueNumbers {
			task := c.rep.Task(aurora.Sprintf(
				aurora.Cyan(" ↪ replicating issue #%d from startercode"),
				aurora.Yellow(issueNumber),
			))

			if starterProjectErr != nil {
				task.Fail(fmt.Sprintf("problem: %v", starterProjectErr))
				continue
			}

			_, isChildTask := parentByChild[issueNumber]
			createdIssueIID, replicateErr := c.replicateIssue(starterProject, project, issueNumber, isChildTask)
			if replicateErr != nil {
				task.Fail(fmt.Sprintf("problem: %v", replicateErr))
				continue
			}

			createdIssueMap[issueNumber] = createdIssueIID
			task.Done("")
		}

		if starterProjectErr == nil && assignmentCfg.Issues.IncludeChildTasks {
			for childSource, parentSource := range parentByChild {
				parentTarget, hasParent := createdIssueMap[parentSource]
				childTarget, hasChild := createdIssueMap[childSource]
				if !hasParent || !hasChild {
					continue
				}

				if err = c.attachChildTaskToParent(project, parentTarget, childTarget); err != nil {
					log.Error().Err(err).
						Int("sourceParentIssue", parentSource).
						Int("sourceChildIssue", childSource).
						Int("targetParentIssue", parentTarget).
						Int("targetChildIssue", childTarget).
						Msg("cannot attach replicated child task to parent")
				}
			}
		}
	}

	c.setaccess(assignmentCfg, project, members)
}

func (c *Client) generatePerStudent(assignmentCfg *config.AssignmentConfig, assignmentGroupID int64,
	starterrepo *git.SourceRepo) {
	if len(assignmentCfg.Students) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no students found")
		return
	}

	for _, student := range assignmentCfg.Students {
		name := assignmentCfg.RepoNameForStudent(student)
		c.generate(assignmentCfg, assignmentGroupID, name, []*config.Student{student}, starterrepo)
	}
}

func (c *Client) generatePerGroup(assignmentCfg *config.AssignmentConfig, assignmentGroupID int64,
	starterrepo *git.SourceRepo) {
	if len(assignmentCfg.Groups) == 0 {
		log.Info().Str("group", assignmentCfg.Course).Msg("no groups found")
		return
	}

	for _, grp := range assignmentCfg.Groups {
		c.generate(assignmentCfg, assignmentGroupID, assignmentCfg.RepoNameForGroup(grp), grp.Members, starterrepo)
	}
}
