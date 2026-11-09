# Automação do Monitoramento para Curadoria no Open Journal Systems: Um Estudo de Caso no JSERD

## Server setup

### Prerequisites

- `Go 1.26.4`

### Environment file

The server requires `src/server/.env` to exist, but it does not parse the file
itself. For a local process, copy the example, restrict access to the current
user, and export its values before starting the server:

```sh
cd src/server
cp .env.example .env
chmod 600 .env

set -a
source ./.env
set +a

go run . serve
```

The default configuration uses local fixtures and the test e-mail transport.
The report exports in `data/examples/` are local, ignored files and are not
included in a fresh checkout. Fixture mode needs these files; tests use
synthetic inputs. Keep real editorial exports out of version control.
It does not contact an OJS or SMTP server. After startup, access:

- API: <http://127.0.0.1:8090/api/>
- PocketBase Admin: <http://127.0.0.1:8090/_/>

On the first run, use the `pbinstall` URL printed by the server to create the
PocketBase superuser.

### Configuration reference

| Variable | Default/example | Purpose |
| --- | --- | --- |
| `ALLOW_TEST_DELIVERY` | `true` | Permits the intercepting e-mail transport. `false` rejects `DELIVERY_MODE=test`. |
| `OJS_SOURCE_MODE` | `fixtures` | Collection source: `fixtures` or `live`. |
| `OJS_BASE_URL` | `http://127.0.0.1` | OJS installation root. Live mode requires an HTTP(S) URL, commonly ending in `/index.php`. |
| `OJS_JOURNAL_CONTEXT` | `jserd` | Journal path/context used below the OJS installation root. |
| `OJS_USERNAME` | empty | OJS account used for form login in live mode. |
| `OJS_PASSWORD` | empty | OJS account password used in live mode. |
| `OJS_REQUEST_TIMEOUT_SECONDS` | `30` | Positive timeout for each OJS HTTP request. |
| `OJS_FIXTURES_DIR` | `../../data/examples` | Directory containing the three fixture CSV files. Required in fixture mode. |
| `OJS_ARTIFACT_ROOT` | `pb_data/collector_runs` | Private directory where downloaded CSV and JSON artifacts are retained. |
| `DELIVERY_MODE` | `test` | E-mail transport: `test` intercepts messages; `smtp` sends them. |
| `SMTP_HOST` | empty | SMTP host. Required only in SMTP mode. |
| `SMTP_PORT` | `0` | SMTP port. Must be between 1 and 65535 in SMTP mode. |
| `SMTP_USERNAME` | empty | Optional SMTP username. Must be configured together with `SMTP_PASSWORD`. |
| `SMTP_PASSWORD` | empty | Optional SMTP password. Must be configured together with `SMTP_USERNAME`. |
| `SMTP_TLS` | `false` | Enables SMTP TLS. Required for every non-loopback SMTP host. |
| `SMTP_FROM_ADDRESS` | `JSERD Development <noreply@example.test>` | Required and validated sender address in both delivery modes. |
| `SMTP_REPLY_TO` | empty | Optional reply-to address used for editor notifications. |
| `OJS_COLLECTION_CRON` | `0 * * * *` | Five-field cron expression for OJS collection. |
| `EMAIL_WORKER_CRON` | `* * * * *` | Five-field cron expression for processing pending individual e-mails. |
| `SUMMARY_CRON` | `0 9 * * 1` | Five-field cron expression for the editorial summary e-mail schedule. |
| `EDITORIAL_TIMEZONE` | `America/Sao_Paulo` | IANA timezone used by scheduled jobs. |
| `EDITORIAL_RN11_FALLBACK_DAYS` | `7` | Positive fallback deadline, in days, for RN11 when OJS has no explicit deadline. |
| `OJS_CHIEF_EDITOR_GROUP_ID` | empty | Optional exact OJS group ID used to resolve chief editors. |
| `OJS_CHIEF_EDITOR_GROUP_NAME` | empty | Optional exact OJS group name used to resolve chief editors. |

## Pre-commit checks

A [Lefthook](https://github.com/evilmartians/lefthook) pre-commit hook, defined in
[`lefthook.yml`](lefthook.yml), runs on every commit that touches server or
client files:

- Server: `golangci-lint` fails the commit if a function's cyclomatic
  complexity exceeds 4 (see `src/server/.golangci.yml`). `go test ./... -cover`
  runs and prints coverage per package; it fails the commit only if a test
  fails, not because of the coverage number itself.
- Client: `biome lint` fails the commit if a function's cognitive complexity
  exceeds 15 (see `src/client/biome.json`). `vitest run --coverage` runs and
  prints coverage per file; it fails the commit only if a test fails, not
  because of the coverage number itself.

### Install

```sh
go install github.com/evilmartians/lefthook@v1.11.13
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
lefthook install
```

## Deployment

Deployment to the production VM is documented separately, in
[`docs/deployment.md`](docs/deployment.md). It covers the one-command deploy,
rollback, backups, and first-time VM setup.
