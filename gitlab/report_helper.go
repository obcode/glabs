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

	now := time.Now()

	hasReleaseMergeRequest := false
	hasReleaseDockerImages := false

	if assignmentCfg.Release != nil {
		hasReleaseMergeRequest = assignmentCfg.Release.MergeRequest != nil
		hasReleaseDockerImages = assignmentCfg.Release.DockerImages != nil
	}

	return &report.Reports{
		Course:                 assignmentCfg.Course,
		Assignment:             assignmentCfg.Name,
		URL:                    assignmentCfg.URL,
		Description:            assignmentCfg.Description,
		Projects:               projectReports,
		Generated:              &now,
		HasReleaseMergeRequest: hasReleaseMergeRequest,
		HasReleaseDockerImages: hasReleaseDockerImages,
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

	members, _, err := c.ProjectMembers.ListProjectMembers(project.ID, nil)
	if err != nil {
		log.Error().Err(err).Msg("cannot get members")
	}

	opened := "opened"
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State: &opened,
	}
	mergeRequests, _, err := c.MergeRequests.ListProjectMergeRequests(project.ID, opts)
	if err != nil {
		log.Error().Err(err).Msg("cannot get merge requests")
	}

	var release *report.Release
	if assignmentCfg.Release != nil {
		release = &report.Release{}
		if assignmentCfg.Release.MergeRequest != nil {
			release.MergeRequest = &report.MergeRequest{}
			for _, mr := range mergeRequests {
				if mr.SourceBranch == assignmentCfg.Release.MergeRequest.SourceBranch &&
					mr.TargetBranch == assignmentCfg.Release.MergeRequest.TargetBranch {

					pipelineStatus := "not configured"
					if assignmentCfg.Release.MergeRequest.HasPipeline {
						pipelines, _, err := c.MergeRequests.ListMergeRequestPipelines(project.ID, mr.IID)
						if err != nil {
							log.Error().Err(err).Msg("cannot get pipeline of merge requests")
							pipelineStatus = "no pipeline found"
						} else {
							if len(pipelines) == 0 {
								pipelineStatus = "no pipeline found"
							} else {
								newestPipeline := pipelines[0]
								for i, p := range pipelines {
									if i == 0 {
										continue
									}
									if newestPipeline.CreatedAt.Before(*p.CreatedAt) {
										newestPipeline = p
									}
								}
								pipelineStatus = newestPipeline.Status
							}
						}
					}

					release.MergeRequest = &report.MergeRequest{
						Found:          true,
						WebURL:         mr.WebURL,
						PipelineStatus: pipelineStatus,
					}
					break
				}
			}
		}
		if assignmentCfg.Release.DockerImages != nil {
			t := true
			opts := &gitlab.ListRegistryRepositoriesOptions{
				Tags:      &t,
				TagsCount: &t,
			}
			dockerImages := &report.DockerImages{
				Status: "unknown",
				Images: []*report.DockerImage{},
			}
			registries, _, err := c.ContainerRegistry.ListProjectRegistryRepositories(project.ID, opts)
			if err != nil {
				dockerImages.Status = ""
			} else {
				dockerImageSlice := make([]*report.DockerImage, 0, len(assignmentCfg.Release.DockerImages))
				for _, dockerImageName := range assignmentCfg.Release.DockerImages {
					for _, registryRepo := range registries {
						if dockerImageName == registryRepo.Name {
							dockerImageSlice = append(dockerImageSlice, &report.DockerImage{
								Wanted: dockerImageName,
								Image:  &registryRepo.Location,
							})
						}
					}
				}
				dockerImages.Images = dockerImageSlice
				if len(dockerImageSlice) == len(assignmentCfg.Release.DockerImages) {
					dockerImages.Status = "all available"
				} else {
					dockerImages.Status = fmt.Sprintf("%d of %d available", len(dockerImageSlice), len(assignmentCfg.Release.DockerImages))
				}
			}
			release.DockerImages = dockerImages
		}
	}

	return project.Name, &report.ProjectReport{
		Name:                   project.Name,
		IsActive:               !project.CreatedAt.Equal(*project.LastActivityAt) || len(allCommits) > 0,
		IsEmpty:                project.EmptyRepo,
		Commits:                len(allCommits),
		CreatedAt:              project.CreatedAt,
		LastActivity:           project.LastActivityAt,
		LastCommit:             lastCommit,
		OpenIssuesCount:        project.OpenIssuesCount,
		OpenMergeRequestsCount: len(mergeRequests),
		WebURL:                 project.WebURL,
		Members:                members,
		Release:                release,
	}
}
