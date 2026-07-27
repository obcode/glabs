# Migrationsplan: Docker Compose → **k3s** (echtes Kubernetes) auf `glabs.cs.hm.edu`

> Status: **Vorschlag / WIP**. Vollständiger Ersatz von Docker durch k3s — **keine
> Koexistenz**. Downtime unkritisch (Semesterferien). Zusätzliches Ziel: Lern-/
> Lehrobjekt für „Verteilte Softwaresysteme und Plattformen" (VSSP).
>
> **Ersetzt** `deploy/podman-migration.md` für das *Server-Deployment*. Der
> DevContainer (lokale Entwicklung) bleibt unberührt (Podman/Docker lokal).

## Kontext & Ausgangslage

VM (geprüft 2026-07-26): 4 vCPU (AMD EPYC 7402), **8 GB RAM** (~6,9 GB frei), 93 GB
Disk (82 GB frei), **cgroup v2 unified**, Kernel 6.18-lts, **QEMU/KVM** (echte VM).
glabs-Stack braucht heute nur **~340 MB** RAM → für k3s **massig Platz**.

Heutiger Stack (Docker Compose): `caddy` (öffentlich 80/443, TLS via HM-CA-ACME+EAB,
`forward_auth` → oauth2-proxy, injiziert `X-Remote-User`) · `oauth2-proxy` (OIDC
sso.hm.edu) · `glabs-web` (:8080) · `gui` (:3000) · `mongo` (:27017) · 2 GH-Runner.
Laufende Tags: `glabs-web:v3.41.0`, `gui:v1.39.0`.

## Getroffene Entscheidungen (2026-07-26)

- **Edge/TLS: Traefik + Traefik-eigener ACME-Resolver** (kein cert-manager). Traefik
  ist in k3s eingebaut; sein ACME-Resolver kann **EAB**.
- **HM-EAB ist Self-Service** → frischen EAB ziehen, **kein** `caddy-data`/`acme.json`-
  Migrationsproblem. Caddy wird komplett durch Traefik ersetzt.
- **ghcr-Images public** → **kein** `imagePullSecret`.
- **Testsystem: MacBook Air mit Debian/Ubuntu** (bequeme k3s-Installation), siehe unten.
- **Callback-URL bleibt** `https://glabs.cs.hm.edu/oauth2/callback` (hängt nur an
  Hostname + Pfad; Edge-Proxy ist für die IdP-Registrierung transparent) → **keine**
  Shibboleth-Neuregistrierung für Prod nötig.

## Leitentscheidung: Traefik als Ingress, Auth als Middleware-Kette

Bei **echtem** k8s wird jede Komponente eigenes **Deployment + Service**. Nenne ich
die Services wie die Compose-Services (`glabs-web`, `gui`, `oauth2-proxy`, `mongo`),
liefert Cluster-DNS dieselben Namen → **`GLABS_SERVER=http://glabs-web:8080/query`**
und **`db.uri: …@mongo:27017`** bleiben unverändert.

Der Edge wechselt von Caddy zu **Traefik** (IngressRoute + Middleware CRDs):

- **TLS:** Traefik `certResolver` (ACME gegen HM-CA, **EAB** via `kid`/`hmac`),
  `acme.json` auf einer **PVC** persistiert. Hostname `glabs.cs.hm.edu` unverändert.
- **Routing (IngressRoutes):**
  - `` PathPrefix(`/oauth2/`) `` → Service `oauth2-proxy:4180` (Login/Callback, **nicht** auth-gated).
  - `` PathPrefix(`/query`) `` → Service `glabs-web:8080` (mit Auth-Middleware-Kette).
  - `` PathPrefix(`/`) `` → Service `gui:3000` (mit Auth-Middleware-Kette).

### Der Kern-Arbeitsposten: Auth-Kette in Traefik nachbauen (Security-Grenze!)

Caddy macht heute zweierlei, das exakt und **verifiziert** repliziert werden muss:

1. **`forward_auth` + Redirect-on-401.** Traefik-`ForwardAuth`-Middleware →
   `http://oauth2-proxy:4180/oauth2/auth`, `authResponseHeaders` =
   `X-Auth-Request-Email, X-Auth-Request-Preferred-Username, X-Auth-Request-Groups`,
   `trustForwardHeader: true`. **Wichtig/knifflig:** Caddy leitet bei 401 explizit auf
   `/oauth2/start?rd=…` um. Traefik-ForwardAuth gibt 401 per Default nur zurück — der
   401→Redirect-auf-Login-Pfad muss gebaut und getestet werden (oauth2-proxy
   `/oauth2/start`). **→ genau das zuerst auf der Testbox verifizieren.**
2. **Header-Hygiene + -Benennung.** Client-`X-Remote-*`/`X-Auth-Request-*` am Eingang
   **strippen** (Headers-Middleware), damit nichts fälschbar ist; die autoritativen
   Werte setzt ForwardAuth aus der oauth2-proxy-Antwort. Caddy *renamed* zusätzlich
   `X-Auth-Request-Email→X-Remote-User` etc. — Traefik kann das nicht elegant.
   **Sauberer Weg:** die vom Backend gelesenen Header-Namen auf die oauth2-proxy-Namen
   umstellen (z.B. `auth.header: X-Auth-Request-Email` in `.glabs-web.yaml`) statt zu
   renamen. Prüfen, welche Header-Namen (User/Displayname/Department) konfigurierbar sind.

> Diese Kette ist der eigentliche Grund fürs Testsystem: **nicht** zuerst an Prod bauen.

## Ziel-Objekte (Namespace `glabs`)

| Komponente | k8s-Objekt | Netzwerk | Storage |
|---|---|---|---|
| mongo | **StatefulSet** (1 Replica) | Service `mongo` :27017 | PVC `mongo-data` (local-path) |
| glabs-web | Deployment | Service `glabs-web` :8080 | Secret-Mount `.glabs-web.yaml` |
| gui | Deployment | Service `gui` :3000 | — |
| oauth2-proxy | Deployment | Service `oauth2-proxy` :4180 | — |
| **Traefik** | k3s-Addon (HelmChartConfig) | Entrypoints 80/443 (Host) | PVC für `acme.json` |
| Auth | Traefik **Middlewares** (ForwardAuth + Headers) + **IngressRoutes** | — | — |

**Config/Secrets:**
- `Secret glabs-web-config` → Datei `/app/.glabs-web.yaml` (secrets.key, SMTP, Mongo-Creds; ggf. `auth.header` angepasst).
- `Secret oauth2` → `OAUTH2_PROXY_CLIENT_SECRET`, `_COOKIE_SECRET`.
- `Secret mongo` → root user/passwort.
- `Secret traefik-acme-eab` → EAB `kid`/`hmac` für den certResolver.
- `ConfigMap app-env` → `SERVER_NAME`, `PUBLIC_GLABS_SERVER`, `GLABS_SERVER`, OIDC-Issuer/Scopes, `OAUTH2_PROXY_REDIRECT_URL` (unverändert).
- ghcr public → **kein** imagePullSecret.

**Startreihenfolge:** kein `depends_on`; `glabs-web` crash-restartet bis `mongo`
`Ready` ist (Deployment-Default). Optional readinessProbe/initContainer-Wait.

## Dateien im Repo

```
deploy/k8s/
  kustomization.yaml
  namespace.yaml
  mongo-statefulset.yaml         + service
  glabs-web-deployment.yaml      + service
  gui-deployment.yaml            + service
  oauth2-proxy-deployment.yaml   + service
  traefik-helmchartconfig.yaml   # EAB-certResolver + acme.json-Persistenz (k3s-Addon konfigurieren)
  ingressroutes.yaml             # /oauth2, /query, /
  middlewares.yaml               # forward-auth + header-strip
  configmap-app-env.yaml
  secrets.example/               # Vorlagen; echte Secrets via `kubectl create secret` (gitignored)
```

Empfehlung: **Kustomize** (`kubectl -k`) — deklarativ, `images:` pinnt Tags zentral.

## k3s-Installation (Alpine/OpenRC, Prod-VM)

cgroup v2 ✓, `cgroups`-Service läuft ✓, KVM ✓. **Traefik NICHT disablen** (wir nutzen es):

```sh
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--write-kubeconfig-mode 644" sh -
rc-update add k3s default && rc-service k3s start
kubectl get nodes            # -> Ready
```

Das k3s-Traefik-Addon wird über eine **`HelmChartConfig`** (in `kube-system`)
angepasst: EAB-`certResolver`, `acme.json`-PVC-Persistenz, Entrypoints. (Alternative:
gebündeltes Traefik disablen und eigenes Traefik-Helm-Release deployen — mehr
Kontrolle, mehr Setup. Für den Start: HelmChartConfig.)

Zu prüfen (kann ich nicht testen): Node wird `Ready`; nftables/iptables-Backend bei
Netzproblemen; optional `swapoff -a`.

## Auto-Deploy (CI) umstellen — `.github/workflows/docker.yml`

Docker weg → `deploy`-Job nutzt **kubectl (Rolling-Update)**:

```sh
kubectl -n glabs set image deployment/glabs-web glabs-web=ghcr.io/obcode/glabs-web:$VERSION
kubectl -n glabs rollout status deployment/glabs-web --timeout=120s
```

- Rollback: `kubectl -n glabs rollout undo deployment/glabs-web`. GUI-Repo analog `deployment/gui`.
- Runner: **host-nativ** (OpenRC-Service statt Docker-Runner-Container) mit kubeconfig-Zugriff.
- Evolutionsstufe (Lehre): **GitOps** (Argo CD reconciled das Repo) statt Push-Deploy.

## Migrationsreihenfolge (Downtime OK)

1. **Sichern:** `docker exec deploy-mongo-1 mongodump --archive=/tmp/glabs.dump --gzip` → auf Host kopieren. Tags notieren.
2. **k3s installieren** (s.o.), Node `Ready`.
3. **Traefik konfigurieren** (HelmChartConfig: EAB-certResolver + acme.json-PVC).
4. **Namespace + Secrets + ConfigMaps** (aus bestehenden `.env`/`.glabs-web.yaml`-Werten; frischen EAB eintragen).
5. **mongo** (StatefulSet + PVC), dann `mongorestore --archive --gzip`.
6. **glabs-web, gui, oauth2-proxy** deployen (`kubectl apply -k deploy/k8s`).
7. **IngressRoutes + Middlewares** (Auth-Kette) — die vorher auf der Testbox verifizierte Variante.
8. **End-to-end verifizieren** (s.u.).
9. **CI-Deploy** auf kubectl umstellen, Host-Runner einrichten.
10. **Docker entfernen:** `docker compose down` (Volumes erst nach bestätigter Migration löschen), `rc-service docker stop`, `rc-update del docker`, `apk del docker docker-cli docker-compose`.

## Testsystem: MacBook Air (Intel) mit Debian/Ubuntu

**Zweck:** die **Auth-Middleware-Kette** und die Manifeste gefahrlos proben, bevor Prod
angefasst wird; zugleich **Demo-/Laborcluster** für VSSP.

- **Distro:** Debian 12 oder Ubuntu Server 24.04 → k3s per `curl -sfL https://get.k3s.io | sh -` (systemd, trivial). **Traefik NICHT disablen.**
- **Hardware:** RAM prüfen (4 GB knapp-machbar, 8 GB komfortabel); **USB-Ethernet** statt WLAN (Broadcom-Treiber auf Intel-Macs zickig).
- **Grenzen off-campus:** `acme.hm.edu` und ggf. der OIDC-Redirect sind evtl. nur im HM-Netz. Auf dem Air daher **Routing + Auth-Header-Kette + Manifeste** testen mit **Let's-Encrypt-*staging*** oder self-signed/ohne TLS. Echtes HM-Zert + echter OIDC-Login → final auf der Prod-VM.
- **OIDC auf der Testbox:** eigener Hostname → Callback nicht registriert. Entweder **zweite Redirect-URI** beim HM-IdP registrieren, oder Auth-Kette ohne echten IdP-Handshake proben.
- **Optional Stufe B:** **k3d**/**Colima** auf dem Hauptrechner für 30-Sekunden-Wegwerf-Cluster beim Iterieren an einzelnen Manifesten.

## Verifikation

- `kubectl get pods -n glabs` alle `Ready`; Traefik terminiert TLS, Zert für `glabs.cs.hm.edu` gültig.
- `curl -I https://glabs.cs.hm.edu` → 302 auf sso.hm.edu-Login (Auth-Gate greift).
- OIDC-Login end-to-end, `/query` antwortet, GUI lädt, YAML-Download ok, **X-Remote-*-Injektion korrekt** (Client kann nichts fälschen).
- Reboot-Test: VM neu → k3s (OpenRC) bringt Cluster hoch.
- Release-Test: Tag → `kubectl set image` → `rollout status` grün.
- Persistenz: `mongo-data` + `acme.json`-PVC überstehen Neustart.

## Rollback

Solange Docker installiert (Schritt 10 zuletzt): `rc-service k3s stop`, `docker compose up -d`. Daten im `mongodump`. Alte Docker-Volumes erst nach bestätigter Stabilität löschen.

## Offene Entscheidungen

1. **Traefik bundled + HelmChartConfig** (Start, empfohlen) vs. **eigenes Traefik-Helm-Release** (mehr Kontrolle).
2. Backend-**Header-Namen anpassen** (`auth.header` etc. auf `X-Auth-Request-*`) vs. Rename-Konstrukt in Traefik.
3. **mongo** StatefulSet (empfohlen) vs. Deployment+PVC.
4. **Zweite Redirect-URI** für die Testbox registrieren vs. Auth-Kette dort ohne echten OIDC proben.
5. **GitOps (Argo CD)** als nächste Lehrstufe nach der funktionierenden Migration.

## Lehrbezug (VSSP)

Erzeugt „nebenbei" ein reales Kurslabor: Namespaces, Deployments/StatefulSets,
Services + Cluster-DNS, PVCs/StorageClass, ConfigMaps/Secrets, **Ingress
(Traefik IngressRoute) + Middlewares (ForwardAuth)**, ACME/TLS mit EAB,
Rolling-Updates/Rollbacks, Probes — Ausbaustufe **GitOps/Argo CD**. Die glabs-
Manifeste sind ein echtes, nicht-triviales Beispiel statt „nginx-Hello-World".
