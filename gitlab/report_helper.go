package gitlab

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab/report"
	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) report(assignmentCfg *config.AssignmentConfig) *report.Reports {
	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[69],
		Suffix: aurora.Sprintf(aurora.Cyan(" fetching info for  %s / %s"),
			aurora.Yellow(assignmentCfg.Course),
			aurora.Magenta(assignmentCfg.Name),
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

	spinner.Message(aurora.Sprintf(aurora.Green("get group info")))
	groupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

		err := spinner.StopFail()
		if err != nil {
			log.Debug().Err(err).Msg("cannot stop spinner")
		}
		return nil
	}
	log.Debug().Int("groupID", groupID).Msg("found group id")

	projects := make([]*gitlab.Project, 0)
	var opts *gitlab.ListGroupProjectsOptions
	for {
		someProjects, response, err := c.Groups.ListGroupProjects(groupID, opts)
		if err != nil {
			spinner.StopFailMessage(fmt.Sprintf("problem: %v", err))

			err := spinner.StopFail()
			if err != nil {
				log.Debug().Err(err).Msg("cannot stop spinner")
			}
			return nil
		}
		projects = append(projects, someProjects...)

		if len(response.Header["X-Next-Page"]) > 0 {
			nextPage := response.Header["X-Next-Page"][0]
			page, err := strconv.Atoi(nextPage)
			if err != nil {
				break
			}

			opts = &gitlab.ListGroupProjectsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    page,
					PerPage: 0,
				},
			}
		}

	}

	projectReportsMap := make(map[string]*report.ProjectReport)
	for _, project := range projects {
		spinner.Message(aurora.Sprintf(aurora.Green(fmt.Sprintf("get info for %s", project.Name))))

		pojectName, projectReport := c.projectReport(assignmentCfg, project)
		projectReportsMap[pojectName] = projectReport
	}

	spinner.Message(aurora.Sprintf(aurora.Green("sorting projects")))
	keys := make([]string, 0, len(projects))
	for k := range projectReportsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	projectReports := make([]*report.ProjectReport, 0, len(projects))
	for _, projectName := range keys {
		projectReports = append(projectReports, projectReportsMap[projectName])
	}

	err = spinner.Stop()
	if err != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}

	return &report.Reports{
		Course:      assignmentCfg.Course,
		Assignment:  assignmentCfg.Name,
		URL:         assignmentCfg.URL,
		Description: assignmentCfg.Description,
		Projects:    projectReports,
	}
}

func (c *Client) projectReport(assignmentCfg *config.AssignmentConfig, project *gitlab.Project) (string, *report.ProjectReport) {
	branches, _, err := c.Branches.ListBranches(project.ID, nil)
	if err != nil {
		log.Error().Err(err).Msg("cannot get commits")
	}

	allCommits := make([]*gitlab.Commit, 0)

	for _, branch := range branches {
		opts := &gitlab.ListCommitsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    0,
				PerPage: 1000,
			},
			RefName: &branch.Name,
			Since:   project.CreatedAt,
		}
		commits, _, err := c.Commits.ListCommits(project.ID, opts)
		if err != nil {
			log.Error().Err(err).Msg("cannot get commits")
		}
		allCommits = append(allCommits, commits...)
	}

	var lastCommit *report.Commit

	for _, commit := range allCommits {
		if lastCommit == nil || lastCommit.CommittedDate.Before(*commit.CommittedDate) {
			lastCommit = &report.Commit{
				Title:         commit.Title,
				CommitterName: commit.CommitterName,
				CommittedDate: commit.CommittedDate,
				WebURL:        commit.WebURL,
			}
		}
	}

	return project.Name, &report.ProjectReport{
		Name:            project.Name,
		IsActive:        !project.CreatedAt.Equal(*project.LastActivityAt) || len(allCommits) > 0,
		IsEmpty:         project.EmptyRepo,
		Commits:         len(allCommits),
		CreatedAt:       project.CreatedAt,
		LastActivity:    project.LastActivityAt,
		LastCommit:      lastCommit,
		OpenIssuesCount: project.OpenIssuesCount,
		WebURL:          project.WebURL,
	}
}
