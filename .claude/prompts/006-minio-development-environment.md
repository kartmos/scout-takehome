# Goal

Add a reproducible local MinIO development environment with Docker Compose, persistent storage, health checking, and idempotent private bucket creation. This task adds infrastructure only; it does not add Go storage code or upload endpoints.

# User prerequisite: Docker Desktop + WSL

Docker Desktop is already installed on this Windows machine, but its CLI is not currently available inside the `Ubuntu` WSL distribution. Before running this task, the user must:

1. start Docker Desktop and use Linux containers;
2. open **Settings → Resources → WSL Integration**;
3. enable integration for **Ubuntu** and apply/restart;
4. verify from the project’s WSL shell:

```bash
docker version
docker compose version
```

Do not install a second Docker Engine with `apt`; Docker recommends avoiding a Docker Engine/CLI installed directly in WSL when Docker Desktop provides the WSL integration. Reference: <https://docs.docker.com/desktop/features/wsl/>.

At task start, run the two verification commands. If either fails, make no project changes: report the prerequisite as blocked and stop.

# Context manifest

Read only:

- `CLAUDE.md`;
- `.gitignore`;
- the `TODO` item about MinIO and the `Runtime` section of `README.md` (locate them with targeted search; do not read the whole file);
- root `git status --short` and a diff scoped to files allowed below.

Verified context:

- the project currently has no Compose, MinIO, `.env.example`, or object-storage adapter;
- MinIO is external to the production service memory budget;
- `.env` is already ignored;
- source and dataset are stored in the WSL Linux filesystem;
- later task 007 will add the Go S3/MinIO adapter, so do not add Go dependencies now.

Do not read previous prompts, `openapi.yaml`, backend source, or dataset contents. Do not use subagents.

# Scope

Create:

```text
compose.yaml
.env.example
```

Modify `.gitignore` only if a newly introduced local secret/data path is not already covered. Do not create or commit `.env`.

# Requirements

## Compose topology

1. Define a standalone `minio` service and a one-shot `minio-init` service.
2. Use official MinIO images pinned to immutable release tags, never `latest`. Use these verified multi-architecture releases unless pulling them proves unavailable:
   - `quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z`;
   - `quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z`.
3. Run MinIO as `server /data --console-address :9001`.
4. Persist `/data` in a named volume. Do not bind-mount `dataset/images` and do not preload photos.
5. Publish API and console ports on `127.0.0.1` only, with configurable host ports defaulting to `9000` and `9001`.
6. Add a real healthcheck against MinIO’s live/ready health endpoint with bounded interval, timeout, start period, and retries.
7. Make `minio-init` wait for healthy MinIO, configure an `mc` alias, and create the configured bucket with `mc mb --ignore-existing`.
8. The bucket must remain private. Do not add anonymous download/upload policy, versioning, lifecycle, replication, or object locking.
9. `minio-init` must exit successfully and be safe to run repeatedly. Do not hide failures with an unconditional success command or infinite retry loop.
10. Add sensible service names, restart behavior for MinIO, and Compose labels/project naming only when useful; avoid Swarm/Kubernetes features.

## Environment and secrets

11. Put all configurable development values in `.env.example`: host-facing S3 endpoint, S3 access key, S3 secret key, bucket name, secure/TLS flag, API port, and console port.
12. Use explicit `SCOUT_...` names suitable for reuse by task 007. Map the local access/secret values to MinIO root credentials only inside this development Compose environment.
13. Provide non-production example values that satisfy MinIO credential length rules. Clearly label them development-only and instruct copying `.env.example` to ignored `.env`.
14. Use Compose required/default interpolation so missing credentials cannot silently fall back to MinIO’s well-known defaults.
15. Do not place credentials directly in `compose.yaml`, logs, healthcheck commands, or committed files other than clearly marked placeholders in `.env.example`.
16. Do not add TLS in this local-only task; expose `SCOUT_S3_SECURE=false` (or an equally clear value) for task 007.

# Out of scope

- Go configuration, SDKs, storage interfaces, presigned URLs, handlers, seed client, photo uploads, thumbnails, metrics, frontend, or production containers.
- API/backend containers in Compose.
- Modifying README, OpenAPI, dataset, Go modules, or application source.
- Installing Docker automatically, changing Docker Desktop settings, committing, or pushing.

# Acceptance criteria

- `docker compose --env-file .env.example config` succeeds without warnings about missing variables.
- MinIO becomes healthy on localhost; the console is reachable on its configured localhost port.
- `minio-init` exits with code 0 and creates exactly the configured private bucket.
- Running the initializer again succeeds without deleting or replacing the bucket.
- Restarting MinIO preserves the bucket through the named volume.
- No dataset file is copied, mounted, or modified; no secret `.env` is tracked.
- Existing Go source and dependencies remain unchanged.

# Verification

Run once after the files are stable:

```bash
docker compose --env-file .env.example config
docker compose --env-file .env.example pull
docker compose --env-file .env.example up -d
docker compose --env-file .env.example ps
docker compose --env-file .env.example logs --no-color minio-init
docker compose --env-file .env.example run --rm minio-init
docker compose --env-file .env.example restart minio
docker compose --env-file .env.example ps
git diff --check
git status --short
```

Inspect container health and initializer exit status explicitly. Verify the configured bucket exists after restart using the pinned `mc` image without exposing credentials in reported output. Finish with `docker compose --env-file .env.example down` (without `-v`) so data persists but containers stop. Do not use `down -v` unless intentionally testing destructive cleanup.

# Final report

Report only: files changed, pinned images, environment variables, bucket-init/persistence design, verification results, and any check not run. Do not proceed to task 007.
