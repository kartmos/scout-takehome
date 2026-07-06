#!/usr/bin/env bash
# Scout production deploy script.
#
# Runs ON THE APPLICATION SERVER from /opt/scout, invoked by
# .github/workflows/deploy.yml over SSH as:
#
#   /opt/scout/deploy.sh <sha-tag>
#
# Expects, already present in /opt/scout before this script runs:
#   .env.production                real production secrets, mode 0600
#   data/predictions.db             read-only dataset
#   compose.server.yaml.new         staged by the workflow (not yet active)
#   Caddyfile.new                   staged by the workflow (not yet active)
#   release/                        created on first run; holds rollback state
#
# This script never overwrites .env.production, data/predictions.db, or
# Caddy's certificate volumes. It validates the staged Compose config before
# switching anything, pulls new images before stopping old containers,
# records the previous release for rollback, waits for container health with
# a bounded timeout, verifies the public HTTPS endpoint and homepage, and
# restores the previous release automatically if any of that fails.
set -Eeuo pipefail
umask 027

SCOUT_DIR="/opt/scout"
cd "$SCOUT_DIR"

COMPOSE_FILE="compose.server.yaml"
CADDYFILE="Caddyfile"
ENV_FILE=".env.production"
STAGED_COMPOSE="compose.server.yaml.new"
STAGED_CADDYFILE="Caddyfile.new"
RELEASE_DIR="release"
CURRENT_TAG_FILE="$RELEASE_DIR/current_tag"
PREVIOUS_TAG_FILE="$RELEASE_DIR/previous_tag"
BACKUP_COMPOSE="$RELEASE_DIR/previous-compose.server.yaml"
BACKUP_CADDYFILE="$RELEASE_DIR/previous-Caddyfile"
HEALTH_TIMEOUT_SECONDS=180
HEALTH_INTERVAL_SECONDS=5
CURL_TIMEOUT_SECONDS=10

log() { printf '[deploy] %s\n' "$*" >&2; }
die() { printf '[deploy] ERROR: %s\n' "$*" >&2; exit 1; }

[ "$#" -eq 1 ] || die "usage: deploy.sh <sha-tag>"
NEW_TAG="$1"

case "$NEW_TAG" in
sha-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]*) ;;
*) die "release tag '$NEW_TAG' does not match required pattern sha-<hex>" ;;
esac

[ -f "$ENV_FILE" ] || die "$ENV_FILE is missing; see deploystepbystep.md section 7.8"
[ -f "$STAGED_COMPOSE" ] || die "$STAGED_COMPOSE was not uploaded"
[ -f "$STAGED_CADDYFILE" ] || die "$STAGED_CADDYFILE was not uploaded"
[ -f "data/predictions.db" ] || die "data/predictions.db is missing; see deploystepbystep.md section 7.9"

mkdir -p "$RELEASE_DIR"

SCOUT_DOMAIN="$(grep -E '^SCOUT_DOMAIN=' "$ENV_FILE" | head -n1 | cut -d= -f2-)"
[ -n "$SCOUT_DOMAIN" ] || die "SCOUT_DOMAIN is not set in $ENV_FILE"

log "Validating staged Compose config for release $NEW_TAG (rendered config is not printed)"
if ! SCOUT_IMAGE_TAG="$NEW_TAG" docker compose -f "$STAGED_COMPOSE" --env-file "$ENV_FILE" config >/dev/null; then
	die "docker compose config failed for $STAGED_COMPOSE; leaving the current release untouched"
fi

PREVIOUS_TAG=""
if [ -f "$CURRENT_TAG_FILE" ]; then
	PREVIOUS_TAG="$(cat "$CURRENT_TAG_FILE")"
fi

# Snapshot the currently active topology so a failed release can restore both
# the previous tag and the previous compose/Caddy configuration together.
if [ -f "$COMPOSE_FILE" ]; then
	cp -f "$COMPOSE_FILE" "$BACKUP_COMPOSE"
fi
if [ -f "$CADDYFILE" ]; then
	cp -f "$CADDYFILE" "$BACKUP_CADDYFILE"
fi

restore_previous() {
	if [ -n "$PREVIOUS_TAG" ] && [ -f "$BACKUP_COMPOSE" ] && [ -f "$BACKUP_CADDYFILE" ]; then
		log "Restoring previous release $PREVIOUS_TAG and its topology"
		cp -f "$BACKUP_COMPOSE" "$COMPOSE_FILE"
		cp -f "$BACKUP_CADDYFILE" "$CADDYFILE"
		SCOUT_IMAGE_TAG="$PREVIOUS_TAG" docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" \
			up -d --remove-orphans || true
	else
		log "No previous release recorded; nothing to roll back to automatically. Manual diagnosis required."
	fi
}

log "Pulling images for release $NEW_TAG"
if ! SCOUT_IMAGE_TAG="$NEW_TAG" docker compose -f "$STAGED_COMPOSE" --env-file "$ENV_FILE" pull api web; then
	die "docker compose pull failed for $NEW_TAG; the previous release keeps running"
fi

log "Switching compose.server.yaml and Caddyfile into place"
mv -f "$STAGED_COMPOSE" "$COMPOSE_FILE"
mv -f "$STAGED_CADDYFILE" "$CADDYFILE"

log "Starting release $NEW_TAG"
if ! SCOUT_IMAGE_TAG="$NEW_TAG" docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" \
	up -d --remove-orphans; then
	log "docker compose up failed for $NEW_TAG"
	restore_previous
	die "deployment of $NEW_TAG failed at 'up'"
fi

log "Waiting for containers to report healthy (timeout ${HEALTH_TIMEOUT_SECONDS}s)"
elapsed=0
healthy=false
while [ "$elapsed" -lt "$HEALTH_TIMEOUT_SECONDS" ]; do
	api_status="$(docker inspect -f '{{.State.Health.Status}}' scout-server-api-1 2>/dev/null || echo unknown)"
	web_status="$(docker inspect -f '{{.State.Health.Status}}' scout-server-web-1 2>/dev/null || echo unknown)"
	caddy_status="$(docker inspect -f '{{.State.Health.Status}}' scout-server-caddy-1 2>/dev/null || echo unknown)"
	if [ "$api_status" = "healthy" ] && [ "$web_status" = "healthy" ] && [ "$caddy_status" = "healthy" ]; then
		healthy=true
		break
	fi
	sleep "$HEALTH_INTERVAL_SECONDS"
	elapsed=$((elapsed + HEALTH_INTERVAL_SECONDS))
done

if [ "$healthy" != "true" ]; then
	log "Containers did not become healthy within ${HEALTH_TIMEOUT_SECONDS}s (api=$api_status web=$web_status caddy=$caddy_status)"
	restore_previous
	die "deployment of $NEW_TAG failed its health check"
fi

log "Verifying public HTTPS endpoints (response bodies are not logged)"
if ! curl -fsS -o /dev/null --max-time "$CURL_TIMEOUT_SECONDS" "https://${SCOUT_DOMAIN}/api/healthz"; then
	log "Public API health check failed"
	restore_previous
	die "public health check failed for $NEW_TAG"
fi
if ! curl -fsS -o /dev/null --max-time "$CURL_TIMEOUT_SECONDS" "https://${SCOUT_DOMAIN}/"; then
	log "Public homepage check failed"
	restore_previous
	die "public homepage check failed for $NEW_TAG"
fi

if [ -n "$PREVIOUS_TAG" ]; then
	echo "$PREVIOUS_TAG" >"$PREVIOUS_TAG_FILE"
fi
echo "$NEW_TAG" >"$CURRENT_TAG_FILE"

log "Deployment of $NEW_TAG succeeded."
