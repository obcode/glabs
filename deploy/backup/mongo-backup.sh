#!/bin/sh
# Local rotating MongoDB backup for the glabs deployment.
#
# glabs had no backup at all until 2026-08-23, while plexams and tallox had had one for their
# databases for a long time. What sits in here is not incidental: the course and assignment
# configurations that the whole web interface exists to edit, plus the sealed GitLab tokens.
# Recreating them means asking every lecturer to type their term back in.
#
# Dumps the `glabs` database from the compose `mongo` service into a gzip'd archive on the
# host, verifies it can be read back, then prunes old ones. Local only, no offsite copy --
# this guards against a bad migration or a deleted volume, not against losing the host. Same
# scope and shape as plexams' and tallox' pg-backup.sh.
#
# Run as the deploy user from cron, e.g. daily at 02:30:
#     MAILTO=oliver.braun@hm.edu
#     30 2 * * *  /home/obraun/glabs/deploy/backup/mongo-backup.sh >> /home/obraun/backups/backup.log
#
# Note what is NOT redirected: stderr. The success line goes to the log, anything that goes
# wrong goes to cron and therefore into a mail. `>> log 2>&1` would be tidier and would mean a
# failing backup says nothing at all.
#
# Restore:
#     gzip -dc <archiv> | docker compose exec -T mongo sh -c \
#       'mongorestore --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" \
#        --authenticationDatabase admin --archive --gzip --drop'
#   --drop ersetzt die vorhandenen Sammlungen. Vorher glabs-web anhalten, sonst schreibt es
#   weiter in eine Datenbank, die gerade unter ihm ausgetauscht wird.
set -eu

DEPLOY_DIR="${DEPLOY_DIR:-/home/obraun/glabs/deploy}"
BACKUP_DIR="${BACKUP_DIR:-/home/obraun/backups}"
KEEP_DAILY="${KEEP_DAILY:-14}"
KEEP_WEEKLY="${KEEP_WEEKLY:-8}"
DB_NAME="${DB_NAME:-glabs}"
# Where the Alloy agent picks up textfile metrics. Missing: the run is simply not reported as
# a metric, which is right on a host without an agent.
TEXTFILE_DIR="${TEXTFILE_DIR:-/home/obraun/monitoring-agent/textfile}"

COMPOSE="docker compose -f ${DEPLOY_DIR}/docker-compose.yml"

[ -d "$DEPLOY_DIR" ] || { echo "FEHLER: $DEPLOY_DIR fehlt" >&2; exit 1; }
mkdir -p "$BACKUP_DIR"

stamp=$(date -u +%Y%m%dT%H%M%SZ)
out="${BACKUP_DIR}/glabs-mongo-${stamp}.archive.gz"
tmp="${out}.part"

# The credentials are read from the container's own environment, never from an argv -- `ps`
# shows those to every user on this host.
#
# `set -e` sees only the LAST element of a pipeline. Here that is the redirect, but mongodump
# runs inside `exec -T`, whose exit code does propagate; the explicit check below is still
# worth having because a dump that says nothing and returns 0 looks like success everywhere.
# shellcheck disable=SC2016  # die Variablen sollen IM Container expandieren
$COMPOSE exec -T mongo sh -c \
    'mongodump --quiet --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" \
        --authenticationDatabase admin --db '"$DB_NAME"' --archive --gzip' > "$tmp"

if [ "$(wc -c < "$tmp")" -lt 1024 ]; then
    rm -f "$tmp"
    echo "FEHLER: mongodump lieferte ein leeres Archiv; nichts behalten." >&2
    exit 1
fi

# Read it back before keeping it. An archive that cannot be listed is not a backup, and the
# moment one finds that out must not be the moment one needs it. Same idea as plexams'
# `pg_restore --list`.
# shellcheck disable=SC2016
if ! $COMPOSE exec -T mongo sh -c \
        'mongorestore --quiet --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" \
            --authenticationDatabase admin --archive --gzip --dryRun' < "$tmp" >/dev/null 2>&1; then
    rm -f "$tmp"
    echo "FEHLER: das Archiv liess sich nicht zuruecklesen; nichts behalten." >&2
    exit 1
fi

mv "$tmp" "$out"

# Weekly copies as hard links: same bytes, second name, so the daily pruning cannot take the
# last Monday with it.
[ "$(date -u +%u)" = "1" ] && ln -f "$out" "${BACKUP_DIR}/glabs-mongo-weekly-${stamp}.archive.gz"

prune() {
    # shellcheck disable=SC2012,SC2086  # ls -t sortiert nach Zeit, und $1 SOLL globben
    ls -t ${1} 2>/dev/null | tail -n "+$(($2 + 1))" | while IFS= read -r old; do rm -f "$old"; done
}
prune "${BACKUP_DIR}/glabs-mongo-2*.archive.gz"       "$KEEP_DAILY"
prune "${BACKUP_DIR}/glabs-mongo-weekly-*.archive.gz" "$KEEP_WEEKLY"

bytes=$(wc -c < "$out" | tr -d ' ')

# Make the run visible. Without this the success sits in a log nobody opens, and an ABSENCE is
# invisible: cron says nothing when a script is no longer called at all. As a metric "it ran"
# becomes a value that can go stale, and a stale value raises an alarm.
if [ -d "$TEXTFILE_DIR" ]; then
    tmp_prom="${TEXTFILE_DIR}/.backup.prom.$$"
    {
        echo "# HELP monitoring_backup_last_success_seconds Unix-Zeit des letzten erfolgreichen Laufs."
        echo "# TYPE monitoring_backup_last_success_seconds gauge"
        echo "monitoring_backup_last_success_seconds $(date -u +%s)"
        echo "# HELP monitoring_backup_archive_bytes Groesse des juengsten Archivs je Art."
        echo "# TYPE monitoring_backup_archive_bytes gauge"
        echo "monitoring_backup_archive_bytes{archive=\"mongo\"} ${bytes}"
    } > "$tmp_prom"
    mv "$tmp_prom" "${TEXTFILE_DIR}/backup.prom"
fi

echo "$(date '+%Y-%m-%d %H:%M') backup ok: ${out} (${bytes} B)"
