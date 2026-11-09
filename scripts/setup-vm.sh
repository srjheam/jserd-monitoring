#!/usr/bin/env bash
#
# One-time guided setup for the OCI VM that runs the JSERD monitor.
# Walks a human through the browser/console steps only they can do (OCI
# security list, reserved IP, DuckDNS), then performs the scriptable parts
# over SSH (Docker install, environment file). After this finishes,
# scripts/deploy.sh has everything it needs on the VM.
#
# Safe to re-run: every stage checks existing state before changing anything.
#
# Everything above the "STAGES" marker is the wizard library: do not hand-edit
# it. Author the per-step stages below the marker.

set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────
# Wizard library: delightful, consistent UX, identical across every wizard.
# ──────────────────────────────────────────────────────────────────────────

if [[ -t 1 ]] && command -v tput >/dev/null 2>&1 && [[ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]]; then
  BOLD=$(tput bold); DIM=$(tput dim); RESET=$(tput sgr0)
  BLUE=$(tput setaf 4); GREEN=$(tput setaf 2); YELLOW=$(tput setaf 3); RED=$(tput setaf 1)
else
  BOLD=""; DIM=""; RESET=""; BLUE=""; GREEN=""; YELLOW=""; RED=""
fi

# Author sets this at the top of the stages section.
TOTAL_STAGES=0

_STAGE_INDEX=0
ENV_FILE="${ENV_FILE:-.env}"
WRITTEN_ENV=() # KEYs written to ENV_FILE this run
SKIPPED=()     # things a stage couldn't finish (e.g. connectivity, unconfirmed step)

# _clear wipes the terminal so only the current step is on screen. No-op when
# output isn't a terminal, so piped logs stay readable.
_clear() {
  [[ -t 1 ]] || return 0
  if command -v tput >/dev/null 2>&1; then tput clear; else printf '\033[2J\033[3J\033[H'; fi
}

# banner "Title" shows the opening frame: what this wizard does.
banner() {
  _clear
  printf '\n%s%s  %s%s\n' "$BOLD" "$BLUE" "$1" "$RESET"
  printf '%s  %s stages%s\n\n' "$DIM" "$TOTAL_STAGES" "$RESET"
  printf '%s  You drive the browser; this wizard tells you exactly what to do and\n' "$DIM"
  printf '  captures the values you copy back. Stop any time with Ctrl-C and re-run\n'
  printf '  later, since it remembers values already saved.%s\n' "$RESET"
  pause "Ready to start?"
}

# stage "Name" clears the screen, then announces a stage and shows progress.
# Clearing keeps only the current step on screen.
stage() {
  _clear
  _STAGE_INDEX=$((_STAGE_INDEX + 1))
  printf '\n%s%s▸ Stage %s/%s · %s%s\n' \
    "$BOLD" "$BLUE" "$_STAGE_INDEX" "$TOTAL_STAGES" "$1" "$RESET"
}

# say "..." prints a plain instruction line.
say()  { printf '  %s\n' "$1"; }
# step "..." is a numbered-feeling action the human takes in the browser.
step() { printf '  %s•%s %s\n' "$BLUE" "$RESET" "$1"; }
note() { printf '  %s%s%s\n' "$DIM" "$1" "$RESET"; }
warn() { printf '  %s⚠ %s%s\n' "$YELLOW" "$1" "$RESET"; }

# open_url URL opens it in the human's browser, cross-platform incl. WSL.
open_url() {
  local url="$1"
  printf '  %s↗ opening%s %s\n' "$GREEN" "$RESET" "$url"
  { if   command -v wslview     >/dev/null 2>&1; then wslview "$url"
    elif command -v explorer.exe >/dev/null 2>&1; then explorer.exe "$url"
    elif command -v xdg-open    >/dev/null 2>&1; then xdg-open "$url"
    elif command -v open        >/dev/null 2>&1; then open "$url"
    else warn "couldn't open a browser; visit it manually: $url"; fi
  } >/dev/null 2>&1 || warn "couldn't open a browser, so visit it manually: $url"
}

# pause "msg" waits for the human to confirm they've done the manual part.
pause() {
  printf '  %s%s%s ' "$DIM" "${1:-Press Enter to continue}" "$RESET"
  read -r _ || true
}

# confirm "question" is a y/N gate; returns success on yes.
confirm() {
  local reply=""
  printf '  %s? %s [y/N] ' "$YELLOW" "$1"
  read -r reply || true
  [[ "$reply" =~ ^[Yy] ]]
}

# _existing KEY: current value of KEY in ENV_FILE, if any.
_existing() {
  [[ -f "$ENV_FILE" ]] || return 1
  local line; line=$(grep -E "^${1}=" "$ENV_FILE" | tail -n1) || return 1
  printf '%s' "${line#*=}"
}

# ask KEY "Prompt" reads a value into $KEY. Offers the existing .env value as
# a default on re-runs (Enter keeps it). Visible input (non-secret).
ask() {
  local key="$1" prompt="$2" current input
  current=$(_existing "$key" || true)
  if [[ -n "$current" ]]; then
    printf '  %s%s%s %s[Enter keeps current: %s]%s ' "$BOLD" "$prompt" "$RESET" "$DIM" "$current" "$RESET"
  else
    printf '  %s%s%s ' "$BOLD" "$prompt" "$RESET"
  fi
  read -r input || true
  [[ -z "$input" && -n "$current" ]] && input="$current"
  printf -v "$key" '%s' "$input"
}

# ask_secret KEY "Prompt" is like ask, but input is hidden and never recalled
# from ENV_FILE (secrets are never written to the local state file).
ask_secret() {
  local key="$1" prompt="$2" input
  printf '  %s%s%s ' "$BOLD" "$prompt" "$RESET"
  read -rs input || true
  printf '\n'
  printf -v "$key" '%s' "$input"
}

# write_env KEY VALUE upserts KEY=VALUE into ENV_FILE (creates it; replaces
# any existing line). Idempotent. Only ever called with non-secret values in
# this script (see ask_secret above).
write_env() {
  local key="$1" value="$2" tmp
  touch "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  tmp=$(mktemp)
  grep -vE "^${key}=" "$ENV_FILE" > "$tmp" || true
  printf '%s=%s\n' "$key" "$value" >> "$tmp"
  mv "$tmp" "$ENV_FILE"
  WRITTEN_ENV+=("$key")
  note "remembered $key for next run in $ENV_FILE"
}

# finish clears, then shows a closing summary of everything configured.
finish() {
  _clear
  printf '\n%s%s  ✓ Setup complete%s\n' "$BOLD" "$GREEN" "$RESET"
  (( ${#WRITTEN_ENV[@]} ))    && note "remembered for next run: ${WRITTEN_ENV[*]}"
  if (( ${#SKIPPED[@]} )); then
    printf '\n'; warn "still to do by hand:"
    for s in "${SKIPPED[@]}"; do note "  - $s"; done
  fi
  printf '\n'
}

# ──────────────────────────────────────────────────────────────────────────
# STAGES: author this section. One stage() per step the human takes.
# ──────────────────────────────────────────────────────────────────────────

TOTAL_STAGES=7

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)

# This wizard's own bookkeeping: non-secret values only (IP, alias, username,
# subdomain), so re-runs can offer them back as defaults. Never used for
# secrets (DuckDNS token, SMTP/OJS credentials) - those are always re-prompted
# and never touch disk here.
mkdir -p "${XDG_CACHE_HOME:-$HOME/.cache}"
ENV_FILE="${XDG_CACHE_HOME:-$HOME/.cache}/tcc-vm-setup.env"

# Where the app's env file lives on the VM. Must match scripts/deploy.sh's
# REMOTE_DIR default ('jserd') and its "$REMOTE_DIR/.env" path.
REMOTE_APP_DIR='$HOME/jserd'
REMOTE_ENV_FILE="$REMOTE_APP_DIR/.env"

banner "One-time VM setup for the JSERD monitor"

# ── Stage 1: OCI ingress rules for 80/443 ──────────────────────────────────
stage "OCI: open ports 80 and 443"
say "Caddy needs inbound HTTP (80, for the ACME challenge) and HTTPS (443)."
say "OCI blocks both by default at the security-list level, in front of any"
say "firewall the VM itself runs."
open_url "https://cloud.oracle.com/networking/vcns"
step "Open your VCN → Security Lists → the list attached to the VM's subnet."
step "Add Ingress Rules: source 0.0.0.0/0, TCP, destination port 80."
step "Add another: source 0.0.0.0/0, TCP, destination port 443."
confirm "Are ingress rules for TCP 80 and TCP 443 in place?" \
  || { warn "Re-run this script once they're in place; deploys will fail without them."; SKIPPED+=("OCI ingress rules for ports 80/443"); }

# ── Stage 2: reserve the public IP ─────────────────────────────────────────
stage "OCI: reserve the VM's public IP"
say "An ephemeral public IP changes on stop/start, breaking links you've"
say "already sent out. Reserving it keeps the address stable across restarts."
open_url "https://cloud.oracle.com/networking/ip-management/public-ips"
step "If the VM's current IP is ephemeral: Compute → Instances → your instance"
step "  → the attached VNIC → the IPv4 address → Edit → change to Reserved Public IP."
step "Copy the resulting public IP address."
VM_IP=""
until [[ -n "$VM_IP" ]]; do
  ask VM_IP "VM public (reserved) IP address:"
  [[ -n "$VM_IP" ]] || warn "An IP address is required."
done
write_env VM_IP "$VM_IP"

# ── Stage 3: SSH host alias on the developer's machine ─────────────────────
stage "Local: SSH host alias"
say "scripts/deploy.sh and the rest of this wizard both reach the VM through"
say "a named SSH alias instead of a raw IP."
ask SSH_ALIAS "SSH host alias to use:"
SSH_ALIAS="${SSH_ALIAS:-tcc-vm}"
ask VM_USER "Remote SSH username (Ubuntu 22.04 Minimal default is 'ubuntu'):"
VM_USER="${VM_USER:-ubuntu}"
ask SSH_KEY_PATH "Path to your SSH private key:"
SSH_KEY_PATH="${SSH_KEY_PATH:-$HOME/.ssh/id_ed25519}"
write_env SSH_ALIAS "$SSH_ALIAS"
write_env VM_USER "$VM_USER"
write_env SSH_KEY_PATH "$SSH_KEY_PATH"

SSH_CONFIG="$HOME/.ssh/config"
mkdir -p "$HOME/.ssh"
touch "$SSH_CONFIG"
chmod 600 "$SSH_CONFIG"
MARKER="# tcc-vm-setup:$SSH_ALIAS"
# Idempotent: drop any block this script previously wrote for this alias,
# then append a fresh one matching the current answers. Re-running never
# leaves duplicate or stale Host blocks.
if grep -qF "$MARKER" "$SSH_CONFIG"; then
  awk -v marker="$MARKER" '
    $0 == marker { skip=1; next }
    skip && /^$/ { skip=0; next }
    skip { next }
    { print }
  ' "$SSH_CONFIG" > "$SSH_CONFIG.tmp"
  mv "$SSH_CONFIG.tmp" "$SSH_CONFIG"
  chmod 600 "$SSH_CONFIG"
fi
{
  printf '\n%s\n' "$MARKER"
  printf 'Host %s\n' "$SSH_ALIAS"
  printf '    HostName %s\n' "$VM_IP"
  printf '    User %s\n' "$VM_USER"
  printf '    IdentityFile %s\n' "$SSH_KEY_PATH"
  printf '    StrictHostKeyChecking accept-new\n'
} >> "$SSH_CONFIG"
note "wrote Host $SSH_ALIAS to $SSH_CONFIG"

step "Testing the connection..."
if ssh -o BatchMode=yes -o ConnectTimeout=8 "$SSH_ALIAS" true 2>/dev/null; then
  note "ssh $SSH_ALIAS works."
else
  warn "ssh $SSH_ALIAS did not succeed. Make sure the key is authorized on the"
  warn "VM and the security list allows SSH, then re-run this script."
  SKIPPED+=("SSH connectivity to $SSH_ALIAS")
fi

# ── Stage 4: Docker Engine + Compose plugin ─────────────────────────────────
stage "VM: install Docker Engine and the Compose plugin"
say "Ubuntu 22.04 Minimal's own repository ships an old Docker release, so we"
say "install from Docker's official apt repository instead."
if ssh -o BatchMode=yes -o ConnectTimeout=8 "$SSH_ALIAS" \
    'command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1' 2>/dev/null; then
  note "Docker Engine and the Compose plugin are already installed on $SSH_ALIAS."
else
  step "Installing over SSH (this can take a minute)..."
  ssh "$SSH_ALIAS" '
    set -e
    sudo apt-get update
    sudo apt-get install -y ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
      | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    sudo usermod -aG docker "$USER"
  '
  note "Installed docker-ce, containerd.io, and the buildx/compose plugins."
  warn "Log out and back in over SSH for the new docker group membership to apply."
fi

# ── Stage 5: DuckDNS hostname and A record ──────────────────────────────────
stage "DuckDNS: hostname and A record"
say "Caddy requests a TLS certificate for this hostname via the HTTP-01"
say "challenge, so it must resolve to the VM's reserved IP before you deploy."
ask DUCKDNS_SUBDOMAIN "DuckDNS subdomain (without .duckdns.org):"
DUCKDNS_SUBDOMAIN="${DUCKDNS_SUBDOMAIN:-jserd}"
open_url "https://www.duckdns.org/"
step "Sign in, create the subdomain '$DUCKDNS_SUBDOMAIN' if it doesn't exist yet,"
step "and copy your account token shown at the top of the page."
ask_secret DUCKDNS_TOKEN "Paste your DuckDNS token (not stored):"
say "Pointing $DUCKDNS_SUBDOMAIN.duckdns.org at $VM_IP..."
duckdns_response=$(curl -fsS "https://www.duckdns.org/update?domains=${DUCKDNS_SUBDOMAIN}&token=${DUCKDNS_TOKEN}&ip=${VM_IP}" 2>/dev/null || echo "request failed")
if [[ "$duckdns_response" == OK* ]]; then
  note "DuckDNS A record updated: $DUCKDNS_SUBDOMAIN.duckdns.org -> $VM_IP"
else
  warn "DuckDNS update did not confirm success. Double-check the subdomain and"
  warn "token on the DuckDNS site, then re-run this stage."
  SKIPPED+=("DuckDNS A record for $DUCKDNS_SUBDOMAIN.duckdns.org")
fi
unset DUCKDNS_TOKEN duckdns_response
write_env DUCKDNS_SUBDOMAIN "$DUCKDNS_SUBDOMAIN"
if [[ "$DUCKDNS_SUBDOMAIN" != "jserd" ]]; then
  warn "deploy/docker-compose.vm.yml defaults CADDY_HOSTNAMES to jserd.duckdns.org."
  warn "Export CADDY_HOSTNAMES=$DUCKDNS_SUBDOMAIN.duckdns.org before running scripts/deploy.sh."
fi

# ── Stage 6: the VM's environment file ──────────────────────────────────────
stage "VM: create the environment file"
say "The app reads its whole configuration from one .env file on the VM."
say "This script never asks you for its secrets (SMTP/OJS credentials) - it"
say "only makes sure the file exists, owned and readable by you alone, so you"
say "can fill it in yourself."
if ssh "$SSH_ALIAS" "test -f $REMOTE_ENV_FILE" 2>/dev/null; then
  note "$REMOTE_ENV_FILE already exists on $SSH_ALIAS; leaving it in place."
else
  step "Creating $REMOTE_ENV_FILE from src/server/.env.example as a starting point..."
  # One SSH session for all three steps. umask 177 makes the file 600 from
  # the moment it's created, so it's never briefly world- or group-readable;
  # the trailing chmod is a belt-and-braces guard on top of that.
  ssh "$SSH_ALIAS" "mkdir -p -m 700 $REMOTE_APP_DIR && umask 177 && cat > $REMOTE_ENV_FILE && chmod 600 $REMOTE_ENV_FILE" \
    < "$ROOT_DIR/src/server/.env.example"
  note "Created $REMOTE_ENV_FILE (permissions 600, owner-only)."
fi
step "Edit it now for production: ssh $SSH_ALIAS -t '\${EDITOR:-nano} $REMOTE_ENV_FILE'"
step "At minimum, review DELIVERY_MODE, OJS_SOURCE_MODE, ALLOW_TEST_DELIVERY,"
step "the SMTP_* and OJS_* credentials, and OJS_CHIEF_EDITOR_GROUP_ID/NAME."
say "This file is never read, shipped, or overwritten by scripts/deploy.sh."
confirm "Have you finished editing $REMOTE_ENV_FILE on the VM?" \
  || { warn "Edit it before your first deploy; the app container won't start without it."; SKIPPED+=("editing $REMOTE_ENV_FILE"); }

# ── Stage 7: the first superuser account ────────────────────────────────────
stage "First superuser: the one-time install URL"
say "PocketBase's data directory is created fresh on the very first boot."
say "On that boot only, the server logs a single-use installation URL that"
say "lets you set the first superuser's password in a browser. This script"
say "never puts a superuser password in $REMOTE_ENV_FILE - that URL exists so"
say "you never have to."
step "After your first scripts/deploy.sh run, find the app container:"
step "  ssh $SSH_ALIAS 'docker ps --filter name=app --format \"{{.Names}}\"'"
step "Then look for the install URL in its logs:"
step "  ssh $SSH_ALIAS 'docker logs <container-name> 2>&1 | grep -i pbinstall'"
step "Open that URL once, in a browser, to create the first superuser."
say "Give every chief editor their own superuser account - never a shared"
say "login - so the record of who verified a submission stays meaningful."
say "Create each additional account over SSH, e.g.:"
step "  ssh $SSH_ALIAS 'docker exec <container-name> ./server superuser upsert <editor-email> <a-strong-password> --dir pb_data'"
warn "That command's password is visible in the VM's shell history; use a"
warn "unique password per editor."
pause "Press Enter once this is clear (it happens after your first deploy, not now)"

finish
note "ENV_FILE=$REMOTE_ENV_FILE is what scripts/deploy.sh needs on the VM."
note "SSH alias: $SSH_ALIAS  ·  VM: $VM_IP  ·  hostname: $DUCKDNS_SUBDOMAIN.duckdns.org"
