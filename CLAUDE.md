# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`glabs` is a Cobra-based CLI (Go 1.26+) for managing GitLab repositories for student assignments at scale: generating per-student/per-group repos, seeding them from starter code, protecting branches, setting access, cloning, and reporting.

## Commands

```sh
go install .                 # build/install from local checkout into $GOPATH/bin
go test ./...                # all unit + contract tests (fast, no network)
go test ./config/...         # tests for a single package
go test ./config/ -run TestName   # a single test
gofmt -w <file> && go vet ./... && golangci-lint run   # what pre-commit / CI enforce
go vet -tags=integration ./...   # compile-check integration tests — plain go vet/test skip them

# Store contract tests (web/db). They skip without a database; in the dev container
# PostgreSQL is already running, so pointing the variable at it is all it takes:
GLABS_TEST_PG_URI=postgres://glabs@localhost:5432/glabs?sslmode=disable \
  go test ./web/db/... -count=1

# After editing web/db/queries/*.sql or web/db/migrations/*.sql:
sqlc generate && sqlc diff        # diff is what CI checks
GLABS_VET_DB_URL=postgres://glabs@localhost:5432/glabs?sslmode=disable sqlc vet
```

> The `integration` build tag hides those tests from `go test ./...` and `go vet ./...`, so a signature change can break them invisibly and only surface on `main` (where the integration job runs). Always `go vet -tags=integration ./...` after changing a signature the integration tests call. CI's fast-test job now does this too.

Integration tests spin up GitLab CE in Testcontainers and are **opt-in** (startup takes 5–25 min). They are gated by both the `integration` build tag and the `GLABS_RUN_GITLAB_TC=1` env var:

```sh
GLABS_RUN_GITLAB_TC=1 go test -tags=integration -v -count=1 ./gitlab/... -run '^TestIntegration_'
```

In CI (`.github/workflows/ci.yml`) integration tests run only on `main` or via `workflow_dispatch` with `run_integration=true`.

## Conventions

- **Conventional Commits are required**: releases are automated by go-semantic-release on push to `main`, which bumps the version, publishes a GitHub Release with generated notes, and runs goreleaser. Use `feat:`, `fix:`, etc. with scopes (e.g. `feat(startercode): ...`). There is no `CHANGELOG.md` — the GitHub Releases page is the changelog. (It was removed because committing it back to `main` raced when two release-triggering PRs merged in quick succession; avoid merging such PRs seconds apart regardless.)
- Run `gofmt`, `go vet ./...`, and `golangci-lint run` before committing — these are pre-commit hooks and CI gates. gitleaks also runs in pre-commit.

## Architecture

Three layers, separated by package:

1. **`cmd/`** — one file per subcommand (`generate.go`, `protect.go`, `clone.go`, etc.), each registered on `rootCmd` via `init()`. Commands are thin: they call `config.Get*Config(...)` to build a config struct, often print it and ask for confirmation, then call a `gitlab.Client` method. `root.go` wires up zerolog logging and viper config loading.

2. **`config/`** — loads and resolves YAML config into typed structs (see `config/types.go`, especially `AssignmentConfig`). This package owns all viper access and config interpretation; the rest of the code consumes the resulting structs. `Student`/`Group` resolution, access levels, URLs, seeder/release/startercode settings are all built here.

3. **`gitlab/`** and **`git/`** — the action layer. `gitlab/` wraps the GitLab API client (`gitlab.com/gitlab-org/api/client-go/v2`) via the `Client` type and performs GitLab-side operations (groups, projects, branches, protect, issues, approvals, reports). `git/` handles local git operations (clone, push, prepare source repo from starter code) using `go-git`. Both report progress through the `reporter` package (a `ConsoleReporter` on the CLI) and return errors rather than exiting.

### Two binaries, one module

`.` builds the `glabs` CLI. `./cmd/glabs-web` builds `glabs-web`, the GraphQL server behind glabs.cs.hm.edu, which shares the core packages above and adds a web layer under `web/` (see `web/README.md`):

- `web/bootstrap` — flags, config, the database, then starts the server
- `web/graph` — gqlgen schema, resolvers, HTTP server, auth middleware
- `web/app` — the core the resolvers delegate to; holds the database
- `web/db` — persistence. **`db.PG` (pgx/v5 + sqlc + goose) is the only implementation**; production has run it since 2026-09-23 and the MongoDB one was removed on 2026-10-13. The shared types and constants live in the unsuffixed files (`courses.go`, `jobs.go`, …) next to their `*_pg.go` implementations. Schema lives in `web/db/migrations/` (goose, embedded in the binary and applied at startup), queries in `web/db/queries/`, generated code in `web/db/sqlc/` (committed; `sqlc diff` in CI). `sqlc.yaml` is at the repo root. Same stack and conventions as plexams.go and tallox.go.
- `web/principal` — the authenticated user in the request context

Import rule: `web/` may import the core packages, never the reverse, and `web/` never imports `cmd/`. Resolvers stay thin (auth gate + delegate to `web/app`). Regenerate GraphQL code with `go generate ./cmd/glabs-web` after editing `web/graph/*.graphqls`; `gqlgen.yml` is at the repo root. Auth is fail-closed on a proxy-injected `X-Remote-User` header; `auth.enabled: false` uses a local dev user. Authenticated is not approved: `web/graph/access_gate.go` refuses every root field except `me`, `serverInfo` and `requestAccess` to users who are neither in `admins` nor approved in the `users` table (PostgreSQL only) — a new root field is closed by default; open it there only if an unapproved user must reach it.

### Config loading model

Config is layered via **viper across multiple files**, set up in `cmd/root.go` `initConfig()`:
- Main config `~/.glabs.yaml` declares `gitlab.host`/`token`, `coursesfilepath`, and a list of `courses`.
- Each course name is merged in from `<coursesfilepath>/<course>.yml`.
- Config keys are addressed as `course.assignment.<field>` (e.g. `viper.GetString(course + "." + assignment + ".containerRegistry")`). `GetAssignmentConfig` validates the course/assignment exist before reading.

A `generate` (and most mutating commands) flow: command builds `AssignmentConfig` → optionally `git.PrepareSourceRepo` for starter code → `Client.Generate` resolves/creates the GitLab group → dispatches to `generatePerGroup` or `generatePerStudent` based on the `per:` setting.

### Testing patterns

- **Contract tests** (`*_contract_test.go` in `gitlab/`) run against an `httptest` mock server via `newContractClient` (in `contract_test_helpers_test.go`), which disables the retryable HTTP wrapper so 5xx mocks don't hang. These run in the default `go test ./...`.
- **Store contract tests** (`web/db/storetest`) are one suite written against the behaviour the app relies on. It was written against MongoDB first, while that was still in production, and then pointed at PostgreSQL — that equivalence is what made the switch a config change rather than a leap of faith. With MongoDB gone it no longer proves two implementations agree; it holds PostgreSQL to what the app was written against, so an assertion that looks arbitrary is probably load-bearing. They are **not** behind the `integration` tag: they skip unless `GLABS_TEST_PG_URI` is set, and CI additionally sets `GLABS_TEST_PG_REQUIRED=1` so a database that fails to come up breaks the build instead of silently skipping. `internal/pgtest` gives each test its own database from a template. Deliberately no Testcontainers: the dev container mounts no Docker socket, and one mechanism has to work in both places.
- **Integration tests** (`integration` build tag) exercise real GitLab CE in containers.

## Documentation

User-facing handbook lives in `docs/` (getting-started, configuration, commands, workflows, troubleshooting, advanced). `README.md` is the entry point. Update the relevant `docs/` page when changing user-visible behavior or config fields.
