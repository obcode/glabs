package gitlab

import (
	"errors"
	"fmt"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) generateProject(assignmentCfg *config.AssignmentConfig, name string, inID int) (*gitlab.Project, bool, error) {
	generated := false
	p := &gitlab.CreateProjectOptions{
		Name:                                  gitlab.Ptr(name),
		Description:                           gitlab.Ptr(assignmentCfg.Description),
		NamespaceID:                           gitlab.Ptr(inID),
		MergeRequestsAccessLevel:              gitlab.Ptr(gitlab.EnabledAccessControl),
		IssuesAccessLevel:                     gitlab.Ptr(gitlab.EnabledAccessControl),
		BuildsAccessLevel:                     gitlab.Ptr(gitlab.EnabledAccessControl),
		JobsEnabled:                           gitlab.Ptr(true),
		Visibility:                            gitlab.Ptr(gitlab.PrivateVisibility),
		ContainerRegistryEnabled:              gitlab.Ptr(assignmentCfg.ContainerRegistry),
		OnlyAllowMergeIfAllStatusChecksPassed: gitlab.Ptr(false),
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
