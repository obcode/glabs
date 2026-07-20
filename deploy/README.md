# glabs.cs.hm.edu — Deployment

Self-hosting stack for **glabs-web** (GraphQL backend) + **glabs.gui** (SvelteKit) behind
**Caddy** + **oauth2-proxy** (OIDC against `sso.hm.edu`), with **MongoDB** for storage.

```
internet ──443/TLS──> caddy ─┬─ forward_auth → oauth2-proxy ──OIDC──> sso.hm.edu
                             ├─ /oauth2/*   → oauth2-proxy   (login/callback)
                             ├─ sets X-Remote-User (verified email) on every route
                             ├─ /query      → glabs-web:8080  (GraphQL + WS subscriptions)
                             └─ /            → gui:3000        (SvelteKit; incl. YAML download)
     glabs-web / gui / mongo / oauth2-proxy ── internal compose network only (never published)
```

Only Caddy publishes ports (80/443, plus Mongo on `127.0.0.1:27017` for an SSH tunnel).
The backend trusts the `X-Remote-User` header Caddy sets, so it must **never** be reachable
except through Caddy — that is why no service other than Caddy has a public `ports:` mapping.

## Prerequisites

- A host with Docker + Docker Compose, reachable at your `SERVER_NAME` (DNS A/AAAA record).
- **Port 80 reachable by the HM ACME CA** for the HTTP-01 challenge (also does HTTP→HTTPS).
- An **OIDC client** registered with the HM IdP team (Zentrale IT), redirect URI
  `https://<SERVER_NAME>/oauth2/callback`.
- A **fresh EAB** (`kid` + `hmac`) for Caddy's ACME client — see the ACME note below.
- The published images: `ghcr.io/obcode/glabs-web` and `ghcr.io/obcode/glabs.gui` (built by
  each repo's `docker.yml` on release).

## Setup

```sh
cp .env.example .env
cp .glabs-web.yaml.example .glabs-web.yaml
```

Fill both in:

1. **`.env`** — `SERVER_NAME`, Mongo credentials, the `ACME_*`/EAB values, the `OAUTH2_PROXY_*`
   client id/secret, and a cookie secret (`openssl rand -base64 24`). The two GUI URLs default
   correctly; only change them if you know why.
2. **`.glabs-web.yaml`** — set `db.uri` to the SAME Mongo credentials as in `.env`, add the
   allow-listed users under `auth.seedusers`, generate `secrets.key` (`openssl rand -base64 32`),
   and set `gitlab.host`. SMTP and ZPA are optional (commented out).

Then:

```sh
docker compose up -d
docker compose logs -f caddy      # watch the first certificate issuance
```

Caddy fetches the TLS certificate itself on first start and stores it in the `caddy-data`
volume; renewal is automatic.

## ACME / EAB (HM CA)

TLS is handled entirely by Caddy's built-in ACME client against the HM CA using an **External
Account Binding**. The `ACME_*` values in `.env` come from HM Zentrale IT.

> **Request a FRESH EAB for Caddy.** The HM EAB binds exactly **one** ACME account. Reusing an
> EAB that `acme.sh` (or anything else) has already registered fails at new-account with
> `401 urn:ietf:params:acme:error:unauthorized`. Ask for a dedicated `kid`/`hmac` for this host.

The `caddy-data` volume holds the ACME account + issued certificates — **keep it**; deleting it
forces a fresh registration.

## Identity model (the SSR trap)

Caddy authenticates every request and injects `X-Remote-User` (the verified email) on **all**
routes, including `/`. glabs identity **is** the email; there are no roles/groups (a single
Owner per course/assignment), so no department header is needed.

The GUI needs **two** backend URLs because only one hop carries the OIDC cookie:

- `PUBLIC_GLABS_SERVER` (`https://<SERVER_NAME>/query`) — the browser bundle (SSR-hydrated
  client + `wss://` subscriptions); it has the cookie and goes through Caddy.
- `GLABS_SERVER` (`http://glabs-web:8080/query`) — the SvelteKit SSR/`load()`/`/api` hop, which
  runs in the gui container **without** a cookie. It reaches the backend internally (bypassing
  oauth2-proxy) and relays the `X-Remote-User` Caddy injected into its page request.

If `GLABS_SERVER` pointed at the public URL, oauth2-proxy would bounce the cookieless SSR call
to the `sso.hm.edu` login page and the GUI would try to parse that HTML as GraphQL (HTTP 500).

## Operating

```sh
docker compose pull && docker compose up -d       # update to the newest images (:latest)
docker compose up -d glabs-web                     # roll one service (e.g. after GLABS_WEB_TAG change)
docker compose logs -f glabs-web                   # backend logs
docker compose down                                # stop (volumes are kept)
```

### Von außen an die MongoDB (SSH-Tunnel)

Mongo is bound to `127.0.0.1:27017` on the host only. To connect a GUI (Compass) from your
machine:

```sh
ssh -L 27017:127.0.0.1:27017 <user>@<host>
# then: mongodb://<MONGO_USER>:<MONGO_PASSWORD>@localhost:27017/?authSource=admin
```

## Automatischer Deploy (self-hosted runners)

CI builds and pushes the images on GitHub-hosted runners (`docker.yml`). The deploy host is
VPN-internal, so GitHub can't push into it — instead a **self-hosted runner on the host** polls
GitHub outbound and, on a release, runs `docker compose` locally. Each repo's `docker.yml` has
a gated `deploy` job (`runs-on: [self-hosted, glabs-deploy]`):

- **glabs** (backend) — syncs `docker-compose.yml` + `Caddyfile` into `$DEPLOY_DIR` (secrets and
  the `caddy-data` volume untouched), pins `GLABS_WEB_TAG` in `.env`, `docker compose pull
  glabs-web` + `up -d`, and force-recreates Caddy if the Caddyfile changed.
- **glabs.gui** — pins `GUI_TAG` and rolls only the `gui` service.

Because `obcode` is a personal account (no org), runners are **repo-scoped**: one per repo,
sharing the label `glabs-deploy`. Both run as containers in this compose under the `runner`
profile.

### One-time host setup

1. Set `DEPLOY_DIR` and `GH_RUNNER_PAT` in `.env` (see `.env.example`). `GH_RUNNER_PAT` is a
   classic PAT with `repo` + `workflow` (covers both repos).
2. Start the runners (the `runner` profile keeps them out of the default stack, so a deploy's
   `docker compose up -d` never restarts them mid-job):
   ```sh
   docker compose --profile runner up -d gh-runner gh-runner-gui
   ```
3. Confirm both appear under **Settings → Actions → Runners** in each repo (idle, label
   `glabs-deploy`).
4. Flip the gate: set the repo variable **`AUTO_DEPLOY=true`** in **both** `obcode/glabs` and
   `obcode/glabs.gui` (Settings → Secrets and variables → Actions → Variables). Optionally set
   `DEPLOY_DIR` there too if it differs from the default. Until `AUTO_DEPLOY=true`, images are
   still built and pushed — only the deploy step is skipped.

From then on, every release rolls itself out. Manual rollback stays available: set an older
`GLABS_WEB_TAG`/`GUI_TAG` in `.env` and `docker compose up -d glabs-web` (or `gui`).

## Notes

- **Rollback:** set `GLABS_WEB_TAG` / `GUI_TAG` in `.env` to an older release tag and
  `docker compose up -d glabs-web` (or `gui`).
- **Backups:** back up the `mongo-data` volume (courses, users, scheduled jobs, activity log)
  and `.glabs-web.yaml` (`secrets.key` — without it, stored GitLab tokens can't be decrypted).
- **Local testing without SSO:** set `auth.enabled: false` in `.glabs-web.yaml` — every request
  then runs as a local dev user. Never do this on a public host.
