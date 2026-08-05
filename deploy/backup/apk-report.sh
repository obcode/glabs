#!/bin/sh
# Weekly "are there updates?" notifier for an Alpine deploy host.
#
# There is no Dependabot for Alpine's own packages -- this is the host-OS half of
# that (the container images are watched by Dependabot in the repos). It only
# REPORTS; it never installs. Applying updates -- especially a linux-lts bump, which
# needs a reboot -- stays a manual, watched step.
#
# What it checks:
#   1. Upgradable packages in the current branch  -> `apk version -l '<'`
#   2. A newer Alpine release branch (e.g. v3.24 -> v3.25), by probing the mirror.
# It mails ONLY when there is something to report (no weekly "all clear" noise).
#
# Runs as the deploy user from cron. `apk update` needs root; it is the ONLY
# privileged step THIS SCRIPT takes, and it goes through the narrow doas rule
#     permit nopass <user> cmd apk args update
# Everything else (apk version, wget, msmtp) is unprivileged.
#
# A second rule, `cmd apk args upgrade`, exists for the human applying the update by
# hand -- the script never calls it. Root login over ssh is off on all three hosts, and
# `su -` for a routine package upgrade would mean keeping a root password alive next to
# the key-only logins; a per-command doas rule keeps the blast radius at one command and
# puts the exact invocation in the log. `reboot` is deliberately NOT covered.
#
# Mail goes out via msmtp reading ~/.msmtprc (account `hm`). See msmtprc.example.
#
# Cron (as the deploy user), Mondays 07:00:
#     0 7 * * 1  /home/<user>/deploy/backup/apk-report.sh >> ~/apk-report.log 2>&1

set -eu

# Recipient comes from the environment -- set it in the cron line so this
# general-purpose script hardcodes no address:
#   0 7 * * 1 MAILTO=you@example.org /path/apk-report.sh >> ~/apk-report.log 2>&1
MAILTO="${MAILTO:-}"
[ -n "$MAILTO" ] || { echo "apk-report: MAILTO ist nicht gesetzt (in der Crontab-Zeile setzen)." >&2; exit 2; }
MSMTP_ACCOUNT="${MSMTP_ACCOUNT:-hm}"
# The header From MUST match the authenticated envelope sender, or a strict gateway
# rejects with 554 5.7.1. Take it from ~/.msmtprc (the `from` line) so header and
# envelope always agree; fall back to a neutral noreply if unreadable.
MAILFROM="${MAILFROM:-$(awk '$1=="from"{print $2; exit}' "$HOME/.msmtprc" 2>/dev/null)}"
[ -n "$MAILFROM" ] || MAILFROM="noreply@$(hostname -d 2>/dev/null || hostname)"
MIRROR="${ALPINE_MIRROR:-https://dl-cdn.alpinelinux.org/alpine}"
host="$(hostname)"
# For the "how to apply this" hint in the mail. Root login over ssh is disabled, so the
# hint has to name a path that works from the deploy user's own shell -- FQDN and user
# taken from the host itself, since the three hosts use three different deploy users.
host_fqdn="$(hostname -f 2>/dev/null || true)"
[ -n "$host_fqdn" ] || host_fqdn="$host"
deploy_user="$(id -un)"
now="$(date '+%Y-%m-%d %H:%M')"

# --- 1. refresh the package index (the one privileged step) --------------------------
# A stale index would under-report. If the refresh fails (mirror hiccup, doas not yet
# set up) we still report against the old index and say so, rather than go silent.
# Called as exactly `apk update` (no flags): the doas rule matches the argument vector
#     permit nopass <user> cmd apk args update
# exactly, so any extra flag here would fall through to a password prompt / denial.
upd_warn=""
if ! doas apk update >/dev/null 2>&1; then
    upd_warn="WARN: 'doas apk update' failed -- report is against a possibly stale index."
fi

# --- 2. upgradable packages ----------------------------------------------------------
# `apk version -l '<'` prints an "Installed:/Available:" header line even when empty;
# strip it so "nothing pending" is genuinely the empty string.
pending="$(apk version -l '<' 2>/dev/null | grep -vE '^(Installed|WARNING)' || true)"

# --- 3. newer release branch? --------------------------------------------------------
# The current branch (v3.24) covers point updates 3.24.x already via apk above; here we
# look for the NEXT minor branch existing on the mirror. Probe upward until one is
# missing; the highest that resolves is the newest available branch. No HTML parsing:
# a directory URL returns 200 when the branch exists, non-zero when it does not.
current_rel="$(cat /etc/alpine-release 2>/dev/null || echo '?')"
branch="$(sed -nE 's#.*/alpine/v([0-9]+\.[0-9]+)/.*#\1#p' /etc/apk/repositories | head -1)"
branch_note=""
if [ -n "${branch:-}" ]; then
    maj="${branch%.*}"; min="${branch#*.}"
    newest="$branch"
    k=1
    while [ "$k" -le 6 ]; do
        cand="${maj}.$((min + k))"
        if wget -q -T 15 -O /dev/null "${MIRROR}/v${cand}/main/" 2>/dev/null; then
            newest="$cand"
            k=$((k + 1))
        else
            break
        fi
    done
    [ "$newest" != "$branch" ] && branch_note="Neueres Alpine-Release verfügbar: v${newest} (aktuell v${branch}). Branch-Wechsel in /etc/apk/repositories nötig -- bewusst manuell."
fi

# --- 4. mail only if there is something to say ---------------------------------------
if [ -z "$pending" ] && [ -z "$branch_note" ] && [ -z "$upd_warn" ]; then
    echo "$now $host: keine Updates, keine Mail."
    exit 0
fi

subject="[apk] ${host}: Updates verfügbar"
[ -n "$branch_note" ] && subject="[apk] ${host}: neues Alpine-Release + Pakete"

# Full RFC-822 headers: strict gateways want Date + Message-ID, and the body carries
# umlauts, so declare UTF-8 explicitly (missing charset is another 554 trigger).
date_hdr="$(date -R 2>/dev/null || date '+%a, %d %b %Y %H:%M:%S %z')"
_dom="${MAILFROM##*@}"; [ -n "$_dom" ] || _dom="localhost"
msgid="<apk-report.$(date +%s).$$@${_dom}>"
{
    echo "From: ${MAILFROM}"
    echo "To: ${MAILTO}"
    echo "Subject: ${subject}"
    echo "Date: ${date_hdr}"
    echo "Message-ID: ${msgid}"
    echo "MIME-Version: 1.0"
    echo "Content-Type: text/plain; charset=UTF-8"
    echo "Content-Transfer-Encoding: 8bit"
    echo
    echo "Host:   ${host}"
    echo "Alpine: ${current_rel} (Branch v${branch:-?})"
    echo "Stand:  ${now}"
    [ -n "$upd_warn" ]    && { echo; echo "$upd_warn"; }
    [ -n "$branch_note" ] && { echo; echo "$branch_note"; }
    if [ -n "$pending" ]; then
        echo
        echo "Aktualisierbare Pakete:"
        echo "$pending"
    fi
    echo
    echo "Einspielen von Hand (Root-Login per ssh ist deaktiviert):"
    echo "  ssh ${deploy_user}@${host_fqdn}"
    echo "  doas apk update && doas apk upgrade"
    echo "Fehlt die doas-Regel: 'su -', dann dieselben Befehle ohne 'doas'."
    echo "Hinweis: linux-lts (Kernel) wird erst nach einem Reboot aktiv; 'reboot' braucht 'su -'."
} | msmtp -a "$MSMTP_ACCOUNT" -- "$MAILTO"

echo "$now $host: Report gemailt an ${MAILTO}."
