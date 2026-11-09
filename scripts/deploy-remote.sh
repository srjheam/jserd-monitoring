#!/usr/bin/env bash
# Runs on the VM, invoked over SSH by scripts/deploy.sh. Never invoke directly.
#
# Expects these environment variables (set by scripts/deploy.sh):
#   REMOTE_DIR, COMPOSE_PROJECT, IMAGE_NAME, NEW_TAG, KEEP, KEEP_ARCHIVES,
#   HEALTH_TIMEOUT, COMMIT
set -euo pipefail

: "${REMOTE_DIR:?}" "${COMPOSE_PROJECT:?}" "${IMAGE_NAME:?}" "${NEW_TAG:?}" \
  "${KEEP:?}" "${KEEP_ARCHIVES:?}" "${HEALTH_TIMEOUT:?}" "${COMMIT:?}"

cd "$HOME/$REMOTE_DIR"

ENV_FILE="$PWD/.env"
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE not found on VM" >&2; exit 1; }
export ENV_FILE

STATE_FILE=".deployed_tag"
PREV_TAG=""
[ -f "$STATE_FILE" ] && PREV_TAG=$(cat "$STATE_FILE")

compose() {
  docker compose -p "$COMPOSE_PROJECT" -f docker-compose.vm.yml "$@"
}

# docker-compose.vm.yml requires APP_IMAGE and ENV_FILE to interpolate, so
# every compose subcommand needs them set, including this stop: without it,
# compose fails to parse the file and the app is never actually stopped,
# and the backup below races the still-running database.
export APP_IMAGE="${IMAGE_NAME}:${NEW_TAG}"

# Stop the app container before touching its data volume, so the backup
# captures a consistent database. Ignore failure: nothing to stop on a first
# deploy.
compose stop app 2>/dev/null || true

# Archive only PocketBase's own databases. Curation data (notes, states,
# categories, verifications) exists only there - everything else in the
# volume (file storage, collector run dumps) rebuilds from OJS. The app is
# already stopped above, so data.db/auxiliary.db are quiescent. Every file
# is existence-checked: on a first deploy to a fresh volume, PocketBase has
# never run, so none of them exist yet, and there is nothing to archive.
mkdir -p archives
TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
ARCHIVE_NAME="${TIMESTAMP}-${COMMIT}.tar.gz"
VOLUME="${COMPOSE_PROJECT}_pb_data"
docker run --rm \
  -v "${VOLUME}:/data:ro" \
  -v "$PWD/archives:/backup" \
  alpine:3.20 \
  sh -c 'cd /data && set --
    for f in data.db data.db-wal data.db-shm auxiliary.db auxiliary.db-wal auxiliary.db-shm; do
      [ -f "$f" ] && set -- "$@" "$f"
    done
    [ "$#" -eq 0 ] && exit 0
    tar czf "/backup/'"${ARCHIVE_NAME}"'" "$@"'

# Keep only the KEEP_ARCHIVES most recent archives.
# shellcheck disable=SC2012  # archive filenames never contain special characters
ls -1t archives/*.tar.gz 2>/dev/null | tail -n +$((KEEP_ARCHIVES + 1)) | xargs -r rm -f --

# Start the new version. APP_PORT and CADDY_HOSTNAMES are read from the
# current shell env if already set on the VM; otherwise the compose file's
# own defaults apply. APP_IMAGE and ENV_FILE were exported above.
compose up -d

CONTAINER_ID=$(compose ps -a -q app)

healthy=false
elapsed=0
status=unknown
while [ "$elapsed" -lt "$HEALTH_TIMEOUT" ]; do
  status=$(docker inspect --format '{{.State.Health.Status}}' "$CONTAINER_ID" 2>/dev/null || echo unknown)
  if [ "$status" = healthy ]; then
    healthy=true
    break
  fi
  running=$(docker inspect --format '{{.State.Running}}' "$CONTAINER_ID" 2>/dev/null || echo false)
  [ "$running" = true ] || break
  sleep 2
  elapsed=$((elapsed + 2))
done

if [ "$healthy" != true ]; then
  echo "ERROR: ${IMAGE_NAME}:${NEW_TAG} did not become healthy within ${HEALTH_TIMEOUT}s (status: $status)" >&2
  docker logs "$CONTAINER_ID" >&2 || true
  if [ -n "$PREV_TAG" ]; then
    echo "Rolling back to ${IMAGE_NAME}:${PREV_TAG}" >&2
    export APP_IMAGE="${IMAGE_NAME}:${PREV_TAG}"
    compose up -d
  else
    echo "No previous image recorded; nothing to roll back to." >&2
  fi
  exit 1
fi

echo "$NEW_TAG" > "$STATE_FILE"

# Keep only the 5 most recent images for this app (docker images lists newest
# first by default).
docker images "$IMAGE_NAME" --format '{{.Tag}}' | tail -n +$((KEEP + 1)) | while read -r old_tag; do
  [ "$old_tag" = "<none>" ] && continue
  docker rmi "${IMAGE_NAME}:${old_tag}" >/dev/null 2>&1 || true
done

echo "Healthy: ${IMAGE_NAME}:${NEW_TAG}"
