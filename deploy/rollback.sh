#!/usr/bin/env bash
# Scout production rollback script.
#
# Runs ON THE APPLICATION SERVER from /opt/scout. Restores the previously
# recorded release (compose.server.yaml, Caddyfile, and image tag) written by
# deploy.sh's release/ bookkeeping, then performs the same bounded health and
# public-endpoint checks as a normal deployment.
#
#   /opt/scout/rollback.sh
#
# Never retags `latest`: it only ever switches to the exact immutable
# sha-<hex> tag recorded in release/previous_tag by the last successful
# deploy.sh run.
set -Eeuo pipefail
umask 027

SCOUT_DIR="/opt/scout"
cd "$SCOUT_DIR"

COMPOSE_FILE="compose.server.yaml"
CADDYFILE="Caddyfile"
ENV_FILE=".env.production"
RELEASE_DIR="release"
CURRENT_TAG_FILE="$RELEASE_DIR/current_tag"
PREVIOUS_TAG_FILE="$RELEASE_DIR/previous_tag"
BACKUP_COMPOSE="$RELEASE_DIR/previous-compose.server.yaml"
BACKUP_CADDYFILE="$RELEASE_DIR/previous-Caddyfile"
HEALTH_TIMEOUT_SECONDS=180
HEALTH_INTERVAL_SECONDS=5
CURL_TIMEOUT_SECONDS=10

log() { printf '[rollback] %s\n' "$*" >&2; }
die() { printf '[rollback] ERROR: %s\n' "$*" >&2; exit 1; }

[ "$#" -eq 0 ] || die "usage: rollback.sh (takes no arguments; uses $PREVIOUS_TAG_FILE)"
[ -f "$ENV_FILE" ] || die "$ENV_FILE is missing"
[ -f "$PREVIOUS_TAG_FILE" ] || die "no previous release recorded in $PREVIOUS_TAG_FILE; nothing to roll back to"

ROLLBACK_TAG="$(cat "$PREVIOUS_TAG_FILE")"
case "$ROLLBACK_TAG" in
sha-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]*) ;;
*) die "recorded previous tag '$ROLLBACK_TAG' is not a valid immutable sha-<hex> tag" ;;
esac

CURRENT_TAG=""
if [ -f "$CURRENT_TAG_FILE" ]; then
	CURRENT_TAG="$(cat "$CURRENT_TAG_FILE")"
fi
if [ "$ROLLBACK_TAG" = "$CURRENT_TAG" ]; then
	die "recorded previous tag '$ROLLBACK_TAG' is the same as the current tag; nothing to roll back"
fi

SCOUT_DOMAIN="$(grep -E '^SCOUT_DOMAIN=' "$ENV_FILE" | head -n1 | cut -d= -f2-)"
[ -n "$SCOUT_DOMAIN" ] || die "SCOUT_DOMAIN is not set in $ENV_FILE"

if [ -f "$BACKUP_COMPOSE" ] && [ -f "$BACKUP_CADDYFILE" ]; then
	log "Restoring the topology recorded alongside $ROLLBACK_TAG"
	cp -f "$BACKUP_COMPOSE" "$COMPOSE_FILE"
	cp -f "$BACKUP_CADDYFILE" "$CADDYFILE"
else
	log "No backed-up topology found; reusing the current compose.server.yaml/Caddyfile with tag $ROLLBACK_TAG"
fi

[ -f "$COMPOSE_FILE" ] || die "$COMPOSE_FILE is missing; cannot roll back"

log "Validating Compose config for rollback tag $ROLLBACK_TAG (rendered config is not printed)"
if ! SCOUT_IMAGE_TAG="$ROLLBACK_TAG" docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" config >/dev/null; then
	die "docker compose config failed for rollback tag $ROLLBACK_TAG"
fi

log "Starting rollback release $ROLLBACK_TAG"
if ! SCOUT_IMAGE_TAG="$ROLLBACK_TAG" docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" \
	up -d --remove-orphans; then
	die "docker compose up failed while rolling back to $ROLLBACK_TAG"
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
	die "rollback to $ROLLBACK_TAG failed its health check (api=$api_status web=$web_status caddy=$caddy_status); manual diagnosis required"
fi

log "Verifying public HTTPS endpoints (response bodies are not logged)"
if ! curl -fsS -o /dev/null --max-time "$CURL_TIMEOUT_SECONDS" "https://${SCOUT_DOMAIN}/api/healthz"; then
	die "public health check failed after rolling back to $ROLLBACK_TAG; manual diagnosis required"
fi
if ! curl -fsS -o /dev/null --max-time "$CURL_TIMEOUT_SECONDS" "https://${SCOUT_DOMAIN}/"; then
	die "public homepage check failed after rolling back to $ROLLBACK_TAG; manual diagnosis required"
fi

# Swap bookkeeping so a second rollback invocation can move forward again.
if [ -n "$CURRENT_TAG" ]; then
	echo "$CURRENT_TAG" >"$PREVIOUS_TAG_FILE"
fi
echo "$ROLLBACK_TAG" >"$CURRENT_TAG_FILE"

log "Rollback to $ROLLBACK_TAG succeeded."
