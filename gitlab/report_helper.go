package gitlab

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab/report"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) report(assignmentCfg *config.AssignmentConfig) *report.Reports {
	groupID, err := c.getGroupID(assignmentCfg)
	if err != nil {
		fmt.Printf("error: GitLab group for assignment does not exist, please create the group %s\n", assignmentCfg.URL)
		os.Exit(1)
	}
	log.Debug().Int("groupID", groupID).Msg("found group id")

	projects := make([]*gitlab.Project, 0)
	var opts *gitlab.ListGroupProjectsOptions
	for {
		someProjects, response, err := c.Groups.ListGroupProjects(groupID, opts)
		if err != nil {
			log.Error().Err(err).Msg("error while trying to get all projects in subgroup")
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
		pojectName, projectReport := c.projectReport(assignmentCfg, project)
		projectReportsMap[pojectName] = projectReport
	}

	keys := make([]string, 0, len(projects))
	for k := range projectReportsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	projectReports := make([]*report.ProjectReport, 0, len(projects))
	for _, projectName := range keys {
		projectReports = append(projectReports, projectReportsMap[projectName])
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
	return project.Name, &report.ProjectReport{
		Name:            project.Name,
		IsActive:        !project.CreatedAt.Equal(*project.LastActivityAt),
		IsEmpty:         project.EmptyRepo,
		Commits:         0,
		CreatedAt:       project.CreatedAt,
		LastActivity:    project.LastActivityAt,
		OpenIssuesCount: project.OpenIssuesCount,
		WebURL:          project.WebURL,
	}
}
