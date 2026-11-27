# Deployment

This page explains how to deploy the JSERD monitor to the production VM, how
to configure each stage, how to roll back, and how to get a backup. The
scripts live in `scripts/`. The Docker Compose file and the reverse proxy
configuration live in `deploy/`.

## Stage

The deployment runs the **validation stage**. In this stage, the system
collects live JSERD data and chief editors curate it. The system sends no
e-mail. Every message goes to the intercepting transport.

The **delivery stage** sends real e-mail to the assigned editors, the
reviewers, and the chief editors. Curation works the same in both stages.
See `CONTEXT.md` for the full definition of both stages, and
[The environment file of each stage](#the-environment-file-of-each-stage)
for the values that select a stage.

## First-time setup

Before you can deploy, you must set up the VM once. Run the guided wizard:

```sh
scripts/setup-vm.sh
```

The wizard walks you through the manual steps (network rules, a reserved IP,
the DuckDNS hostname) and does the scriptable steps for you (Docker install,
the SSH alias, the VM's environment file). Run this wizard first. Do not run
`scripts/deploy.sh` before it finishes.

The wizard suggests `tcc-vm` as the SSH alias, but `scripts/deploy.sh` and
`scripts/rollback.sh` default to the alias `jserd-vm`. Give the wizard the
alias `jserd-vm`, or export `DEPLOY_HOST` to match the alias you chose:

```sh
export DEPLOY_HOST=tcc-vm   # only if you kept the wizard's suggested alias
```

## The environment file of each stage

The wizard copies `src/server/.env.example` to `~/jserd/.env` on the VM. The
example file runs on local fixtures, so you must edit it before the first
deploy. The configuration reference in the README describes the variables.

If a value is not valid, the server stops at startup with
`invalid configuration: <reason>`. The container does not become healthy, and
the deploy rolls back to the previous image, if one exists.

### Values for both stages

| Variable | Value |
| --- | --- |
| `OJS_SOURCE_MODE` | `live` |
| `OJS_BASE_URL` | The OJS installation root, commonly ending in `/index.php`. |
| `OJS_JOURNAL_CONTEXT` | `jserd` |
| `OJS_USERNAME`, `OJS_PASSWORD` | The collection account in OJS. |
| `OJS_CHIEF_EDITOR_GROUP_ID` or `OJS_CHIEF_EDITOR_GROUP_NAME` | The OJS group of the chief editors. |
| `OJS_ARTIFACT_ROOT` | `pb_data/collector_runs` |
| `SMTP_FROM_ADDRESS` | The sender address. Both stages require a valid address. |

Set `OJS_ARTIFACT_ROOT` to `pb_data/collector_runs`. The default path is
outside the `pb_data` volume. A deploy recreates the container, so the
downloaded artifacts at the default path are lost, and the backup does not
include them.

### Validation stage

| Variable | Value |
| --- | --- |
| `ALLOW_TEST_DELIVERY` | `true` |
| `DELIVERY_MODE` | `test` |

The intercepting transport records each message as sent and writes it to the
log of the app container. In the log, pseudonyms replace the addresses, the
names, and the data of the submission. To read the intercepted messages, run:

```sh
ssh jserd-vm
cd jserd
APP_IMAGE=tcc-jserd-monitor:$(cat .deployed_tag) ENV_FILE=.env \
  docker compose -p jserd -f docker-compose.vm.yml logs app
```

If `SMTP_HOST` is set, the password reset e-mail of the dashboard goes
through that SMTP server in both stages. `DELIVERY_MODE` controls only the
notifications and the editorial summary.

### Delivery stage

| Variable | Value |
| --- | --- |
| `DELIVERY_MODE` | `smtp` |
| `ALLOW_TEST_DELIVERY` | `false`. The server then rejects `DELIVERY_MODE=test`. |
| `SMTP_HOST`, `SMTP_PORT` | The SMTP server. |
| `SMTP_TLS` | `true`. The server requires TLS for every SMTP host that is not local. |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | The SMTP account. Set both or neither. |
| `SMTP_REPLY_TO` | Optional. The reply-to address of the notifications to editors. |
| `APP_URL` | The public URL of the dashboard, for example `https://jserd.duckdns.org`. It goes in the password reset link. |

`SMTP_TLS=true` opens an implicit TLS connection, which SMTP servers usually
accept on port 465. The server does not use STARTTLS on a remote host.

To move from the validation stage to the delivery stage:

1. On your local machine, run `scripts/test-smtp-integration.sh`. The script
   starts a local Mailpit container and tests the SMTP transport against it.
   It does not test your SMTP server.
2. On the VM, edit `~/jserd/.env` with the values of the delivery stage.
3. Recreate the app container, as
   [Changing an environment variable without a full deploy](#changing-an-environment-variable-without-a-full-deploy)
   explains.
4. Read the log of the app container. Make sure that it has no
   `invalid configuration` line.

The messages that the intercepting transport recorded stay recorded as sent.
The delivery stage does not send them again. This includes the welcome
e-mail of each recipient.

The Email worker does not retry a failed attempt. The Activity section of the
dashboard shows the delivery status of each notification.

To go back to the validation stage, set `DELIVERY_MODE=test` and
`ALLOW_TEST_DELIVERY=true`, then recreate the app container.

## The deploy command

```sh
scripts/deploy.sh
```

This command:

1. Builds a Docker image from the current commit.
2. Ships the image to the VM over SSH (there is no image registry).
3. Ships `deploy/docker-compose.vm.yml` to the VM. It ships
   `deploy/Caddyfile.example` only when the VM has no Caddyfile yet.
4. On the VM: stops the running app container, backs up the database volume,
   then starts the new image.
5. Waits for the new container to report healthy.
6. Rolls back to the previous image automatically if the new container does
   not become healthy in time, provided a previous image was recorded. On the
   VM's first-ever deploy, no previous image exists yet: the command exits
   with an error and leaves the container in its unhealthy state instead of
   rolling back.

### Preconditions

The command refuses to run when any of these are true:

- The local working tree has uncommitted changes.
- The current branch is not `master`. The command does not check that
  `master` is pushed to `origin/master`: `master` is never force-pushed, so a
  deployed commit stays reachable from the remote history once it is pushed.
- Docker is not installed, or the Docker daemon is not running, on the local
  machine.
- The SSH alias for the VM does not resolve or is not reachable.
- `.env` does not exist yet on the VM (create it once with
  `scripts/setup-vm.sh`).

### Configuration

The command reads these environment variables. Each one has a default:

| Variable | Default | Purpose |
| --- | --- | --- |
| `DEPLOY_HOST` | `jserd-vm` | SSH alias for the VM. |
| `REMOTE_DIR` | `jserd` | App directory on the VM, relative to the VM user's home directory. |
| `IMAGE_NAME` | `tcc-jserd-monitor` | Docker image name. |
| `COMPOSE_PROJECT` | `jserd` | Docker Compose project name on the VM. |
| `KEEP` | `5` | Number of recent images and backup archives to keep. |
| `HEALTH_TIMEOUT_SECONDS` | `180` | Seconds to wait for the new container to become healthy. |

## The backup

Every deploy takes a backup before it starts the new version. The backup
step:

1. Stops the running app container, so the backup captures a consistent
   database.
2. Archives the `pb_data` Docker volume into a `.tar.gz` file.
3. Keeps only the `KEEP` most recent archives (5 by default) and deletes
   older ones.

Archives live on the VM, at `$REMOTE_DIR/archives` (`~/jserd/archives` with
the default `REMOTE_DIR`). Each archive file name has this form:

```
<UTC timestamp>-<git short SHA>.tar.gz
```

To list or download an archive from your local machine, run:

```sh
scripts/fetch-backup.sh          # list archives on the VM, newest first
scripts/fetch-backup.sh <index>  # download the Nth listed archive (1 = newest)
scripts/fetch-backup.sh <name>   # download an archive by its exact file name
```

Downloads land in the current directory. Keep a copy of important archives
off the VM: the archives live on the same volume as the database, so losing
that volume also loses the archives.

## The rollback command

```sh
scripts/rollback.sh          # roll back to the second-most-recent image
scripts/rollback.sh <tag>    # roll back to a specific retained image tag
```

This command restarts the app on the VM with a previously deployed image. It
does not build or push a new image, so it works even when your local machine
cannot build. An image tag is the git short SHA of the commit it was built
from.

The command waits for the target container to become healthy, using the same
`HEALTH_TIMEOUT_SECONDS` default as `scripts/deploy.sh`. It reads the same
`DEPLOY_HOST`, `REMOTE_DIR`, `IMAGE_NAME`, and `COMPOSE_PROJECT` variables.

## The reverse proxy configuration

The live Caddyfile is **hand-managed on the VM** at `~/jserd/Caddyfile`. It is
the source of truth. `scripts/deploy.sh` does not overwrite it; it places
`deploy/Caddyfile.example` only when the VM has no Caddyfile at all, which
happens once, on a fresh VM.

This is deliberate. The VM's Caddy owns ports 80 and 443, so it is the only
thing that can serve any other site on that host. Keeping the live file
editable lets a second site be added without changing this repo. The trade-off
is that `deploy/Caddyfile.example` can drift from what is actually running:
when you change one, change the other by hand.

To see what is really serving traffic:

```sh
ssh jserd-vm "cat jserd/Caddyfile"
```

After editing it on the VM, reload the proxy without touching the app:

```sh
ssh jserd-vm 'cd jserd && APP_IMAGE=tcc-jserd-monitor:$(cat .deployed_tag) ENV_FILE=.env docker compose -p jserd -f docker-compose.vm.yml restart caddy'
```

## The hostname

Caddy serves the app under one or more hostnames, set by the
`CADDY_HOSTNAMES` environment variable in `deploy/docker-compose.vm.yml`.
The default is `jserd.duckdns.org`.

To add a second hostname, set `CADDY_HOSTNAMES` as a system-wide environment
variable on the VM, with a space-separated list of hostnames.
`scripts/deploy.sh` does not forward local environment variables to the VM,
so a value must exist on the VM itself. Write it to `/etc/environment`: SSH
reads that file through PAM for every session, including the non-interactive
one `scripts/deploy.sh` uses, so the value applies with no other setup.

```sh
ssh jserd-vm "echo 'CADDY_HOSTNAMES=jserd.duckdns.org jserd.ufes.br' | sudo tee -a /etc/environment"
scripts/deploy.sh
```

Each hostname must resolve to the VM's IP address before you deploy. Caddy
requests a TLS certificate for each hostname through the HTTP-01 challenge,
and renews it automatically.

## Changing an environment variable without a full deploy

To change a value in the VM's `.env` (for example, `SMTP_HOST`) without
building and shipping a new image, edit `~/jserd/.env` on the VM directly,
then recreate the app container:

```sh
ssh jserd-vm
cd jserd
APP_IMAGE=tcc-jserd-monitor:$(cat .deployed_tag) ENV_FILE=.env COMPOSE_PROJECT_NAME=jserd \
  docker compose -f docker-compose.vm.yml up -d app
```

Do not use `docker compose restart` for this. `restart` only restarts the
process inside the existing container; it does not read `.env` again. The
container keeps the environment values it had at creation, so an edited
`.env` file has no effect until the container is recreated. `up -d` compares
the desired state against `.env` and the compose file, and recreates the
container when they differ.
