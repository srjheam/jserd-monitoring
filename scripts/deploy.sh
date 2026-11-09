#!/usr/bin/env bash
# Builds the current master, ships it to the VM over SSH (no registry on the
# Always Free tier), protects the curation data, starts the new version, and
# rolls back automatically when the new version does not come up healthy.
#
# Assumes ticket 07 (guided VM setup) has already run once, so the following
# exist on the VM:
#   - the SSH host alias below, configured in the developer's ~/.ssh/config
#   - $REMOTE_DIR (relative to the VM user's home directory)
#   - $REMOTE_DIR/.env, the app's environment file (owner-only permissions)
#
# This script itself ships deploy/docker-compose.vm.yml to the VM on every run,
# since ticket 07 only provisions the host, the environment file, and Docker -
# it does not place the compose file.
#
# The reverse proxy configuration is different: the live Caddyfile is
# hand-managed on the VM at $REMOTE_DIR/Caddyfile, so that other sites can be
# served from the same host without this script overwriting them. This script
# places deploy/Caddyfile.example only when the VM has no Caddyfile at all.
# Keep the example in step with the VM by hand when you change either one.
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)

DEPLOY_HOST=${DEPLOY_HOST:-jserd-vm}
REMOTE_DIR=${REMOTE_DIR:-jserd}
IMAGE_NAME=${IMAGE_NAME:-tcc-jserd-monitor}
COMPOSE_PROJECT=${COMPOSE_PROJECT:-jserd}
KEEP=${KEEP:-5}
KEEP_ARCHIVES=${KEEP_ARCHIVES:-67}
HEALTH_TIMEOUT_SECONDS=${HEALTH_TIMEOUT_SECONDS:-180}

die() {
  echo "deploy.sh: $1" >&2
  exit 1
}

check_clean_tree() {
  if [ -n "$(git -C "$ROOT_DIR" status --porcelain)" ]; then
    die "working tree is dirty. Commit or stash your changes before deploying."
  fi
}

check_branch() {
  local branch
  branch=$(git -C "$ROOT_DIR" rev-parse --abbrev-ref HEAD)
  if [ "$branch" != "master" ]; then
    die "current branch is '$branch', not 'master'. Deploy only ships master."
  fi
}

check_docker() {
  command -v docker >/dev/null 2>&1 || die "docker is not installed on this machine."
  docker info >/dev/null 2>&1 || die "docker is installed but not usable (is the daemon running?)."
}

check_ssh_alias() {
  ssh -o BatchMode=yes -o ConnectTimeout=8 "$DEPLOY_HOST" true >/dev/null 2>&1 \
    || die "SSH alias '$DEPLOY_HOST' does not resolve or is unreachable. Configure it in ~/.ssh/config (see ticket 07)."
}

check_env_file() {
  ssh -o BatchMode=yes "$DEPLOY_HOST" "test -f '$REMOTE_DIR/.env'" \
    || die "'$REMOTE_DIR/.env' is absent on the VM. Create it once (see ticket 07) before deploying."
}

main() {
  check_clean_tree
  check_branch
  check_docker
  check_ssh_alias
  check_env_file

  local short_sha
  short_sha=$(git -C "$ROOT_DIR" rev-parse --short HEAD)
  local tag="${IMAGE_NAME}:${short_sha}"

  echo "Building ${tag}..."
  docker build -t "$tag" "$ROOT_DIR"

  echo "Shipping ${tag} to ${DEPLOY_HOST}..."
  docker save "$tag" | gzip | ssh "$DEPLOY_HOST" "gunzip | docker load"

  echo "Placing compose files on ${DEPLOY_HOST}..."
  # shellcheck disable=SC2029  # variables are intentionally expanded locally: none of them are secrets
  ssh "$DEPLOY_HOST" "mkdir -p '$REMOTE_DIR/archives'"
  scp -q "$ROOT_DIR/deploy/docker-compose.vm.yml" "$DEPLOY_HOST:$REMOTE_DIR/"

  # Never overwrite a Caddyfile the VM already has: it is the hand-managed
  # source of truth and may serve sites this repo knows nothing about.
  if ssh -o BatchMode=yes "$DEPLOY_HOST" "test -f '$REMOTE_DIR/Caddyfile'"; then
    echo "Keeping the existing Caddyfile on ${DEPLOY_HOST} (hand-managed)."
  else
    echo "No Caddyfile on ${DEPLOY_HOST} yet; placing deploy/Caddyfile.example..."
    scp -q "$ROOT_DIR/deploy/Caddyfile.example" "$DEPLOY_HOST:$REMOTE_DIR/Caddyfile"
  fi

  echo "Deploying ${tag} on ${DEPLOY_HOST}..."
  # shellcheck disable=SC2029  # variables are intentionally expanded locally: none of them are secrets
  ssh "$DEPLOY_HOST" \
    "REMOTE_DIR='$REMOTE_DIR' COMPOSE_PROJECT='$COMPOSE_PROJECT' IMAGE_NAME='$IMAGE_NAME' NEW_TAG='$short_sha' KEEP='$KEEP' KEEP_ARCHIVES='$KEEP_ARCHIVES' HEALTH_TIMEOUT='$HEALTH_TIMEOUT_SECONDS' COMMIT='$short_sha' bash -s" \
    < "$ROOT_DIR/scripts/deploy-remote.sh"

  echo "Deployed commit ${short_sha}"
}

main "$@"
