package gitlab

import (
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) generateProject(prefix, group, assignment, assignmentPath string,
	inID int) (*gitlab.Project, error) {
	name := assignment + "-" + prefix
	description := "generated by glabs"

	if desc := viper.GetString(group + "." + assignment + ".description"); desc != "" {
		description = desc
	}

	log.Debug().Str("desciption", description).Msg("generating with description")

	p := &gitlab.CreateProjectOptions{
		Name:                     gitlab.String(name),
		Description:              gitlab.String(description),
		NamespaceID:              gitlab.Int(inID),
		MergeRequestsAccessLevel: gitlab.AccessControl("enabled"),
		IssuesAccessLevel:        gitlab.AccessControl("enabled"),
		BuildsAccessLevel:        gitlab.AccessControl("enabled"),
		JobsEnabled:              gitlab.Bool(true),
		Visibility:               gitlab.Visibility(gitlab.PrivateVisibility),
	}

	project, _, err := c.Projects.CreateProject(p)

	if err == nil {
		log.Debug().Str("name", name).Msg("generated repo")
	} else { // err != nil
		if project == nil {
			projectname := assignmentPath + "/" + name
			log.Debug().Str("name", projectname).Msg("searching for project")
			opt := &gitlab.ListProjectsOptions{
				Search:           gitlab.String(projectname),
				SearchNamespaces: gitlab.Bool(true),
			}
			projects, _, err := c.Projects.ListProjects(opt)
			if err != nil {
				log.Fatal().Err(err)
			} else {
				if len(projects) == 1 {
					project = projects[0]
				} else {
					log.Debug().Interface("projects", projects).Msg("more than one project found")
					return nil, errors.New("more than one project found")
				}
			}
		} else {
			log.Fatal().Err(err)
		}
	}

	return project, nil
}