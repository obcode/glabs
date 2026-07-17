# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`glabs` is a Cobra-based CLI (Go 1.24+) for managing GitLab repositories for student assignments at scale: generating per-student/per-group repos, seeding them from starter code, protecting branches, setting access, cloning, and reporting.

## Commands

```sh
go install .                 # build/install from local checkout into $GOPATH/bin
go test ./...                # all unit + contract tests (fast, no network)
go test ./config/...         # tests for a single package
go test ./config/ -run TestName   # a single test
gofmt -w <file> && go vet ./... && golangci-lint run   # what pre-commit / CI enforce
go vet -tags=integration ./...   # compile-check integration tests — plain go vet/test skip them
```

> The `integration` build tag hides those tests from `go test ./...` and `go vet ./...`, so a signature change can break them invisibly and only surface on `main` (where the integration job runs). Always `go vet -tags=integration ./...` after changing a signature the integration tests call. CI's fast-test job now does this too.

Integration tests spin up GitLab CE in Testcontainers and are **opt-in** (startup takes 5–25 min). They are gated by both the `integration` build tag and the `GLABS_RUN_GITLAB_TC=1` env var:

```sh
GLABS_RUN_GITLAB_TC=1 go test -tags=integration -v -count=1 ./gitlab/... -run '^TestIntegration_'
```

In CI (`.github/workflows/ci.yml`) integration tests run only on `main` or via `workflow_dispatch` with `run_integration=true`.

## Conventions

- **Conventional Commits are required**: releases are automated by go-semantic-release on push to `main`, which bumps the version, prepends to `CHANGELOG.md`, and runs goreleaser. Use `feat:`, `fix:`, etc. with scopes (e.g. `feat(startercode): ...`). Do not hand-edit `CHANGELOG.md`.
- Run `gofmt`, `go vet ./...`, and `golangci-lint run` before committing — these are pre-commit hooks and CI gates. gitleaks also runs in pre-commit.

## Architecture

Three layers, separated by package:

1. **`cmd/`** — one file per subcommand (`generate.go`, `protect.go`, `clone.go`, etc.), each registered on `rootCmd` via `init()`. Commands are thin: they call `config.Get*Config(...)` to build a config struct, often print it and ask for confirmation, then call a `gitlab.Client` method. `root.go` wires up zerolog logging and viper config loading.

2. **`config/`** — loads and resolves YAML config into typed structs (see `config/types.go`, especially `AssignmentConfig`). This package owns all viper access and config interpretation; the rest of the code consumes the resulting structs. `Student`/`Group` resolution, access levels, URLs, seeder/release/startercode settings are all built here.

3. **`gitlab/`** and **`git/`** — the action layer. `gitlab/` wraps the GitLab API client (`gitlab.com/gitlab-org/api/client-go/v2`) via the `Client` type and performs GitLab-side operations (groups, projects, branches, protect, issues, approvals, reports). `git/` handles local git operations (clone, push, prepare source repo from starter code) using `go-git`.

### Config loading model

Config is layered via **viper across multiple files**, set up in `cmd/root.go` `initConfig()`:
- Main config `~/.glabs.yaml` declares `gitlab.host`/`token`, `coursesfilepath`, and a list of `courses`.
- Each course name is merged in from `<coursesfilepath>/<course>.yml`.
- Config keys are addressed as `course.assignment.<field>` (e.g. `viper.GetString(course + "." + assignment + ".containerRegistry")`). `GetAssignmentConfig` validates the course/assignment exist before reading.

A `generate` (and most mutating commands) flow: command builds `AssignmentConfig` → optionally `git.PrepareSourceRepo` for starter code → `Client.Generate` resolves/creates the GitLab group → dispatches to `generatePerGroup` or `generatePerStudent` based on the `per:` setting.

### Testing patterns

- **Contract tests** (`*_contract_test.go` in `gitlab/`) run against an `httptest` mock server via `newContractClient` (in `contract_test_helpers_test.go`), which disables the retryable HTTP wrapper so 5xx mocks don't hang. These run in the default `go test ./...`.
- **Integration tests** (`integration` build tag) exercise real GitLab CE in containers.

## Documentation

User-facing handbook lives in `docs/` (getting-started, configuration, commands, workflows, troubleshooting, advanced). `README.md` is the entry point. Update the relevant `docs/` page when changing user-visible behavior or config fields.
