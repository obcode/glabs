//go:build integration

package gitlab

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/obcode/glabs/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

const (
	gitLabImage       = "gitlab/gitlab-ce:17.6.1-ce.0"
	gitLabRootToken   = "glabs-integration-root-token"
	runIntegrationEnv = "GLABS_RUN_GITLAB_TC"
)

func requireIntegrationEnabled(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv(runIntegrationEnv) != "1" {
		t.Skipf("set %s=1 to run GitLab testcontainer integration tests", runIntegrationEnv)
	}
}

func createRootToken(ctx context.Context, t *testing.T, c testcontainers.Container) string {
	t.Helper()

	script := strings.Join([]string{
		"user = User.find_by_username('root')",
		"token = user.personal_access_tokens.find_by(name: 'glabs-integration-token')",
		"token&.revoke!",
		"token = user.personal_access_tokens.create!(name: 'glabs-integration-token', scopes: [:api], expires_at: 365.days.from_now)",
		fmt.Sprintf("token.set_token('%s')", gitLabRootToken),
		"token.save!",
		"puts token.token",
	}, "; ")

	cmd := []string{"gitlab-rails", "runner", script}
	exitCode, reader, err := c.Exec(ctx, cmd)
	if err != nil {
		t.Fatalf("creating root token via gitlab-rails failed: %v", err)
	}
	outputBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading gitlab-rails output failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("gitlab-rails runner exit code %d, output:\n%s", exitCode, string(outputBytes))
	}

	scanner := bufio.NewScanner(strings.NewReader(string(outputBytes)))
	lastLine := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lastLine = line
		}
	}
	if lastLine == "" {
		t.Fatalf("could not parse token from gitlab-rails output: %q", string(outputBytes))
	}

	return lastLine
}

func startGitLabContainer(t *testing.T) (*Client, string) {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        gitLabImage,
		ExposedPorts: []string{"80/tcp"},
		Env: map[string]string{
			"GITLAB_ROOT_PASSWORD": "glabs-root-password",
			"GITLAB_OMNIBUS_CONFIG": strings.Join([]string{
				"external_url 'http://localhost'",
				"nginx['listen_port'] = 80",
				"prometheus_monitoring['enable'] = false",
				"puma['worker_processes'] = 0",
				"sidekiq['max_concurrency'] = 5",
			}, "; "),
		},
		WaitingFor: wait.ForHTTP("/users/sign_in").WithPort("80/tcp").WithStartupTimeout(25 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("starting gitlab testcontainer failed: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("getting container host failed: %v", err)
	}
	port, err := container.MappedPort(ctx, "80/tcp")
	if err != nil {
		t.Fatalf("getting mapped port failed: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())
	rootToken := createRootToken(ctx, t, container)

	apiClient, err := gitlabapi.NewClient(rootToken, gitlabapi.WithBaseURL(baseURL+"/api/v4"))
	if err != nil {
		t.Fatalf("creating gitlab api client failed: %v", err)
	}

	return &Client{apiClient}, baseURL
}

func TestIntegration_GitLab_GroupAndProjectLifecycle(t *testing.T) {
	requireIntegrationEnabled(t)

	client, baseURL := startGitLabContainer(t)

	parentName := "mpd-it-parent"
	parentPath := "mpd-it-parent"
	visibility := gitlabapi.PublicVisibility
	parent, _, err := client.Groups.CreateGroup(&gitlabapi.CreateGroupOptions{
		Name:       &parentName,
		Path:       &parentPath,
		Visibility: &visibility,
	})
	if err != nil {
		t.Fatalf("creating parent group failed: %v", err)
	}

	assignmentCfg := &config.AssignmentConfig{
		Course:            "mpd",
		Name:              "a1",
		Path:              parent.FullPath + "/blatt-01",
		URL:               baseURL + "/" + parent.FullPath + "/blatt-01",
		Per:               config.PerStudent,
		Description:       "integration test assignment",
		ContainerRegistry: false,
	}

	groupID, err := client.createGroup(assignmentCfg)
	if err != nil {
		t.Fatalf("createGroup failed: %v", err)
	}
	if groupID == 0 {
		t.Fatal("createGroup returned zero group id")
	}

	resolvedGroupID, err := client.getGroupIDByFullPath(assignmentCfg.Path)
	if err != nil {
		t.Fatalf("getGroupIDByFullPath failed: %v", err)
	}
	if resolvedGroupID != groupID {
		t.Fatalf("resolved group id = %d, want %d", resolvedGroupID, groupID)
	}

	project, generated, err := client.generateProject(assignmentCfg, "a1-team1", groupID)
	if err != nil {
		t.Fatalf("generateProject failed: %v", err)
	}
	if !generated {
		t.Fatal("expected generateProject to create a new project")
	}
	if project == nil || project.PathWithNamespace == "" {
		t.Fatalf("invalid project response: %#v", project)
	}

	foundProject, err := client.getProjectByName(project.PathWithNamespace)
	if err != nil {
		t.Fatalf("getProjectByName failed: %v", err)
	}
	if foundProject.ID != project.ID {
		t.Fatalf("found project id = %d, want %d", foundProject.ID, project.ID)
	}
}
