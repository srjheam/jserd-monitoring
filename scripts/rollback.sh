#!/usr/bin/env bash
# Restarts the app on the VM with a previously deployed image tag. Runs
# against the VM alone (no local build, no git checks) so it works even when
# the developer's machine cannot build or push a new image.
#
# Usage:
#   scripts/rollback.sh          # roll back to the second-most-recent tag
#   scripts/rollback.sh <tag>    # roll back to a specific retained tag
#
# Assumes ticket 07 (guided VM setup) has already run once, so the SSH host
# alias below is configured in the developer's ~/.ssh/config.
set -euo pipefail

DEPLOY_HOST=${DEPLOY_HOST:-jserd-vm}
REMOTE_DIR=${REMOTE_DIR:-jserd}
IMAGE_NAME=${IMAGE_NAME:-tcc-jserd-monitor}
COMPOSE_PROJECT=${COMPOSE_PROJECT:-jserd}
HEALTH_TIMEOUT_SECONDS=${HEALTH_TIMEOUT_SECONDS:-180}

die() {
  echo "rollback.sh: $1" >&2
  exit 1
}

check_ssh_alias() {
  ssh -o BatchMode=yes -o ConnectTimeout=8 "$DEPLOY_HOST" true >/dev/null 2>&1 \
    || die "SSH alias '$DEPLOY_HOST' does not resolve or is unreachable. Configure it in ~/.ssh/config (see ticket 07)."
}

main() {
  check_ssh_alias

  local target_tag="${1:-}"
  if [ -n "$target_tag" ] && [[ ! "$target_tag" =~ ^[A-Za-z0-9._-]+$ ]]; then
    die "'$target_tag' is not a valid image tag (image tags are git short SHAs: letters, digits, '.', '_', '-')."
  fi

  # shellcheck disable=SC2029  # variables are intentionally expanded locally: none of them are secrets
  ssh "$DEPLOY_HOST" \
    "REMOTE_DIR='$REMOTE_DIR' COMPOSE_PROJECT='$COMPOSE_PROJECT' IMAGE_NAME='$IMAGE_NAME' TARGET_TAG='$target_tag' HEALTH_TIMEOUT='$HEALTH_TIMEOUT_SECONDS' bash -s" \
    < "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/rollback-remote.sh"
}

main "$@"
