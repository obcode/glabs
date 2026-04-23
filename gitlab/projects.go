package gitlab

import (
	"errors"
	"fmt"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func (c *Client) generateProject(assignmentCfg *config.AssignmentConfig, name string, inID int64) (*gitlab.Project, bool, error) {
	generated := false

	// Merge method should already be defaulted by config parsing.
	// Keep this fallback for safety when AssignmentConfig is constructed manually.
	mergeMethod := config.MergeCommit
	squashOption := config.SquashDefaultOff
	pipelineMustSucceed := false
	skippedPipelinesAreSuccessful := false
	allThreadsMustBeResolved := false
	statusChecksMustSucceed := false
	if assignmentCfg.MergeRequest != nil {
		mergeMethod = assignmentCfg.MergeRequest.MergeMethod
		squashOption = assignmentCfg.MergeRequest.SquashOption
		pipelineMustSucceed = assignmentCfg.MergeRequest.PipelineMustSucceed
		skippedPipelinesAreSuccessful = assignmentCfg.MergeRequest.SkippedPipelinesAreSuccessful
		allThreadsMustBeResolved = assignmentCfg.MergeRequest.AllThreadsMustBeResolved
		statusChecksMustSucceed = assignmentCfg.MergeRequest.StatusChecksMustSucceed
	}

	// Convert glabs MergeMethod to GitLab API MergeMethodValue
	var gitlabMergeMethod gitlab.MergeMethodValue
	switch mergeMethod {
	case config.SemiLinearHistory:
		gitlabMergeMethod = gitlab.RebaseMerge
	case config.FastForward:
		gitlabMergeMethod = gitlab.FastForwardMerge
	default:
		gitlabMergeMethod = gitlab.NoFastForwardMerge
	}

	// Convert glabs SquashOption to GitLab API SquashOptionValue
	var gitlabSquashOption gitlab.SquashOptionValue
	switch squashOption {
	case config.SquashNever:
		gitlabSquashOption = gitlab.SquashOptionNever
	case config.SquashAlways:
		gitlabSquashOption = gitlab.SquashOptionAlways
	case config.SquashDefaultOn:
		gitlabSquashOption = gitlab.SquashOptionDefaultOn
	default:
		gitlabSquashOption = gitlab.SquashOptionDefaultOff
	}

	p := &gitlab.CreateProjectOptions{
		Name:                             gitlab.Ptr(name),
		Description:                      gitlab.Ptr(assignmentCfg.Description),
		NamespaceID:                      gitlab.Ptr(inID),
		MergeRequestsAccessLevel:         gitlab.Ptr(gitlab.EnabledAccessControl),
		IssuesAccessLevel:                gitlab.Ptr(gitlab.EnabledAccessControl),
		BuildsAccessLevel:                gitlab.Ptr(gitlab.EnabledAccessControl),
		JobsEnabled:                      gitlab.Ptr(true),
		Visibility:                       gitlab.Ptr(gitlab.PrivateVisibility),
		ContainerRegistryEnabled:         gitlab.Ptr(assignmentCfg.ContainerRegistry),
		OnlyAllowMergeIfPipelineSucceeds: gitlab.Ptr(pipelineMustSucceed),
		AllowMergeOnSkippedPipeline:      gitlab.Ptr(skippedPipelinesAreSuccessful),
		OnlyAllowMergeIfAllDiscussionsAreResolved: gitlab.Ptr(allThreadsMustBeResolved),
		OnlyAllowMergeIfAllStatusChecksPassed:     gitlab.Ptr(statusChecksMustSucceed),
		MergeMethod:                               gitlab.Ptr(gitlabMergeMethod),
		SquashOption:                              gitlab.Ptr(gitlabSquashOption),
	}

	project, _, err := c.Projects.CreateProject(p)

	if err == nil {
		log.Debug().Str("name", name).Msg("generated repo")
		generated = true
	} else {
		if project == nil {
			projectname := assignmentCfg.Path + "/" + name
			log.Debug().Err(err).Str("name", projectname).Msg("searching for project")
			project, err = c.getProjectByName(projectname)
			if err != nil {
				log.Debug().Err(err).Msg("project not found")
				return nil, false, fmt.Errorf("problem while creating project %w", err)
			}
		} else {
			log.Debug().Err(err).Msg("got project, but error")
			return nil, false, err
		}
	}

	return project, generated, nil
}

func (c *Client) getProjectByName(fullpathprojectname string) (*gitlab.Project, error) {
	opt := &gitlab.ListProjectsOptions{
		Search:           gitlab.Ptr(fullpathprojectname),
		SearchNamespaces: gitlab.Ptr(true),
	}
	projects, _, err := c.Projects.ListProjects(opt)
	if err != nil {
		log.Debug().Err(err).
			Str("projectname", fullpathprojectname).
			Msg("no project found")
		return nil, fmt.Errorf("error while trying to find project: %w", err)
	} else {
		switch len(projects) {
		case 1:
			return projects[0], nil
		case 0:
			log.Debug().Msg("no project found")
			return nil, errors.New("project not found")
		default:
			log.Debug().Msg("more than one project matching the search string found")
			for _, project := range projects {
				if project.PathWithNamespace == fullpathprojectname {
					log.Debug().Str("name", fullpathprojectname).Msg("found project")
					return project, nil
				}
			}
			return nil, errors.New("more than one project matching the search string found")
		}
	}
}
