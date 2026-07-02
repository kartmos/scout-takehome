# Goal

Make the repository confidently buildable and runnable on the two primary Linux
container platforms, `linux/amd64` and `linux/arm64`, so the local Docker Compose
stack works on x86-64 Linux, ARM64 Linux, Intel Macs, and Apple Silicon Macs through
Docker Desktop. Add native architecture verification to the existing two GitHub
Actions workflows, make both application Dockerfiles explicitly multi-platform,
and document the exact support contract and remaining limits.

Do not claim native macOS containers: macOS users run the same Linux containers
inside Docker Desktop. Do not claim support for every Linux architecture.

# Source of truth

The intended support matrix is:

| Host | Container platform | Required result |
| --- | --- | --- |
| x86-64 Linux | `linux/amd64` | build, Compose startup, seed, health, integration tests |
| ARM64 Linux | `linux/arm64` | build, Compose startup, seed, health, integration tests |
| Intel macOS + Docker Desktop | `linux/amd64` | documented local Compose support |
| Apple Silicon macOS + Docker Desktop | `linux/arm64` | documented local Compose support |

Other platforms such as `linux/arm/v7`, `linux/386`, `linux/s390x`,
`linux/riscv64`, Windows containers, and native Darwin binaries are outside this
task. Do not add emulation or a hardcoded `platform: linux/amd64` fallback that
silently hides missing ARM64 support.

# Current working state

- Begin with `git status --short`, record `HEAD`, and inspect the scoped diff.
- The repository already contains the completed greenhouse-background work and
  two workflow files. Preserve that feature and all unrelated user changes.
- Exactly two workflows exist or are being prepared:

  ```text
  .github/workflows/ci.yml
  .github/workflows/integration.yml
  ```

  Enhance those files; do not create a third workflow.
- Current GitHub jobs use `ubuntu-latest`, which proves only x64 behavior.
- The backend uses pure-Go `modernc.org/sqlite` and builds with `CGO_ENABLED=0`.
- The backend Dockerfile already consumes `TARGETOS`/`TARGETARCH`, but its build
  stage is not pinned explicitly to `BUILDPLATFORM` and its architecture defaults
  should be made unambiguous.
- The frontend build output is static and architecture-independent, while its
  Node builder and nginx runtime images are architecture-specific.
- The pinned Go, Node, Debian, nginx-unprivileged, MinIO, and MinIO Client images
  currently expose both `linux/amd64` and `linux/arm64` manifests. The task must
  verify the exact pinned tags rather than assume this remains true.
- `compose.yaml` builds application images locally. `compose.production.yaml`
  consumes externally supplied images and must not pretend that local-only image
  names are a published multi-architecture release.
- Do not commit, push, publish images, log in to a registry, change secrets, or
  rewrite history.

# Context manifest

Read only:

```text
CLAUDE.md
README.md: prerequisites, quick start, production images, and troubleshooting only
backend/Dockerfile
backend/go.mod: Go version and SQLite dependency only
frontend/Dockerfile
frontend/package.json: package manager, Node engine, and scripts only
.nvmrc
compose.yaml
compose.production.yaml
.github/workflows/ci.yml
.github/workflows/integration.yml
.dockerignore
.claude/prompts/ROADMAP.md: current status and final task table only
```

Inspect one directly connected file only if a demonstrated build/workflow blocker
requires it. Do not inspect application feature code, regenerate APIs, query the
dataset, reread old task prompts, or use subagents.

# Scope

Modify only when necessary:

```text
backend/Dockerfile
frontend/Dockerfile
.github/workflows/ci.yml
.github/workflows/integration.yml
README.md
.claude/prompts/ROADMAP.md
```

Do not modify Go/Node dependencies or lockfiles, application source/tests,
OpenAPI, dataset contents, Compose topology, ports, credentials, API behavior,
resource limits, nginx configuration, or the greenhouse background work.

Change `compose.yaml` or `compose.production.yaml` only if static validation proves
that an architecture-neutral correction is strictly required. Never add a fixed
`platform:` key to either Compose file.

# Dockerfile multi-platform contract

## Backend

1. Use BuildKit's automatic platform arguments explicitly:

   ```dockerfile
   FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS builder
   ARG TARGETOS
   ARG TARGETARCH
   ```

   Keep the runtime stage target-native. Do not pin the final Debian stage to the
   build platform.
2. Remove architecture defaults such as `TARGETARCH=amd64`; the selected Buildx
   target must provide the values. A requested ARM64 build must produce an ARM64
   Go binary, not an emulated or mislabeled AMD64 binary.
3. Preserve `CGO_ENABLED=0`, `-trimpath`, stripped binaries, non-root UID/GID,
   health tooling, cache directory ownership, and the existing entrypoint.
4. Do not introduce a C compiler, cross-toolchain, QEMU dependency inside the
   image, or architecture-specific shell branches.

## Frontend

1. Pin only the Node build stage to `$BUILDPLATFORM`:

   ```dockerfile
   FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS builder
   ```

2. Keep the final nginx-unprivileged stage target-native so Buildx selects the
   matching AMD64 or ARM64 runtime manifest.
3. Preserve Corepack/pnpm pinning, frozen-lockfile install, API type generation,
   typecheck, Vite production build, static asset copy, nginx config, and port.
4. Do not add native Node modules, platform-specific frontend artifacts, or a
   second frontend build path.

# Base-image platform validation

Before changing workflows, inspect the exact pinned image manifests with
`docker buildx imagetools inspect` and confirm both `linux/amd64` and
`linux/arm64` exist for:

```text
golang:1.26-bookworm
debian:bookworm-slim
node:24-bookworm-slim
nginxinc/nginx-unprivileged:1.27-alpine
quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z
quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z
```

If any exact tag lacks one of the two required platforms, stop and report
`BLOCKED`. Do not silently replace a pinned image, use `latest`, or force
architecture emulation.

# Workflow 1 — CI source and multi-platform build gate

Keep `.github/workflows/ci.yml` as the fast source-quality and image-build gate.

1. Preserve the existing `push` to `main`, `pull_request`, and minimal
   `permissions: contents: read` behavior.
2. Preserve frontend lint/test/build and backend formatting/vet/race-test/build
   jobs on the standard x64 runner. Do not duplicate every source-level test on
   both architectures; the integration workflow provides native runtime proof.
3. Add one bounded Docker Buildx validation job after source jobs pass:
   - use only official Docker actions;
   - create a Buildx builder;
   - build both application images for
     `linux/amd64,linux/arm64`;
   - do not push, publish, log in, or require registry credentials;
   - use a cache-only/output strategy appropriate for multi-platform validation;
   - use GitHub Actions cache with separate API/web scopes;
   - pass only the existing development/demo API build argument needed by the
     frontend build and never print real secrets;
   - set a proportionate `timeout-minutes`.
4. A build of both target manifests must execute the complete Dockerfile stages,
   not merely lint Dockerfile syntax.
5. Keep workflow YAML valid and pin action major versions consistently with the
   repository's existing policy.

# Workflow 2 — native AMD64 and ARM64 integration matrix

Extend `.github/workflows/integration.yml` so the complete local Compose stack is
tested natively on both architectures.

1. Use an explicit matrix with native GitHub-hosted runners:

   ```yaml
   include:
     - runner: ubuntu-latest
       platform: linux/amd64
     - runner: ubuntu-24.04-arm
       platform: linux/arm64
   ```

   Equivalent names are acceptable only if they are current documented native
   x64 and ARM64 GitHub-hosted Linux runners. Do not use QEMU for the integration
   job and do not add a macOS runner: GitHub-hosted macOS does not provide Docker
   Desktop/Engine suitable for this Compose acceptance.
2. Keep `push`, `pull_request`, `workflow_dispatch`, minimal permissions, and a
   bounded timeout. Add `fail-fast: false` so both architecture results remain
   visible.
3. Before building, assert the native runner architecture (`uname -m`) matches the
   matrix expectation and record `docker info` architecture concisely.
4. On each architecture run the same real acceptance path:
   - `docker compose config`;
   - `docker compose build` without a hardcoded platform override;
   - `docker compose up -d --wait --wait-timeout 120`;
   - frontend, `/healthz`, and `/metrics` smoke checks;
   - first seed run;
   - second seed run proving idempotency;
   - Go integration tests with race detector and the existing environment;
   - service state/log dump on failure;
   - `docker compose down --volumes` under `if: always()`.
5. Confirm the built `scout-api:local` and `scout-web:local` image architectures
   match the current matrix platform using `docker image inspect`. Fail on a
   mismatch.
6. Keep dev-only credentials local to the workflow environment. Do not introduce
   GitHub secrets for the disposable integration stack.
7. Avoid duplicated image tags, project names, caches, or artifacts that could
   collide between matrix jobs.

# macOS support contract

GitHub CI can prove the same Linux images natively on AMD64 and ARM64, which is the
container-level prerequisite for Intel and Apple Silicon Docker Desktop. It cannot
honestly prove every Docker Desktop/macOS release combination.

Update README with a concise `Supported platforms` section:

1. State verified CI platforms:
   - Linux x86-64 / `linux/amd64`;
   - Linux ARM64 / `linux/arm64`.
2. State supported host path:
   - Intel Mac through current Docker Desktop using `linux/amd64`;
   - Apple Silicon Mac through current Docker Desktop using `linux/arm64`.
3. State that containers are Linux containers, not native Darwin binaries.
4. Require a current Docker Desktop with Compose v2 and at least the repository's
   existing documented resource budget.
5. Keep the normal command architecture-neutral:

   ```bash
   docker compose up -d --build --wait
   ```

   Docker should select the native platform automatically; users should not need
   `DOCKER_DEFAULT_PLATFORM=linux/amd64` on Apple Silicon.
6. Add a short diagnostic block:

   ```bash
   uname -m
   docker version
   docker compose version
   docker info --format '{{.Architecture}}'
   docker image inspect scout-api:local --format '{{.Architecture}}'
   docker image inspect scout-web:local --format '{{.Architecture}}'
   ```

7. Explain that `compose.production.yaml` requires externally supplied API/web
   images whose manifest includes the host's target platform. Do not claim that
   this repository currently publishes those images.
8. List unsupported/unverified architectures without promising emulation.

# Local verification

Do not publish images. On the current host:

1. Validate both workflow files as YAML using an already available repository
   tool if present; do not add a dependency only for YAML parsing.
2. Run:

   ```bash
   docker compose config
   docker compose -f compose.production.yaml config
   docker buildx version
   ```

   Supply safe temporary values only where Compose requires environment
   interpolation; never print contents from `.env`.
3. Run complete Buildx builds for both application Dockerfiles and both target
   platforms:

   ```text
   linux/amd64
   linux/arm64
   ```

   Use cache-only output or another non-publishing output. Do not use `--push`.
4. Run the existing native local Compose smoke/integration path only for the
   current host architecture. Cross-platform Buildx success does not authorize an
   emulated full-stack integration run.
5. If native ARM64 execution is unavailable locally, rely on the workflow matrix
   for that runtime proof and report it as pending until Actions completes. Never
   claim an unexecuted ARM64 integration result.
6. Run:

   ```bash
   git diff --check
   git status --short
   ```

# Acceptance criteria

- Backend and frontend Dockerfiles build for both `linux/amd64` and
  `linux/arm64` without architecture hardcoding or runtime emulation.
- Every pinned base/service image exposes both required platform manifests.
- CI executes source gates and non-publishing Buildx builds for both platforms.
- Integration runs the complete stack natively on x64 and ARM64 runners, including
  double seed, health checks, metrics, and Go integration tests.
- Built application image architecture is asserted in each integration job.
- README accurately covers Linux x86-64, Linux ARM64, Intel Mac Docker Desktop,
  Apple Silicon Docker Desktop, production-image requirements, and exclusions.
- Exactly the existing two workflow files remain; no registry login, publish,
  commit, push, dependency, application behavior, API, dataset, or Compose
  topology change occurs.
- No claim of universal architecture support or native macOS containers is made.

# Documentation bookkeeping

After available verification passes, add task `019` to the roadmap. Record:

- explicit BuildKit platform handling in both Dockerfiles;
- Buildx `linux/amd64,linux/arm64` build gate;
- native x64/ARM64 integration matrix;
- Docker Desktop support contract for Intel and Apple Silicon;
- exact checks run locally and any Actions result still pending.

Do not mark remote ARM64 integration as passed before the workflow actually
finishes successfully.

# Final report

Report Dockerfile platform changes, exact base-image manifest results, workflow
matrix and build-gate behavior, README support matrix, local native architecture,
all Buildx/Compose/integration checks run, checks deferred to GitHub Actions, exact
Git status, and confirmation that no images were published. Distinguish clearly
between local evidence and pending remote CI. Do not commit or push. Finish with
`READY FOR REVIEW`, `BLOCKED`, or `CHANGES REQUIRED`.
