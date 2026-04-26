package gitlab

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/obcode/glabs/v2/config"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// getStartercodeProject extracts the project path from the startercode URL and returns the GitLab project
func (c *Client) getStartercodeProject(assignmentCfg *config.AssignmentConfig) (*gitlab.Project, error) {
	// Parse project path from URL
	// Expected formats:
	// git@gitlab.lrz.de:mpd/startercode/blatt-01.git
	// https://gitlab.lrz.de/mpd/startercode/blatt-01.git
	// https://gitlab.lrz.de/mpd/startercode/blatt-01

	url := assignmentCfg.Startercode.URL

	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	var projectPath string

	// Handle SSH URLs (git@host:path/to/project)
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			projectPath = parts[1]
		}
	} else {
		// Handle HTTPS URLs (https://host/path/to/project)
		re := regexp.MustCompile(`https?://[^/]+/(.+)`)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			projectPath = matches[1]
		}
	}

	if projectPath == "" {
		return nil, fmt.Errorf("could not parse project path from URL: %s", assignmentCfg.Startercode.URL)
	}

	log.Debug().Str("projectPath", projectPath).Msg("loading startercode project for issue replication")

	project, _, err := c.Projects.GetProject(projectPath, nil)
	if err != nil {
		return nil, fmt.Errorf("could not get startercode project: %w", err)
	}

	return project, nil
}

// replicateIssue loads a single issue from source project and creates it in target project
func (c *Client) replicateIssue(sourceProject *gitlab.Project, targetProject *gitlab.Project, issueNumber int) error {
	// Load issue from startercode project
	issue, _, err := c.Issues.GetIssue(sourceProject.ID, int64(issueNumber), nil)
	if err != nil {
		return fmt.Errorf("could not get issue from startercode project %d with number %d: %w", sourceProject.ID, issueNumber, err)
	}

	// Create issue in target project
	createIssueOpts := &gitlab.CreateIssueOptions{
		Title:       gitlab.Ptr(issue.Title),
		Description: gitlab.Ptr(issue.Description),
	}

	_, _, err = c.Issues.CreateIssue(targetProject.ID, createIssueOpts)
	if err != nil {
		return fmt.Errorf("could not create issue %q in target project %d: %w", issue.Title, targetProject.ID, err)
	}

	log.Debug().
		Str("issueTitle", issue.Title).
		Str("targetProject", targetProject.PathWithNamespace).
		Msg("successfully replicated issue")

	return nil
}
