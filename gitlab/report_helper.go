package gitlab

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

type Reports struct {
	Projects []*ProjectReport
}

type ProjectReport struct {
	Name               string
	IsActive           bool
	IsEmpty            bool
	Commits            int
	LastActivity       *time.Time
	OpenIssuesCount    int
	MergeRequestsCount int
	WebURL             string
}

func (c *Client) report(assignmentCfg *config.AssignmentConfig) *Reports {
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

	projectReportsMap := make(map[string]*ProjectReport)
	for _, project := range projects {
		pojectName, projectReport := c.projectReport(assignmentCfg, project)
		projectReportsMap[pojectName] = projectReport
	}

	keys := make([]string, 0, len(projects))
	for k := range projectReportsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	projectReports := make([]*ProjectReport, 0, len(projects))
	for _, projectName := range keys {
		projectReports = append(projectReports, projectReportsMap[projectName])
	}

	return &Reports{
		Projects: projectReports,
	}
}

func (c *Client) projectReport(assignmentCfg *config.AssignmentConfig, project *gitlab.Project) (string, *ProjectReport) {
	return project.Name, &ProjectReport{
		Name:               project.Name,
		IsActive:           true,
		IsEmpty:            project.EmptyRepo,
		Commits:            0,
		LastActivity:       project.LastActivityAt,
		OpenIssuesCount:    project.OpenIssuesCount,
		MergeRequestsCount: 0,
		WebURL:             project.WebURL,
	}
}
