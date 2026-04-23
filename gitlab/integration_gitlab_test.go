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

// TestIntegration_GitLab_Operations starts one container and exercises Archive,
// Delete, ProtectToBranch and Setaccess end-to-end in sub-tests so that the
// expensive container start-up happens only once.
func TestIntegration_GitLab_Operations(t *testing.T) {
	requireIntegrationEnabled(t)

	client, baseURL := startGitLabContainer(t)

	// ── Shared parent group ──────────────────────────────────────────────────
	visibility := gitlabapi.PublicVisibility
	parentName := "ops-it-parent"
	parentPath := "ops-it-parent"
	parent, _, err := client.Groups.CreateGroup(&gitlabapi.CreateGroupOptions{
		Name:       &parentName,
		Path:       &parentPath,
		Visibility: &visibility,
	})
	if err != nil {
		t.Fatalf("creating parent group failed: %v", err)
	}

	// ── Shared test user (used by Setaccess sub-test) ────────────────────────
	itUsername := "it-testuser"
	itName := "IT Testuser"
	itEmail := "it-testuser@example.com"
	itPassword := "Pa$$w0rd-test-99"
	skipConfirmation := true
	_, _, err = client.Users.CreateUser(&gitlabapi.CreateUserOptions{
		Username:         &itUsername,
		Name:             &itName,
		Email:            &itEmail,
		Password:         &itPassword,
		SkipConfirmation: &skipConfirmation,
	})
	if err != nil {
		t.Fatalf("creating test user failed: %v", err)
	}

	// ── Helper: build an AssignmentConfig for a sub-group of the parent ──────
	makeAssignmentCfg := func(subPath, studentUsername string) *config.AssignmentConfig {
		path := parent.FullPath + "/" + subPath
		un := studentUsername
		return &config.AssignmentConfig{
			Course:            "it",
			Name:              "a1",
			Path:              path,
			URL:               baseURL + "/" + path,
			Per:               config.PerStudent,
			Description:       "integration test",
			ContainerRegistry: false,
			Students:          []*config.Student{{Username: &un, Raw: un}},
		}
	}

	// ── Helper: create assignment group + student project ────────────────────
	createGroupAndProject := func(t *testing.T, subPath, studentUsername string, withReadme bool) *config.AssignmentConfig {
		t.Helper()
		cfg := makeAssignmentCfg(subPath, studentUsername)
		groupID, err := client.createGroup(cfg)
		if err != nil {
			t.Fatalf("createGroup(%q) failed: %v", subPath, err)
		}
		repoName := cfg.RepoNameWithSuffix(studentUsername)
		initReadme := withReadme
		_, _, err = client.Projects.CreateProject(&gitlabapi.CreateProjectOptions{
			Name:                 &repoName,
			NamespaceID:          &groupID,
			InitializeWithReadme: &initReadme,
		})
		if err != nil {
			t.Fatalf("createProject(%q) failed: %v", repoName, err)
		}
		return cfg
	}

	// ── Sub-test: Archive / Unarchive ─────────────────────────────────────────
	t.Run("Archive", func(t *testing.T) {
		cfg := createGroupAndProject(t, "archive-a1", "student1", false)
		projectPath := cfg.Path + "/" + cfg.RepoNameWithSuffix("student1")

		client.Archive(cfg, false)

		proj, _, err := client.Projects.GetProject(projectPath, &gitlabapi.GetProjectOptions{})
		if err != nil {
			t.Fatalf("GetProject after archive failed: %v", err)
		}
		if !proj.Archived {
			t.Fatal("expected project to be archived")
		}

		client.Archive(cfg, true) // unarchive

		proj, _, err = client.Projects.GetProject(projectPath, &gitlabapi.GetProjectOptions{})
		if err != nil {
			t.Fatalf("GetProject after unarchive failed: %v", err)
		}
		if proj.Archived {
			t.Fatal("expected project to be unarchived after Archive(unarchive=true)")
		}
	})

	// ── Sub-test: Delete ──────────────────────────────────────────────────────
	t.Run("Delete", func(t *testing.T) {
		cfg := createGroupAndProject(t, "delete-a1", "student1", false)
		repoName := cfg.RepoNameWithSuffix("student1")

		groupID, err := client.getGroupIDByFullPath(cfg.Path)
		if err != nil {
			t.Fatalf("getGroupIDByFullPath before delete failed: %v", err)
		}

		client.Delete(cfg)

		projects, _, err := client.Search.ProjectsByGroup(groupID, repoName, &gitlabapi.SearchOptions{})
		if err != nil {
			t.Fatalf("search after Delete failed: %v", err)
		}
		for _, p := range projects {
			if p.Name == repoName {
				t.Fatalf("project %q still exists after Delete()", repoName)
			}
		}
	})

	// ── Sub-test: ProtectToBranch ─────────────────────────────────────────────
	t.Run("ProtectToBranch", func(t *testing.T) {
		// withReadme=true so the project has a 'main' branch immediately
		cfg := createGroupAndProject(t, "protect-a1", "student1", true)
		cfg.Startercode = &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		}

		client.ProtectToBranch(cfg)

		projectPath := cfg.Path + "/" + cfg.RepoNameWithSuffix("student1")
		proj, _, err := client.Projects.GetProject(projectPath, &gitlabapi.GetProjectOptions{})
		if err != nil {
			t.Fatalf("GetProject after ProtectToBranch failed: %v", err)
		}
		branches, _, err := client.ProtectedBranches.ListProtectedBranches(
			proj.ID, &gitlabapi.ListProtectedBranchesOptions{})
		if err != nil {
			t.Fatalf("ListProtectedBranches failed: %v", err)
		}
		found := false
		for _, b := range branches {
			if b.Name == "main" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected branch 'main' to be listed as protected")
		}
	})

	// ── Sub-test: Setaccess ───────────────────────────────────────────────────
	t.Run("Setaccess", func(t *testing.T) {
		cfg := createGroupAndProject(t, "setaccess-a1", itUsername, false)
		cfg.AccessLevel = config.AccessLevel(gitlabapi.DeveloperPermissions) // 30

		client.Setaccess(cfg)

		projectPath := cfg.Path + "/" + cfg.RepoNameWithSuffix(itUsername)
		proj, _, err := client.Projects.GetProject(projectPath, &gitlabapi.GetProjectOptions{})
		if err != nil {
			t.Fatalf("GetProject after Setaccess failed: %v", err)
		}
		members, _, err := client.ProjectMembers.ListProjectMembers(
			proj.ID, &gitlabapi.ListProjectMembersOptions{})
		if err != nil {
			t.Fatalf("ListProjectMembers failed: %v", err)
		}
		found := false
		for _, m := range members {
			if m.Username == itUsername {
				found = true
				if gitlabapi.AccessLevelValue(m.AccessLevel) != gitlabapi.DeveloperPermissions {
					t.Fatalf("member access level = %v, want DeveloperPermissions (30)", m.AccessLevel)
				}
				break
			}
		}
		if !found {
			t.Fatalf("expected user %q to be a project member after Setaccess()", itUsername)
		}
	})
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
