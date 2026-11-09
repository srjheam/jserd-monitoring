#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
COMPOSE_FILE="$ROOT_DIR/docker-compose.smtp-test.yml"
PROJECT_NAME="tcc-smtp-test-$$"
MAILPIT_SMTP_PORT=${MAILPIT_SMTP_PORT:-1025}
MAILPIT_HTTP_PORT=${MAILPIT_HTTP_PORT:-8025}
MAILPIT_API_URL="http://127.0.0.1:${MAILPIT_HTTP_PORT}"

cleanup() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d

ready=false
attempt=0
while [ "$attempt" -lt 30 ]; do
  if curl --fail --silent "$MAILPIT_API_URL/api/v1/info" >/dev/null; then
    ready=true
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done

if [ "$ready" != true ]; then
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" logs mailpit
  echo "Mailpit did not become ready" >&2
  exit 1
fi

cd "$ROOT_DIR/src/server"
SMTP_INTEGRATION=1 \
SMTP_HOST=127.0.0.1 \
SMTP_PORT="$MAILPIT_SMTP_PORT" \
MAILPIT_API_URL="$MAILPIT_API_URL" \
go test ./internal/email -run '^TestSMTPDockerIntegration$' -count=1
