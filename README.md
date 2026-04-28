# glabs

Command line tool to manage GitLab repositories for student assignments.

This README is the quick entry point. The full user handbook lives in the docs folder.

## Why glabs

- Create assignment repositories for students or groups
- Seed repositories from starter code or custom seeding tools
- Protect branches and set access rules at scale
- Generate URLs, clone repos, and build reports

## Installation

### Build from source

Prerequisite: Go 1.24+

```sh
go install github.com/obcode/glabs/v2@latest
```

### Add glabs to your PATH

After installation, make sure that your Go bin directory is in your `PATH` so you can use `glabs` from anywhere:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

You can add this line to your `~/.bashrc`, `~/.zshrc`, or `~/.profile` to make it permanent.

### Build from local checkout

```sh
go install .
```

### or just unpack the prebuilt binaries

from <https://github.com/obcode/glabs/releases>

## Quickstart

### 1) Create main config in your home directory

File: ~/.glabs.yaml

```yaml
gitlab:
  host: https://gitlab.example.org
  token: <personal-access-token>

coursesfilepath: /absolute/path/to/course-configs
courses:
  - mpd
  - vss
```

### 2) Create one course file

Example: /absolute/path/to/course-configs/mpd.yaml

```yaml
mpd:
  coursepath: mpd/semester
  semesterpath: ob-26ss

  blatt01:
    assignmentpath: blatt-01
    per: student
    startercode:
      url: git@gitlab.example.org:mpd/startercode/blatt-01.git
      fromBranch: template
```

### 3) Validate config and generate repos

```sh
glabs check mpd
glabs generate mpd blatt01
```

## Common commands

```sh
glabs check <course>
glabs generate <course> <assignment> [groups...|students...]
glabs protect <course> <assignment> [groups...|students...]
glabs clone <course> <assignment> [groups...|students...]
glabs urls <course> <assignment> [groups...|students...]
glabs report <course> <assignment> [--html|--json]
```

## User handbook

- Getting started: [docs/getting-started.md](docs/getting-started.md)
- Configuration reference: [docs/configuration.md](docs/configuration.md)
- Workflows and recipes: [docs/workflows.md](docs/workflows.md)
- Command reference: [docs/commands.md](docs/commands.md)
- Troubleshooting: [docs/troubleshooting.md](docs/troubleshooting.md)
- Advanced topics: [docs/advanced.md](docs/advanced.md)

## Contributing

Issues and pull requests are welcome.

## Testing

Default unit and contract tests:

```sh
go test ./...
```

Integration tests with GitLab Testcontainers (opt-in):

```sh
# Group/project lifecycle (createGroup, generateProject, …)
GLABS_RUN_GITLAB_TC=1 go test -tags=integration -v -count=1 ./gitlab/... -run TestIntegration_GitLab_GroupAndProjectLifecycle

# Archive, Delete, ProtectToBranch, Setaccess end-to-end
GLABS_RUN_GITLAB_TC=1 go test -tags=integration -v -count=1 ./gitlab/... -run TestIntegration_GitLab_Operations

# Run all integration tests at once
GLABS_RUN_GITLAB_TC=1 go test -tags=integration -v -count=1 ./gitlab/... -run '^TestIntegration_'
```

Notes:

- Integration tests are intentionally opt-in because starting GitLab CE in a container takes 5–25 minutes.
- `GLABS_RUN_GITLAB_TC` means: run GitLab Testcontainer tests.
- Set `GLABS_RUN_GITLAB_TC=1` to enable them; without it the tests are skipped automatically.
- Example: `GLABS_RUN_GITLAB_TC=0` (or variable unset) keeps integration tests disabled.
- In CI, trigger them via the `run_integration` workflow dispatch input (dedicated `test-integration` job).

## License

MIT, see [LICENSE](LICENSE).
