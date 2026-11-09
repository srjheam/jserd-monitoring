#!/usr/bin/env bash
# Runs on the VM, invoked over SSH by scripts/rollback.sh. Never invoke directly.
#
# Expects these environment variables (set by scripts/rollback.sh):
#   REMOTE_DIR, COMPOSE_PROJECT, IMAGE_NAME, TARGET_TAG, HEALTH_TIMEOUT
# TARGET_TAG may be empty, meaning "the tag before the one currently running".
set -euo pipefail

: "${REMOTE_DIR:?}" "${COMPOSE_PROJECT:?}" "${IMAGE_NAME:?}" "${HEALTH_TIMEOUT:?}"
TARGET_TAG=${TARGET_TAG:-}

cd "$HOME/$REMOTE_DIR"

ENV_FILE="$PWD/.env"
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE not found on VM" >&2; exit 1; }

STATE_FILE=".deployed_tag"
CURRENT_TAG=""
[ -f "$STATE_FILE" ] && CURRENT_TAG=$(cat "$STATE_FILE")

# Tags of images still present on the VM, newest first (the deploy keeps the
# 5 most recent, per scripts/deploy-remote.sh).
mapfile -t TAGS < <(docker images "$IMAGE_NAME" --format '{{.Tag}}' | grep -v '^<none>$')

[ "${#TAGS[@]}" -gt 0 ] || { echo "ERROR: no retained images for ${IMAGE_NAME} on this VM" >&2; exit 1; }

if [ -z "$TARGET_TAG" ]; then
  # Default: the tag before the currently running one.
  index=-1
  for i in "${!TAGS[@]}"; do
    if [ "${TAGS[$i]}" = "$CURRENT_TAG" ]; then
      index=$i
      break
    fi
  done
  if [ "$index" -ge 0 ]; then
    if [ $((index + 1)) -lt "${#TAGS[@]}" ]; then
      TARGET_TAG=${TAGS[$((index + 1))]}
    else
      echo "ERROR: ${IMAGE_NAME}:${CURRENT_TAG} is already the oldest retained image; nothing to roll back to" >&2
      exit 1
    fi
  elif [ "${#TAGS[@]}" -gt 1 ]; then
    # Current tag unknown (no .deployed_tag yet, or it was pruned): fall
    # back to the second-most-recent retained image.
    TARGET_TAG=${TAGS[1]}
  else
    echo "ERROR: only one retained image (${TAGS[0]}); nothing to roll back to" >&2
    exit 1
  fi
else
  found=false
  for t in "${TAGS[@]}"; do
    [ "$t" = "$TARGET_TAG" ] && { found=true; break; }
  done
  "$found" || { echo "ERROR: tag '${TARGET_TAG}' is not one of the retained images: ${TAGS[*]}" >&2; exit 1; }
fi

compose() {
  docker compose -p "$COMPOSE_PROJECT" -f docker-compose.vm.yml "$@"
}

echo "Rolling back to ${IMAGE_NAME}:${TARGET_TAG}..."
export APP_IMAGE="${IMAGE_NAME}:${TARGET_TAG}"
export ENV_FILE
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
  echo "ERROR: ${IMAGE_NAME}:${TARGET_TAG} did not become healthy within ${HEALTH_TIMEOUT}s (status: $status)" >&2
  docker logs "$CONTAINER_ID" >&2 || true
  exit 1
fi

echo "$TARGET_TAG" > "$STATE_FILE"

echo "Running: ${IMAGE_NAME}:${TARGET_TAG}"
