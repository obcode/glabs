# glabs-web — the GraphQL server behind glabs.cs.hm.edu.
#
# The repo builds TWO binaries from one Go module (the `glabs` CLI at `.` and
# `glabs-web` at `./cmd/glabs-web`); this image builds ONLY the web server.
# The mail templates are embedded via //go:embed (web/mail/tmpl), so no assets
# need to be copied into the runtime stage.
FROM golang:1.26-alpine AS builder
WORKDIR /src

# Version metadata, passed by docker.yml from the release tag (mirrors the
# goreleaser ldflags used for the CLI). .git is excluded via .dockerignore, so
# main.go's debug.ReadBuildInfo VCS fallback cannot fill these — hence the ARGs.
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

# Cache the module graph separately from the source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off → a static binary that runs on the bare alpine runtime below.
RUN CGO_ENABLED=0 GOOS=linux go build \
	-trimpath \
	-ldflags "-s -w -X 'main.version=${VERSION}' -X 'main.commit=${COMMIT}' -X 'main.date=${DATE}' -X 'main.builtBy=docker'" \
	-o /out/glabs-web ./cmd/glabs-web

# --- Runtime ---
FROM alpine:3.21
# ca-certificates: TLS to GitLab / ZPA / SMTP. tzdata: main() sets time.Local to
# Europe/Berlin at startup and needs the zoneinfo database.
RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -u 10001 glabs
WORKDIR /app
COPY --from=builder /out/glabs-web /usr/local/bin/glabs-web
USER glabs
EXPOSE 8080
# .glabs-web.yaml is searched in "." (this WORKDIR) first, then $HOME — the
# deploy bind-mounts it read-only into /app. server.port defaults to 8080.
ENTRYPOINT ["glabs-web"]
