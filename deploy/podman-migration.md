# Migrationsplan: Deployment von Docker Compose → Podman (`kube play`) auf Alpine

> Status: **Vorschlag / WIP**, noch nicht umgesetzt. Downtime unkritisch (kein
> produktiver Nutzer). Getrennt vom DevContainer-Vorhaben.

## Kontext & Ziel

Der Prod-Stack auf `glabs.cs.hm.edu` läuft heute als **Docker Compose**
([deploy/docker-compose.yml](docker-compose.yml)): Caddy (TLS/Auth-Gate) →
oauth2-proxy (OIDC gegen sso.hm.edu) → `glabs-web` + `gui` + `mongo`, plus zwei
self-hosted GitHub-Actions-Runner für Auto-Deploy. Ziel ist der Umstieg auf
**Podman** (daemonless, rootless-fähig, FOSS) auf dem Alpine-Server.

## Warum `kube play` und nicht Quadlet

Quadlet (Podmans deklarative Unit-Dateien) setzt **systemd** voraus. **Alpine
nutzt OpenRC, nicht systemd** — es gibt kein unterstütztes systemd-Paket. Damit
ist Quadlet auf diesem Server nicht verfügbar (außer man wechselt die Distro,
was für dieses eine Deployment nicht lohnt).

Der portable, deklarative Podman-Weg auf Alpine ist **`podman kube play`**: der
Stack wird als Kubernetes-YAML beschrieben (Pod + Volumes + optional
Secrets/ConfigMaps), Podman spielt es lokal ab. Der Boot-Start erfolgt über ein
**OpenRC-Init-Script** (systemd-Ersatz).

## Zielarchitektur: ein Pod, `localhost`-Networking

Der wichtigste konzeptionelle Unterschied zu Compose: In Kubernetes teilen sich
alle Container **eines Pods** einen Netzwerk-Namespace — sie erreichen einander
über **`localhost:<port>`**, nicht über Service-Namen wie `glabs-web:8080`.
Da die fünf Container **kollisionsfreie Ports** belegen, ist genau ein Pod die
pragmatische Wahl:

| Container | Port (im Pod) |
|---|---|
| mongo | 27017 |
| glabs-web | 8080 |
| gui | 3000 |
| oauth2-proxy | 4180 |
| caddy | 80 / 443 (einzige veröffentlichte Ports) |

**Daraus folgende Referenz-Änderungen** (heute Service-Namen → künftig `localhost`):

- [deploy/Caddyfile](Caddyfile): `oauth2-proxy:4180` → `localhost:4180`,
  `glabs-web:8080` → `localhost:8080`, `reverse_proxy gui:3000` → `localhost:3000`.
- `gui`-Env `GLABS_SERVER`: `http://glabs-web:8080/query` → `http://localhost:8080/query`.
- `.glabs-web.yaml` `db.uri`: `…@mongo:27017…` → `…@localhost:27017…`.
- `.env`-Interpolation entfällt (kube play kann kein `${VAR}`) — siehe Secrets.

> Alternative (mehrere Pods + Services): Podman ist kein voller K8s-Cluster, Cross-
> Pod-Service-Discovery ist fragil. **Ein Pod wird empfohlen.**

## Zu erstellende Dateien

```
deploy/
  k8s/
    glabs.yaml            # Pod (5 Container) + PVCs (mongo-data, caddy-data, caddy-config)
    glabs-secret.yaml     # Secret: OIDC-Secret, Cookie-Secret, Mongo-Creds, ACME-EAB  (gitignored)
    glabs-configmap.yaml  # nicht-geheime Env: SERVER_NAME, PUBLIC_GLABS_SERVER, ACME_EMAIL, URLs
  openrc/
    glabs                 # OpenRC-Init-Script  → /etc/init.d/glabs
```

### `glabs.yaml` (Pod) — Kernpunkte

- `spec.restartPolicy: Always` ersetzt Composes `restart: unless-stopped`.
- **Volumes** → `persistentVolumeClaim` (Podman legt gleichnamige Named Volumes an):
  `mongo-data`, `caddy-data`, `caddy-config`. `Caddyfile` und `.glabs-web.yaml`
  → `hostPath`-Volumes (read-only Mounts der on-host Dateien) — hält Secrets aus
  dem YAML/Git heraus, analog zu heute.
- **Nur Caddy** bekommt `ports:`/`hostPort` (80/443). Die übrigen Container haben
  **keinen** hostPort → nur Pod-intern erreichbar (entspricht dem heutigen
  „nicht veröffentlicht"). Mongo optional zusätzlich `127.0.0.1:27017` für den
  SSH-Tunnel-Zugang (wie heute).
- **Env** aus `configMapRef`/`secretRef` (siehe unten) statt `.env`.
- **Start-Reihenfolge**: In einem Pod starten Container parallel — Composes
  `depends_on: mongo healthy` gibt es nicht. `glabs-web` bricht ab, wenn Mongo
  beim Boot nicht erreichbar ist, wird aber durch `restartPolicy: Always` in
  Sekunden neu gestartet, bis Mongo bereit ist. Sauberer wäre eine
  `readinessProbe` auf mongo + ein kurzer Retry im Backend; für WIP reicht der
  Restart-Loop. (Bewusst dokumentieren.)

### Secrets & ConfigMap

Composes `.env`-Interpolation wird ersetzt:

- **`glabs-secret.yaml`** (gitignored, wie heute `.env`/`.glabs-web.yaml`):
  `OAUTH2_PROXY_CLIENT_SECRET`, `OAUTH2_PROXY_COOKIE_SECRET`, `MONGO_*`,
  `ACME_EAB_KID`, `ACME_EAB_HMAC_KEY`. Per `--secret`/`secretRef` in den Pod.
- **`glabs-configmap.yaml`** (nicht geheim, kann eingecheckt werden):
  `SERVER_NAME`, `PUBLIC_GLABS_SERVER`, `GLABS_SERVER`, `ACME_EMAIL`,
  `ACME_DIRECTORY_URL`, OIDC-Scopes etc.
- `podman kube play --configmap glabs-configmap.yaml glabs.yaml` (Secret analog).

### `openrc/glabs` (Boot-Start, systemd-Ersatz)

Minimales OpenRC-Service-Script:

```sh
#!/sbin/openrc-run
description="glabs stack (podman kube play)"
K8S=/home/glabs/glabs/deploy/k8s

depend() { need net; after firewall; }

start() {
    ebegin "Starting glabs pod"
    podman kube play --replace \
        --configmap "$K8S/glabs-configmap.yaml" \
        "$K8S/glabs.yaml"
    eend $?
}

stop() {
    ebegin "Stopping glabs pod"
    podman kube down "$K8S/glabs.yaml"
    eend $?
}
```

Aktivieren: `rc-update add glabs default`.

## Rootful vs. rootless

Der Stack braucht **:80 und :443** (Caddy, ACME-HTTP-01). Rootless-Podman bindet
Ports <1024 nur mit `sysctl net.ipv4.ip_unprivileged_port_start=80`.

- **Empfehlung für den Server: rootful Podman** (via OpenRC als root) — hält
  Parität zum heutigen (rootful) Docker, keine Linger-/Privileged-Port-Sonderfälle,
  ACME auf :80 funktioniert direkt.
- Der Rootless-Ideal-Fall (FOSS-Purismus) bleibt lokal im DevContainer; auf einem
  dedizierten Single-Purpose-Host wiegt Betriebssicherheit schwerer.

## CI-Deploy-Step umstellen ([.github/workflows/docker.yml](../.github/workflows/docker.yml))

Der `deploy`-Job (self-hosted Runner auf dem Host) ändert sich von `docker
compose` auf `podman kube play`:

- Image-Tag pinnen: heute `sed` auf `GLABS_WEB_TAG` in `.env`. Künftig `sed` auf
  die `image:`-Zeile in `deploy/k8s/glabs.yaml` (oder ein kleines Overlay).
- `docker compose pull glabs-web && docker compose up -d` →
  `podman pull …/glabs-web:$VERSION` dann
  `podman kube play --replace --configmap … glabs.yaml`.
- `--replace` rekonziliert den Pod (nur geänderte Container werden neu erstellt).
- Caddyfile-Änderung → Pod ohnehin über `--replace` neu aufgesetzt.
- `docker image prune -f` → `podman image prune -f`.

## Offener Knackpunkt: die self-hosted Runner

Heute laufen die zwei GH-Runner **als Compose-Services** und mounten den
**Docker-Socket**, um `docker compose` auf dem Host zu fahren
([docker-compose.yml](docker-compose.yml) `gh-runner*`). Unter Podman gibt es
keinen Daemon-Socket im selben Sinn. Zwei Optionen:

1. **Runner nativ auf dem Host** (empfohlen für Alpine): den GitHub-Actions-
   Runner als OpenRC-Service (oder `apk`-Paket/Tarball) direkt installieren, der
   `podman kube play` aufruft. Entkoppelt Deploy-Mechanik von der App, kein
   Socket-Mount, kein `myoung34/github-runner`-Docker-Image mehr.
2. **Runner-Container unter Podman** mit gemountetem rootful Podman-Socket
   (`podman system service`). Näher am Status quo, aber mehr bewegliche Teile
   (Podman-Socket-Aktivierung unter OpenRC).

→ **Entscheidung nötig** (siehe unten). Empfehlung: Option 1.

## Migrationsschritte (Reihenfolge)

1. Podman auf dem Alpine-Host: `apk add podman` (+ `podman-compose` optional als
   Fallback), rootful-Setup, `catatonit`/`fuse-overlayfs` prüfen.
2. `deploy/k8s/*.yaml` + `deploy/openrc/glabs` erstellen; `.gitignore` um
   `glabs-secret.yaml` erweitern.
3. Caddyfile + `.glabs-web.yaml` + gui-`GLABS_SERVER` auf `localhost` umstellen.
4. Alten Stack stoppen: `docker compose down` (Volumes behalten).
5. **Volume-Daten migrieren**: da WIP/leer i.d.R. trivial — sonst
   `mongo-data`/`caddy-data`/`caddy-config` von Docker- nach Podman-Volumes
   kopieren (`podman volume import` bzw. `tar` zwischen den Volume-Mountpoints).
   Caddy holt bei leerem `caddy-data` ein **frisches** Zertifikat (EAB beachten).
6. `rc-update add glabs default && rc-service glabs start`.
7. Deploy-Job in `docker.yml` auf Podman umschreiben; Runner-Variante umsetzen.
8. Docker vom Host entfernen, sobald stabil (optional).

## Verifikation

- `podman pod ps` / `podman ps` → alle 5 Container laufen, nur Caddy hat Ports.
- `curl -I https://glabs.cs.hm.edu` → 302 auf sso.hm.edu-Login (Auth-Gate greift).
- OIDC-Login end-to-end, GraphQL `/query` antwortet, GUI lädt Daten.
- Caddy hat gültiges HM-CA-Zertifikat (`caddy-data` persistiert).
- Reboot-Test: Host neu starten → OpenRC bringt den Pod hoch.
- Release-Test: Tag bauen → Runner deployt via `kube play --replace`, neuer
  `glabs-web`-Tag läuft.

## Rollback

Docker-Compose-Stack bleibt im Repo, bis Podman stabil ist. Rollback =
`rc-service glabs stop` + `docker compose up -d` im `DEPLOY_DIR`. Volumes nicht
löschen, bis der Umstieg bestätigt ist.

## Zu klärende Entscheidungen

1. **Runner-Variante**: nativ auf dem Host (empfohlen) vs. Podman-Container mit Socket.
2. **Rootful** (empfohlen, wg. :80/:443) vs. rootless + sysctl.
3. **Config-Übergabe**: hostPath-Mount von `.glabs-web.yaml`/Caddyfile (näher am
   Status quo) vs. alles in Kubernetes Secret/ConfigMap (deklarativer).
4. `docker-compose.yml` nach erfolgreicher Migration **behalten** (Fallback/Doku)
   oder entfernen — betrifft auch den GUI-Repo-Deploy-Job, der dieselbe Datei sync't.
