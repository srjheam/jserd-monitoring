#!/usr/bin/env bash
# Lists or downloads a database backup archive from the VM. The archives live
# on the same volume as the database (deploy/docker-compose.vm.yml's pb_data
# volume), so keeping a copy elsewhere is the only way to survive that volume
# being lost.
#
# Usage:
#   scripts/fetch-backup.sh          # list archives on the VM, newest first
#   scripts/fetch-backup.sh <index>  # download the Nth listed archive (1 = newest)
#   scripts/fetch-backup.sh <name>   # download an archive by its exact filename
#
# Downloads land in the current directory. Assumes ticket 07 (guided VM
# setup) has already run once, so the SSH host alias below is configured in
# the developer's ~/.ssh/config.
set -euo pipefail

DEPLOY_HOST=${DEPLOY_HOST:-jserd-vm}
REMOTE_DIR=${REMOTE_DIR:-jserd}

die() {
  echo "fetch-backup.sh: $1" >&2
  exit 1
}

check_ssh_alias() {
  ssh -o BatchMode=yes -o ConnectTimeout=8 "$DEPLOY_HOST" true >/dev/null 2>&1 \
    || die "SSH alias '$DEPLOY_HOST' does not resolve or is unreachable. Configure it in ~/.ssh/config (see ticket 07)."
}

list_archives() {
  # shellcheck disable=SC2029  # variables are intentionally expanded locally: none of them are secrets
  ssh "$DEPLOY_HOST" "cd '$REMOTE_DIR' && ls -1t archives/*.tar.gz 2>/dev/null"
}

main() {
  check_ssh_alias

  local archives
  archives=$(list_archives) || true
  [ -n "$archives" ] || die "no archives found on ${DEPLOY_HOST}:${REMOTE_DIR}/archives"

  local selector="${1:-}"

  if [ -z "$selector" ]; then
    echo "$archives" | nl -ba -w2 -s'  '
    return
  fi

  local target
  if [[ "$selector" =~ ^[0-9]+$ ]]; then
    target=$(echo "$archives" | sed -n "${selector}p")
    [ -n "$target" ] || die "no archive at index ${selector}"
  else
    target=$(echo "$archives" | grep -Fx "archives/${selector}" || echo "$archives" | grep -Fx "$selector" || true)
    [ -n "$target" ] || die "no archive named '${selector}' found on the VM"
  fi

  echo "Fetching ${target}..."
  scp -q "$DEPLOY_HOST:$REMOTE_DIR/$target" .
  echo "Saved $(basename "$target") to $(pwd)"
}

main "$@"
