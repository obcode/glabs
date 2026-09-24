# glabs-web

The GraphQL server behind `glabs.cs.hm.edu`. It shares the CLI's core packages
(`config`, `gitlab`, `git`, `reporter`) and adds a web layer under `web/`.

This is the skeleton: authentication and the `me` / `serverInfo` queries. Course
management, GitLab operations and scheduling come next.

## Layout

| Package | Role |
|---|---|
| `cmd/glabs-web` | entry point; sets `time.Local` and build metadata, calls `web/bootstrap` |
| `web/bootstrap` | flags, config, Mongo connection, user seeding, then starts the server |
| `web/graph` | GraphQL: schema, resolvers, the HTTP server, and the auth middleware |
| `web/app` | the core the resolvers delegate to; holds the database |
| `web/db` | MongoDB access (mongo-driver v2) |
| `web/principal` | carries the authenticated user through the request context |

Resolvers stay thin — auth gate plus a call into `web/app` — so the rules live in
one place. `web/` never imports back into `cmd/`, and only `web/bootstrap` reads
viper beyond the config keys below.

## Auth

Identity comes from an auth proxy (oauth2-proxy behind Caddy) that sets
`X-Remote-User` to the verified OIDC email. The server trusts that header and is
**fail-closed on it**: no header is 401. The whole model assumes the server is
reachable *only* through the proxy — never publish its port directly, or the
header can be forged.

With `auth.enabled: false` the server injects a local dev user, so development
needs no proxy.

## Access approval

Authenticated is not approved. Only the `admins` and users an admin approved may
use glabs; everyone acts strictly as their own user (per-user isolation). The
access gate (`web/graph/access_gate.go`) is a gqlgen field middleware that refuses
every root field to an unapproved user except `me`, `serverInfo` and
`requestAccess` — an allowlist of open fields, so a field added later is closed by
default.

1. An unapproved user sees `me.access` = `NONE` and calls
   `requestAccess(reason)`. The request is stored as `PENDING` in the `users`
   table and every admin gets a mail with a link to
   `<server.publicurl>/admin/access?user=<email>`.
2. An admin calls `approveUser` (the user gets a mail), `rejectUser` (the user
   gets a mail) or `revokeUser` (no mail). `resetUser` deletes the row, so the
   user can ask again; a rejected or revoked user cannot ask on their own.
3. Every step is written to the event log (`access-*`).

Admins never need a row — except in **preview mode**: a request carrying
`X-Glabs-Preview: unapproved` treats an admin like anyone else, judged by their own
row and without admin rights. That lets an admin walk the request flow with their
single SSO identity. The header can only lower the caller's own rights, so it needs
no protection by the proxy; the GUI sets it from a cookie.

Admins never need a row otherwise. Mails need SMTP; without it the request is stored and
shown on the admin page anyway. The status is cached for 30 s per process, and
every decision drops the cache entry at once.

## Monitoring & nightly summary

Every operator-relevant thing that happens is written to the `events` collection
(cross-user, 180-day TTL): logins (throttled to one per user per 8h) and rejected
logins, scheduled jobs, job outcomes, interactive operations, and course/token
changes. This is separate from the owner-scoped `activity` log each user sees for
their own courses — `events` is the platform-wide trail an **admin** reads.

Admins are the emails in the `admins` config list — the only privilege above
ordinary owner-scoped access. They get:

- **A nightly summary mail** (`summary.enabled`, default hour 05:00 local): an
  aggregated digest — active users, rejected logins, jobs, operations, and a
  warnings/errors section — never raw log lines. Recipients default to the admins
  list. Needs SMTP configured. The window is `(last-sent, now]`, persisted in the
  `system` collection so a restart never double-sends.
- **Admin GraphQL** (all admin-gated): `platformEvents(since)` (live feed),
  `platformSummary(since, until)` (the same digest, live), and the
  `sendSummaryNow` mutation (send the last 24h on demand). `me.isAdmin` tells the
  GUI whether to show the admin page.

The optional `X-Remote-Department` header (the HM `fhmDepartment` claim, forwarded
by the proxy — see `deploy/`) enriches login events with the faculty number; it is
authorization-irrelevant and absent-safe.

## Running locally

Needs MongoDB. A throwaway instance:

```sh
docker run -d --name glabs-mongo -p 127.0.0.1:27017:27017 mongo:8
```

`.glabs-web.yaml` (in `.` or `$HOME`):

```yaml
db:
  uri: mongodb://localhost:27017
  database: glabs
server:
  port: "8080"
  production: false        # false → GraphQL playground on / and introspection on
auth:
  enabled: false           # local development: every request runs as the dev user
  devuser: you@hm.edu      # optional identity for the dev user
```

Then:

```sh
go run ./cmd/glabs-web
```

Playground on <http://localhost:8080/>, queries on `POST /query`.

```sh
curl -s -X POST http://localhost:8080/query -H 'Content-Type: application/json' \
  -d '{"query":"{ me { email name } serverInfo { version } }"}'
```

After changing the schema, regenerate:

```sh
go generate ./cmd/glabs-web
```

## Config keys

| Key | Purpose |
|---|---|
| `db.uri` | MongoDB connection string (override with `--db-uri`) |
| `db.database` | database name (default `glabs`) |
| `server.port` | listen port (default `8080`) |
| `server.production` | `true` disables the playground and introspection |
| `server.allowedorigins` | CORS origins (default: localhost 5173/8080/3000) |
| `server.publicurl` | base URL of the GUI, for links in the access mails (empty: no link) |
| `auth.enabled` | `false` uses the local dev user; `true` requires the proxy header |
| `auth.header` | identity header (default `X-Remote-User`) |
| `auth.displaynameheader` | display-name header (default `X-Remote-Displayname`) |
| `auth.departmentheader` | optional department header (default `X-Remote-Department`) |
| `auth.devuser` | dev user email when auth is disabled |
| `admins` | emails that are always approved, approve access requests, see the admin monitoring page and get the nightly summary |
| `summary.enabled` | send the nightly admin summary mail (needs SMTP) |
| `summary.hour` | local hour to send the summary (default `5`) |
| `summary.recipient` | optional single recipient; overrides the admins list |
