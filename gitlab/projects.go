package gitlab

import (
	"errors"
	"fmt"

	"github.com/obcode/glabs/config"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) generateProject(assignmentCfg *config.AssignmentConfig, suffix string, inID int) (*gitlab.Project, bool, error) {
	generated := false
	name := assignmentCfg.Name + "-" + suffix

	p := &gitlab.CreateProjectOptions{
		Name:                     gitlab.String(name),
		Description:              gitlab.String(assignmentCfg.Description),
		NamespaceID:              gitlab.Int(inID),
		MergeRequestsAccessLevel: gitlab.AccessControl("enabled"),
		IssuesAccessLevel:        gitlab.AccessControl("enabled"),
		BuildsAccessLevel:        gitlab.AccessControl("enabled"),
		JobsEnabled:              gitlab.Bool(true),
		Visibility:               gitlab.Visibility(gitlab.PrivateVisibility),
		ContainerRegistryEnabled: gitlab.Bool(assignmentCfg.ContainerRegistry),
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
				log.Fatal().Err(err)
				return nil, false, fmt.Errorf("%w", err)
			}
		} else {
			log.Fatal().Err(err)
		}
	}

	return project, generated, nil
}

func (c *Client) getProjectByName(fullpathprojectname string) (*gitlab.Project, error) {
	opt := &gitlab.ListProjectsOptions{
		Search:           gitlab.String(fullpathprojectname),
		SearchNamespaces: gitlab.Bool(true),
	}
	projects, _, err := c.Projects.ListProjects(opt)
	if err != nil {
		log.Error().Err(err).
			Str("projectname", fullpathprojectname).
			Msg("no project found")
	} else {
		switch len(projects) {
		case 1:
			return projects[0], nil
		case 0:
			log.Debug().Interface("projects", projects).Msg("more than one project found")
			return nil, errors.New("more than one project found")
		default:
			log.Debug().Msg("more than one project matching the search string found")
			for _, project := range projects {
				if project.PathWithNamespace == fullpathprojectname {
					log.Debug().Str("name", fullpathprojectname).Msg("found project")
					return project, nil
				}
			}
			log.Debug().Str("name", fullpathprojectname).Msg("project not found")
			return nil, errors.New("project not found")
		}
	}
	return nil, nil // could not happen
}
