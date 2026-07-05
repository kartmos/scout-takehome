# Goal

Add a reproducible, secure production deployment path for Scout on one rented
Linux application server: Caddy terminates HTTPS, GitHub Actions builds and
publishes immutable API/web images to GitHub Container Registry (GHCR), and an
automatic post-CI workflow deploys each successful `main` commit over SSH. Add a concise
deployment section to `README.md` and a separate, exhaustive Russian-language
operator guide at `deploystepbystep.md` for a first-time VPS owner.

This task packages and documents the existing application. Do not redesign
product behavior, authentication, the API contract, database access, thumbnail
logic, frontend behavior, or local development topology.

# Current verified state

- The application already has multi-stage, non-root `backend/Dockerfile` and
  `frontend/Dockerfile` builds for `linux/amd64` and `linux/arm64`.
- `.github/workflows/ci.yml` and `.github/workflows/integration.yml` already own
  source/build/integration gates. Preserve them unless a demonstrated deployment
  blocker requires the smallest possible change.
- `compose.yaml` is the clean-clone local topology with MinIO and seed.
- `compose.production.yaml` is API + web with external object storage, read-only
  SQLite, persistent bounded thumbnail cache, health checks, and resource limits.
- The web image serves React through unprivileged Nginx and proxies `/api/` to the
  `api` service. Caddy therefore needs to proxy only to `web:8080`; it must not
  duplicate application routing or proxy original object bytes.
- The frontend build embeds `VITE_SCOUT_API_KEY`; this value is visible in the
  downloaded JavaScript by design and must equal runtime `SCOUT_API_KEY`.
- Production object storage is external to the application-server CPU/RAM
  budget. Originals must never be served from `dataset/images`.
- The repository currently does not publish application images and does not have
  a Caddy/GHCR/server-deployment workflow.
- Task 019 introduced multi-platform builds. Do not regress either target or
  claim remote CI evidence that was not observed.

# Context manifest

Begin with `git status --short` and a diff limited to the files below. Read only:

```text
CLAUDE.md
README.md: Supported platforms, Quick start, Production images, Production deployment,
           Environment variables, Internal vs public S3 endpoints, Resource budget,
           Security limitations, Cleanup and reset
.env.example
.gitignore
.dockerignore
compose.production.yaml
compose.yaml: api, web, and seed service definitions only
backend/Dockerfile
frontend/Dockerfile
frontend/nginx.conf: server and /api locations only
.github/workflows/ci.yml
.github/workflows/integration.yml
.claude/prompts/README.md
.claude/prompts/ROADMAP.md: task 019/020 status and next-stage bookkeeping only
```

Inspect one directly connected file only when a concrete syntax, build, or
deployment blocker requires it. Do not reread previous prompt files, all source
code, tests, `openapi.yaml`, dataset image bytes, or Git history. Do not use
subagents.

# Context and implementation budget

- Keep this as one deployment/infrastructure task.
- Prefer repository-native shell, Compose, Caddy, and official Docker/GitHub
  actions over a new application dependency.
- Do not add Terraform, Ansible, Kubernetes, a queue, Redis, or a cloud-specific
  SDK.
- Do not execute real DNS changes, rent infrastructure, create GitHub secrets,
  log into the user's registry, contact a real production server, publish images,
  commit, or push.
- Static validation and local non-publishing checks are allowed. Any remote
  action must remain a documented user step.

# Scope

Create or modify only:

```text
.github/workflows/deploy.yml
deploy/Caddyfile
deploy/compose.server.yaml
deploy/.env.production.example
deploy/deploy.sh
deploy/rollback.sh                 # only if a real tested rollback is implemented
README.md
deploystepbystep.md
.gitignore                         # only for exact production-secret/release files
.claude/prompts/ROADMAP.md          # add task 021 bookkeeping only after validation
```

An adjacent deployment-only file may be added only if the design cannot be made
safe and understandable without it; explain why in the final report. Do not
change application source, `openapi.yaml`, datasets, the existing Dockerfiles,
or local Compose unless a verified blocker makes the deployment impossible.

# Pinned deployment design

Implement this topology:

```text
Internet
   |
   | TCP 80/443
   v
Caddy container
   |
   | private Compose network, HTTP
   v
web container (unprivileged Nginx + React)
   |
   | /api, private Compose network
   v
api container (Go)
   |-- read-only /app/predictions.db
   |-- bounded persistent thumbnail-cache volume
   `-- external S3-compatible object storage / external MinIO
```

Only Caddy publishes host ports. `api`, `web`, metrics, health, object-storage
credentials, and Caddy admin must not be exposed publicly. Preserve the existing
application resource limits; explicitly bound Caddy CPU, memory, and PIDs with a
small documented allowance. The application server remains a single small box;
external object storage is not added to this Compose project.

# Requirement 1 — Caddy edge configuration

1. Use a concrete, stable, multi-platform official Caddy Alpine image tag; never
   use `latest`. Verify the exact tag exists for both `linux/amd64` and
   `linux/arm64`, or report the blocked network check without inventing a result.
2. `deploy/Caddyfile` must:
   - obtain the hostname from a validated environment placeholder such as
     `{$SCOUT_DOMAIN}`;
   - use automatic HTTPS and a configurable ACME contact email;
   - redirect HTTP to HTTPS through normal Caddy behavior;
   - disable the public/admin API or keep it inaccessible;
   - proxy the site to `web:8080` only;
   - preserve host and standard forwarded information using safe Caddy defaults;
   - enable sensible `zstd`/`gzip` response encoding;
   - emit structured logs to stdout without logging secrets, authorization
     headers, API keys, signed object-storage query strings, or cookies;
   - avoid duplicating Nginx asset caching, SPA fallback, and `/api` routing;
   - avoid unsafe wildcard CORS, Basic Auth, or a second application auth model.
3. Persist Caddy certificate/state data in named volumes. Do not bind private
   certificate files from the repository.
4. Document that ports 80 and 443 must reach Caddy and that DNS must resolve to
   the server before ACME succeeds. If Cloudflare is mentioned, recommend
   DNS-only during initial certificate issuance and `Full (strict)` if the proxy
   is enabled later; do not make Cloudflare mandatory.

# Requirement 2 — server Compose topology

Create `deploy/compose.server.yaml` as a complete, production-only topology rather
than a fragile override that accidentally retains public API/web ports.

1. Services:
   - `api`: use `${SCOUT_API_IMAGE}:${SCOUT_IMAGE_TAG}`;
   - `web`: use `${SCOUT_WEB_IMAGE}:${SCOUT_IMAGE_TAG}`;
   - `caddy`: pinned official image and Caddy config;
   - optional one-shot `seed` profile may reuse the API image and existing
     `scout-seed` binary if it is the simplest reliable first-ingest path.
2. Preserve the relevant production API settings from
   `compose.production.yaml`: read-only SQLite bind, persistent bounded thumbnail
   cache, read-only root filesystem, required tmpfs, non-root user,
   `no-new-privileges`, health check, restart policy, stop grace period,
   `GOMAXPROCS`, `GOMEMLIMIT`, thumbnail concurrency, memory/CPU/PID limits, and
   external object-storage variables.
3. Preserve equivalent web health/security/resource settings, but expose it only
   to the private Compose network. Do not publish `8090`.
4. Caddy alone publishes `80:80` and `443:443` (TCP; add UDP 443 only if the
   selected Caddy/HTTP3 policy actually uses and documents it). Bound its
   resources and use `restart: unless-stopped`, `no-new-privileges`, and a
   sensible health/startup dependency strategy supported by ordinary Docker
   Compose—not Swarm-only fields.
5. Use named networks/volumes only where they improve clarity. Do not grant
   `privileged`, host networking, the Docker socket, or unnecessary Linux
   capabilities.
6. `SCOUT_DATABASE_PATH` on the host must be an explicit absolute path such as
   `/opt/scout/data/predictions.db`, supplied through the production env file.
7. If a seed profile is included, it mounts `/opt/scout/data/images` read-only,
   uses the private API address, stays bounded, runs only on explicit command,
   and is absent from steady-state resource use. The guide must explain how to
   upload the sample files, run the seed twice safely, verify success, and then
   optionally remove the temporary server-side image copies only after object
   storage is verified.
8. Validate all required Compose interpolation with non-secret placeholders.
   Missing domain, image, tag, API key, CORS origin, S3 endpoint/credentials,
   bucket, secure flag, or region must fail early with a readable message.

# Requirement 3 — production environment example and secret policy

Create `deploy/.env.production.example` containing placeholders and comments for
every required value, at least:

```text
SCOUT_DOMAIN
SCOUT_ACME_EMAIL
SCOUT_API_IMAGE
SCOUT_WEB_IMAGE
SCOUT_IMAGE_TAG
SCOUT_DATABASE_PATH
SCOUT_API_KEY
SCOUT_CORS_ALLOWED_ORIGINS
SCOUT_S3_ENDPOINT
SCOUT_S3_PUBLIC_ENDPOINT          # when different
SCOUT_S3_ACCESS_KEY
SCOUT_S3_SECRET_KEY
SCOUT_S3_BUCKET
SCOUT_S3_SECURE
SCOUT_S3_PUBLIC_SECURE            # when different
SCOUT_S3_REGION
SCOUT_THUMBNAIL_CACHE_MAX_BYTES
```

1. Use obvious placeholders, never realistic credentials.
2. Explain which values are secrets and which are public configuration.
3. The real file is `/opt/scout/.env.production`, mode `0600`, owned by the
   deployment user, never committed, printed, uploaded as a workflow artifact,
   or embedded into shell history.
4. Add exact `.gitignore` entries for the real deployment env/release files if
   needed, without ignoring the example.
5. Make the API-key limitation explicit: `VITE_SCOUT_API_KEY` used while building
   the web image must equal server `SCOUT_API_KEY`, but it is browser-visible and
   must not be described as genuine user authentication. Rotating it requires a
   new web image and a coordinated server env update.
6. No production S3/API secrets should be required in GitHub if they can remain
   only on the server. The workflow needs only build/deployment secrets.

# Requirement 4 — GHCR build and publication workflow

Create `.github/workflows/deploy.yml`. This task explicitly authorizes a third
workflow; preserve the existing CI and integration workflows.

1. Automatically deploy every commit pushed or merged into `main`, but only
   after both existing workflows named `CI` and `Integration` have completed
   successfully for that exact commit SHA. A pull-request workflow run must never
   deploy. Use a reliable cross-workflow gate, preferably:
   - `workflow_run` for the completed `Integration` workflow on `main`;
   - require `github.event.workflow_run.conclusion == 'success'`;
   - checkout and build `github.event.workflow_run.head_sha`, never the moving
     branch tip;
   - query/poll the GitHub Actions API with a bounded timeout to verify the `CI`
     workflow for that same SHA also concluded successfully;
   - fail closed when the matching CI run is absent, pending past the timeout,
     cancelled, skipped unexpectedly, or unsuccessful.
   An equivalently safe design is allowed, but a plain concurrent `push: main`
   deploy that can race ahead of CI/Integration is not.
2. Retain `workflow_dispatch` as an explicit recovery/first-deploy path. It must
   select an exact commit SHA or immutable existing image tag, validate it, and
   must not silently deploy the current moving `main` tip.
3. Add a repository Actions variable `PRODUCTION_DEPLOY_ENABLED`. Automatic
   publication/deployment must be skipped unless its value is exactly `true`.
   This lets the infrastructure workflow be merged before the server, DNS, and
   secrets are ready. Document enabling it only after the first-deploy checklist;
   keep manual dispatch available to perform that first deployment.
4. Use a GitHub Environment named `production` for deployment secrets and branch
   restrictions. For fully automatic deployment it must not require a reviewer;
   document that adding a required reviewer intentionally changes the process to
   approval-gated deployment. Recommend branch protection/rulesets that require
   pull requests plus successful `CI` and `Integration` checks before merge.
5. Grant least privilege:
   - workflow/job `contents: read`;
   - cross-workflow gate `actions: read`;
   - build/publish job `packages: write`;
   - no unnecessary write permissions.
6. Use pinned major versions of official first-party actions already consistent
   with the repository (`actions/checkout`, Docker QEMU/Buildx/login/metadata/
   build-push). Do not use an opaque third-party SSH action; use OpenSSH commands
   available on the GitHub runner.
7. Authenticate to `ghcr.io` with `${{ github.actor }}` and
   `${{ secrets.GITHUB_TOKEN }}` for publication. Never require a personal token
   with package-write permission in Actions.
8. Build and push API and web for both `linux/amd64` and `linux/arm64`. Preserve
   the existing Dockerfile contexts and frontend build argument.
9. Publish immutable, common release tags for both images, including
   `sha-<short-commit>`. A mutable `main` convenience tag may also be published,
   but deployment must use the immutable SHA tag.
10. Use lower-case, valid GHCR image names derived safely from repository owner/
   repository, or make explicit image names documented inputs. API and web names
   must be deterministic and match the server env example.
11. Keep build cache bounded through GitHub Actions cache scopes. Do not publish
   secrets in labels, build output, cache, artifacts, or logs.
12. Pass the production Environment secret `VITE_SCOUT_API_KEY` only where the
    existing web build requires it. Do not pretend BuildKit secret mounts can
    hide a value that Vite intentionally places into the final JavaScript.
13. Make deploy depend on successful publication. Record the exact immutable
    tag being deployed and expose it in the job summary without exposing secrets.
14. Add concurrency control so only one production deployment runs at a time and
    a newer push does not cancel a deployment halfway through. Queued commits may
    deploy sequentially, or safely skip superseded commits only before any
    production mutation; document the chosen behavior.

# Requirement 5 — SSH deployment and host verification

The deploy job must use these GitHub `production` Environment secrets:

```text
DEPLOY_HOST
DEPLOY_PORT
DEPLOY_USER
DEPLOY_SSH_PRIVATE_KEY
DEPLOY_SSH_KNOWN_HOSTS
VITE_SCOUT_API_KEY
```

Use these exact names unless a technical blocker requires a clearly documented
alternative.

1. Write the temporary private key with mode `0600`, create `~/.ssh` with mode
   `0700`, and remove temporary credentials in an `always()` cleanup step.
2. Populate `known_hosts` only from the pinned `DEPLOY_SSH_KNOWN_HOSTS` secret.
   Do not run an unauthenticated `ssh-keyscan` inside the deployment workflow.
   The Russian guide must show how the user obtains and verifies the host key
   through a trusted provider console/first connection before adding it to
   GitHub.
3. Transfer only non-secret deployment files to `/opt/scout` (or a temporary
   directory followed by an atomic move). Never overwrite the server's real
   `.env.production`, database, sample images, Caddy state, or release backup.
4. Invoke `deploy/deploy.sh` with the immutable SHA tag. The script must:
   - use `set -Eeuo pipefail`, restrictive `umask`, quoted variables, and an
     explicit working directory;
   - validate the release tag before using it;
   - validate required files and `docker compose config` without printing the
     rendered secrets;
   - pull before switching containers;
   - retain the previous immutable tag for rollback;
   - update the release/tag state atomically;
   - run `docker compose up -d --remove-orphans` and wait for health with a
     bounded timeout;
   - verify the public HTTPS health endpoint and homepage without logging bodies
     or signed URLs;
   - on failure, restore the previous tag and topology when one exists, report a
     clear failure, and retain enough information for manual diagnosis;
   - never run `docker system prune -a`, delete volumes, delete the database, or
     remove all old images indiscriminately.
5. If a separate `rollback.sh` is added, it must use the recorded previous tag,
   validate it, perform the same bounded health checks, and be documented and
   statically checked. Do not offer a fake rollback that simply retags `latest`.
6. GHCR pull policy on the server:
   - document public package pulls without login;
   - for private packages, create a fine-grained/classic token with only the
     minimum `read:packages` capability allowed by GitHub, run `docker login
     ghcr.io --password-stdin` interactively on the server, and never store that
     token in the repository or workflow logs;
   - explain that package visibility and repository access must permit the
     server account/token to pull both images.

# Requirement 6 — concise README deployment block

Add a clearly named section such as `## Deploy to a VPS with Caddy and GHCR`.
Keep it concise and suitable for a reviewer, while linking to
`deploystepbystep.md` for beginner-level detail.

It must include:

1. The topology and why Caddy does not replace React/Nginx/Go or violate the
   assignment stack.
2. Prerequisites: domain, Linux VPS, Docker Compose v2, external object storage,
   GitHub repository/GHCR, ports 22/80/443, and committed dataset.
3. The exact deployment file locations and one-time server directory.
4. The GitHub Environment/secret names, without values.
5. The automatic `main` flow, its exact CI/Integration SHA gate, the
   `PRODUCTION_DEPLOY_ENABLED` kill switch, and the manual first-deploy/recovery
   procedure.
6. A short seed procedure if the server seed profile is implemented.
7. Health/status/log/rollback commands using placeholders.
8. Explicit security statements: only Caddy public, real env remains on server,
   browser API key limitation, private bucket/presigned originals, and no secrets
   in Git.
9. Link to the Russian step-by-step guide.
10. Do not duplicate the entire operator manual in README or claim that a real
    deployment/domain/certificate was verified by this task.

# Requirement 7 — `deploystepbystep.md` in Russian

Write the entire file in clear Russian for a developer who has never deployed a
VPS. Commands and variable names remain in English. Use short explanations,
numbered stages, copy-pasteable fenced Bash blocks, expected results, and explicit
warnings. Avoid unexplained jargon and avoid provider-specific screenshots.

The guide must be complete enough to follow without guessing and must contain all
of the following sections.

## 7.1 Preparation inventory

Start with a fill-in table:

| Placeholder | Example format | Where to obtain it | Where it is used |
| --- | --- | --- | --- |
| `<SERVER_IP>` | `203.0.113.10` | VPS panel | DNS, SSH, GitHub secret |
| `<SSH_PORT>` | `22` | VPS/sshd config | SSH, firewall, GitHub secret |
| `<DEPLOY_USER>` | `deploy` | created on server | SSH workflow |
| `<DOMAIN>` | `scout.example.com` | registrar/DNS | DNS, Caddy, CORS |
| `<ACME_EMAIL>` | email format | user's email | Caddy certificate account |
| `<GITHUB_OWNER>` | lower-case owner | repository URL | GHCR image names |
| `<GITHUB_REPOSITORY>` | repository name | repository URL | GHCR image names |
| `<S3_ENDPOINT>` | bare host[:port] | object-storage panel | server env |
| `<S3_PUBLIC_ENDPOINT>` | public host[:port] | object-storage/DNS | signed URLs |
| `<S3_BUCKET>` | bucket name | object-storage panel | server env |
| `<S3_REGION>` | region identifier | object-storage panel | server env |

Label every placeholder that the user must replace. Never use the documentation
TEST-NET IP as if it were a real address.

## 7.2 Rent the application server

- Recommend a supported Linux LTS image and explain x86-64 vs ARM64 briefly.
- Give a practical minimum and comfortable server size, accounting separately
  for the OS/Caddy versus the application's documented container limits.
- Require a public IPv4 address; explain when IPv6/AAAA should be omitted.
- State that the provider console/recovery access must be retained.
- Explain why external object storage is not placed on the small application
  server. If a separate MinIO host is used, distinguish its address and resource
  needs; do not silently count it inside the Scout box.

## 7.3 Generate and install an SSH key

Provide commands for macOS/Linux and a brief Windows PowerShell equivalent:

```bash
ssh-keygen -t ed25519 -a 100 -f ~/.ssh/scout_deploy -C "scout-deploy"
cat ~/.ssh/scout_deploy.pub
```

Explain public versus private key, where to paste the `.pub` key in the VPS
panel, permissions, first connection, fingerprint verification, and an optional
local `~/.ssh/config` alias. Never tell the user to send or commit the private
key. Show how to test a second SSH session before changing access settings.

## 7.4 Bootstrap and harden the server safely

Provide exact commands for the selected Linux distribution to:

- update packages;
- create `<DEPLOY_USER>` and its `authorized_keys` with correct modes;
- install Docker Engine from Docker's official repository and Compose v2;
- add the deployment user to the `docker` group, explicitly warning that this is
  root-equivalent access;
- verify `docker version` and `docker compose version` after a new login;
- configure time synchronization if the image does not already provide it;
- configure UFW/firewall for the actual SSH port, 80, and 443 before enabling it;
- create `/opt/scout`, `/opt/scout/data`, and required ownership/modes;
- optionally enable unattended security updates with a clear trade-off.

Do not automate disabling root/password SSH in a way that can lock out the user.
If hardening is documented, require all of these first: key login for the new
user works in a second terminal, sudo works, provider recovery console is known,
and the effective sshd config is validated with `sshd -t`. Show backup and
rollback commands. State clearly that changing the SSH port is optional and not
a meaningful substitute for keys/firewall.

## 7.5 Buy/configure the domain

- Explain that buying a domain and configuring DNS are separate operations.
- Show an `A` record for `<DOMAIN>` or a `scout` subdomain pointing to
  `<SERVER_IP>`.
- Explain TTL, propagation, and verification with `dig +short <DOMAIN>` or
  `nslookup`.
- Add `AAAA` only when the VPS and firewall have verified IPv6; otherwise omit it.
- Explain Cloudflare DNS-only versus proxied mode as optional, with initial
  DNS-only and later `Full (strict)` guidance.
- State that Caddy certificate issuance requires public reachability on 80/443
  and a matching DNS record.

## 7.6 Prepare external object storage

- Follow the repository's current production contract: external S3-compatible
  storage; where the project policy requires MinIO specifically, describe an
  external MinIO instance without adding it to the Scout application Compose.
- Create a private bucket, least-privilege application access key, endpoint,
  public endpoint, region, and TLS settings.
- Explain any required bucket CORS for presigned browser reads without making the
  bucket public.
- Map every obtained value to the corresponding `SCOUT_S3_*` variable.
- Explain that credentials are written only to `/opt/scout/.env.production`.
- Include reachability/TLS/DNS checks and common signature errors caused by
  scheme, hostname, region, proxy rewriting, or clock skew.

## 7.7 Prepare GitHub and GHCR

Explain exact UI paths in stable terms (Repository → Settings → Environments /
Secrets and variables → Actions), while noting labels may vary slightly.

- Create Environment `production`, restrict it to `main`, and explain that a
  required reviewer must remain disabled for zero-click automatic deployment.
  Describe the optional approval-gated alternative separately.
- Create repository Actions variable `PRODUCTION_DEPLOY_ENABLED=false` initially.
- Configure a branch protection rule/ruleset for `main`: require a pull request,
  require successful `CI` and `Integration` checks, block force pushes/deletion,
  and prevent ordinary direct pushes. Explain any administrator-bypass caveat.
- Add each required secret by exact name.
- Show how to copy the deployment private key into
  `DEPLOY_SSH_PRIVATE_KEY` locally without printing it unnecessarily.
- Show how to obtain and independently verify the server host key, then create
  `DEPLOY_SSH_KNOWN_HOSTS`; distinguish this from the user's login public key.
- Set `DEPLOY_HOST`, `DEPLOY_PORT`, `DEPLOY_USER`, and the matching
  `VITE_SCOUT_API_KEY`.
- Explain repository Actions permissions needed for `packages: write` and that
  workflow permissions remain least-privilege.
- Explain GHCR package visibility. For a private package, show safe server-side
  `docker login ghcr.io --username ... --password-stdin` using a read-only package
  token; do not put it into a command argument or file in the repo.
- Explain why production S3 credentials stay on the server rather than GitHub.

## 7.8 Prepare `/opt/scout/.env.production`

Show copying the example, editing with `nano` or `vim`, applying `chmod 600`, and
validating required placeholders without printing secrets. Include a complete
annotated example with placeholders only. Explain these relationships:

- `SCOUT_DOMAIN=<DOMAIN>`;
- `SCOUT_CORS_ALLOWED_ORIGINS=https://<DOMAIN>`;
- GHCR image names exactly match workflow output;
- server `SCOUT_API_KEY` equals GitHub `VITE_SCOUT_API_KEY`;
- database path points to the uploaded read-only file;
- internal/public object-storage endpoints and secure flags are consistent.

Generate API/S3 secrets with a cryptographically secure command and explain how
to paste them without committing them. Do not claim the browser-visible API key
is confidential.

## 7.9 Upload the database and seed source images

Give local-machine `scp`/`rsync` commands with the SSH key, port, user, and exact
paths. Verify file existence, owner, mode, count, and the repository-documented
database hash when applicable. Do not modify `predictions.db` and mount it
read-only. Explain that `dataset/images` is temporary seed input only; production
serves originals from object storage.

## 7.10 First deployment and activation of automation

- Explain why the first merge is safe while
  `PRODUCTION_DEPLOY_ENABLED` is absent or `false`.
- After DNS, server files, environment secrets, GHCR pull access, and object
  storage are ready, set `PRODUCTION_DEPLOY_ENABLED=true` and show the GitHub
  Actions manual workflow-dispatch steps for the first exact SHA.
- Explain build/push/deploy stages and expected GHCR SHA tag.
- Show server-side commands to inspect `docker compose ps`, bounded recent logs,
  image tags, health, Caddy certificate state, and public HTTPS endpoints.
- If the first workflow expects deployment files already present, show the exact
  one-time bootstrap transfer before dispatch. If the workflow transfers them,
  make that explicit.
- Include the seed command, rerun it to prove idempotency, verify the gallery, and
  only then discuss deleting temporary source JPEGs.

## 7.11 Routine deployment and rollback

- Show the normal zero-click flow: merge reviewed code into protected `main`,
  observe `CI` and `Integration` for the same SHA, then observe automatic
  build/publish/deploy and health verification. No manual dispatch or Environment
  approval is required after initial activation.
- Show how setting `PRODUCTION_DEPLOY_ENABLED=false` stops future automatic
  deployments without stopping the currently running application. Explain that
  a workflow already past the gate may need to finish because deployment
  concurrency is intentionally not cancelled midway.
- Show exact rollback invocation to the recorded previous SHA tag, how to verify
  it, and what is/is not rolled back (images/config versus database/object data).
- Explain API-key rotation as a coordinated web rebuild plus server env update.
- Do not use `latest` as the rollback mechanism.

## 7.12 Operations, backups, and updates

Include concise commands and policy for:

- `docker compose ps`, bounded `logs --tail`, health and metrics from inside the
  private network or localhost-safe diagnostics;
- disk/memory checks and thumbnail-cache volume awareness;
- backup of `.env.production` to a secure secret store, database, and Caddy data;
- external object-storage backup/versioning responsibility;
- Docker/OS security updates with a maintenance window;
- certificate renewal being automatic while Caddy data remains persistent;
- safe image cleanup limited to demonstrably unused old images, with no blanket
  destructive prune command;
- recovery after reboot and verification of restart policies.

## 7.13 Troubleshooting matrix

Add symptom → likely cause → exact check → remedy rows for at least:

- SSH timeout / permission denied / host key mismatch;
- workflow cannot pull or push GHCR;
- server cannot pull a private image;
- Compose missing required variable;
- Caddy cannot issue a certificate;
- redirect loop behind Cloudflare;
- Caddy 502/503;
- `/api` 401/403 due to mismatched build/runtime key;
- API unhealthy because database path/permissions are wrong;
- S3 bucket unavailable;
- presigned original URL DNS/TLS/signature/CORS failure;
- wrong image architecture;
- disk full from images/logs/cache;
- rollback has no previous release.

For every diagnostic command, avoid printing `.env.production`, authorization
headers, signed URLs, cookies, or secrets.

## 7.14 Final security checklist

End with checkboxes confirming:

- SSH key login tested and private key stored safely;
- provider recovery console retained;
- firewall exposes only intended ports;
- only Caddy is publicly published by Compose;
- DNS and HTTPS work;
- GitHub Environment secrets/branch restriction and the repository deployment
  enable variable are configured;
- `main` protection requires PR + successful CI/Integration before merge;
- real env is `0600` and uncommitted;
- GHCR permissions are minimal;
- bucket is private and originals use signed URLs;
- database is read-only;
- backups exist and restore steps are understood;
- no secret appears in Git, workflow logs, shell history, or screenshots.

# Requirement 8 — scripts and shell quality

1. All repository shell scripts use POSIX-compatible constructs where practical;
   if Bash features are required, use `#!/usr/bin/env bash` and state Bash as a
   requirement.
2. Run `shellcheck` if available. If unavailable, use `bash -n` and report that
   ShellCheck was not run.
3. Quote expansions, use arrays for commands, avoid `eval`, avoid sourcing
   untrusted files, and never echo secrets.
4. Avoid commands that can destroy unrelated server state: no blanket `rm -rf`,
   volume deletion, `docker system prune -a`, firewall reset, or unvalidated
   `sed` edits to sshd configuration.
5. Scripts must support the documented `/opt/scout` layout and fail with useful
   messages when prerequisites are missing.

# Out of scope

- Renting a VPS or buying/configuring a real domain on the user's behalf.
- Creating/changing real DNS records, certificates, GitHub Environments, secrets,
  package visibility, personal access tokens, or production buckets.
- Running a real production deployment, registry push, SSH connection, commit,
  or Git push during this task.
- Hosting MinIO on the constrained application server unless the source-of-truth
  documentation explicitly changes; keep production object storage external.
- Changing application auth, adding OAuth/users, hiding the intentionally
  browser-visible demo key, proxying original bytes, or changing CORS behavior in
  application code.
- Terraform, Ansible, Kubernetes, Swarm, Watchtower, auto-updaters, zero-downtime
  multi-replica orchestration, or multi-server HA.
- Modifying `openapi.yaml`, API/frontend behavior, tests unrelated to deployment,
  dataset contents, or local developer Compose.

# Verification

Perform locally without publishing or contacting a real server.

## Static checks

1. Parse `.github/workflows/deploy.yml` as YAML with an already available parser;
   do not add a dependency solely for this.
2. Run `bash -n` on every new Bash script and `shellcheck` when installed.
3. Run `caddy validate` using the pinned Caddy container with safe placeholder
   environment values if Docker is available; do not obtain a certificate or
   bind public ports.
4. Render `deploy/compose.server.yaml` with a temporary placeholder env file.
   Inspect the rendered config without printing it in full and prove:
   - only Caddy publishes ports;
   - API/web remain private;
   - no MinIO exists in the server topology;
   - image tags are immutable placeholders;
   - required mounts, volumes, health checks, limits, security options, and
     restart policies are present;
   - missing required values fail.
5. If registry network access is available, inspect the pinned Caddy manifest for
   amd64/arm64. Do not push or log into GHCR.
6. Mechanically validate every repository path, workflow secret name, image name,
   env variable, service name, port, and command shared by workflow/scripts/
   Compose/README/guide.

## Existing project regression gate

Because application source must not change, do not rerun the full backend and
frontend suites unless the scoped diff unexpectedly touches application build or
runtime behavior. At minimum run:

```bash
docker compose config
docker compose -f compose.production.yaml config   # safe placeholder values
git diff --check
git status --short
```

If Docker or network access is unavailable, finish all static file validation and
report exact blocked commands. Never fabricate Caddy, GHCR, DNS, SSH, certificate,
or production-server results.

# Acceptance criteria

- Each enabled `main` commit automatically deploys only after both existing CI
  and Integration workflows succeed for the exact same SHA. The workflow builds
  and publishes both application images for amd64/arm64 under the same immutable
  SHA tag before deployment; PR runs and racing/pending/failed checks fail closed.
- `PRODUCTION_DEPLOY_ENABLED` defaults to disabled for safe infrastructure setup;
  `workflow_dispatch` remains an exact-SHA first-deploy/recovery path.
- SSH uses a pinned known-hosts secret, a temporary protected private key, and no
  opaque third-party action.
- Server Compose exposes only Caddy on 80/443; web/API stay private, SQLite stays
  read-only, thumbnail cache stays bounded/persistent, external object storage
  remains external, and all services are resource/security bounded.
- Caddy provides automatic HTTPS and proxies only to the existing web service.
- Real production secrets remain in `/opt/scout/.env.production` and are neither
  committed nor printed. GHCR uses least privilege.
- Deployments use immutable tags, bounded health checks, serialized production
  execution, and a real previous-tag rollback path.
- README has a concise accurate VPS/Caddy/GHCR section linking to a comprehensive
  Russian `deploystepbystep.md`.
- The Russian guide tells a novice exactly what to rent/buy, which values to copy
  from which control panel, where to paste them, how to configure SSH/Docker/
  firewall/DNS/object storage/GitHub/Caddy, how to seed/verify/deploy/rollback,
  and how to avoid lockout or secret leakage.
- No real infrastructure mutation, image publication, secret creation, commit,
  push, application behavior change, or unsupported success claim occurs.

# Roadmap bookkeeping

After locally available validation succeeds, add task `021` to
`.claude/prompts/ROADMAP.md` as `READY FOR REVIEW`, not `DONE`. Record exactly
which static/local checks passed and explicitly mark real GHCR publication, DNS,
SSH, ACME, seed, and production smoke as pending user infrastructure.

# Final report

Report:

- exact files created/changed;
- Caddy image tag and verified platforms;
- server topology, published ports, limits, volumes, and secret boundaries;
- GHCR image names/tags, workflow trigger, permissions, cache, and build targets;
- GitHub Environment secret names;
- SSH known-host/private-key handling;
- deployment and rollback behavior;
- README and Russian guide sections;
- every static/Compose/Caddy/shell check and outcome;
- checks blocked by missing Docker/network/real infrastructure;
- exact `git status --short` and confirmation that nothing was staged, committed,
  pushed, published, rented, or remotely changed.

Finish with exactly one of:

```text
READY FOR REVIEW
CHANGES REQUIRED
BLOCKED
```
