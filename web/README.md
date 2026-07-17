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
**fail-closed**: no header is 401, a user not on the allowlist is 403. The whole
model assumes the server is reachable *only* through the proxy — never publish its
port directly, or the header can be forged.

With `auth.enabled: false` the server injects a local dev user, so development
needs no proxy.

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
| `auth.enabled` | `false` uses the local dev user; `true` requires the proxy header |
| `auth.header` | identity header (default `X-Remote-User`) |
| `auth.displaynameheader` | display-name header (default `X-Remote-Displayname`) |
| `auth.devuser` | dev user email when auth is disabled |
| `auth.seedusers` | allowlist seeded only when the users collection is empty |
