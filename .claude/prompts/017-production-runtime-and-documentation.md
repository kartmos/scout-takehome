# Goal

Package the completed Scout MVP for reproducible clean-clone operation: multi-stage non-root API and web images, an Nginx same-origin reverse proxy, bounded production Compose, local MinIO/seed workflow, persistent thumbnail cache, health checks, and accurate README documentation. Resolve the container-only internal/public S3 endpoint split so presigned URLs work from both the seed container and a host browser. Do not implement new product features or perform final acceptance/history rewriting.

# Prerequisite

Run only after backend through 012.3 and frontend through 015.1 are complete. Start with `git status --short` and a scoped diff. Required baseline:

- backend full gate and independent review are clean;
- frontend has 83 passing tests, strict typecheck, zero-warning lint, production build, and closed review findings;
- Docker Desktop with Ubuntu WSL integration is available;
- `dataset/` contains 50 JPEGs and `predictions.db`;
- task 018 owns final clean-clone/resource acceptance and Git-history cleanup.

If Docker is unavailable or the frontend/backend gates do not compile, stop and report the prerequisite instead of rewriting application architecture.

# Context manifest

Read only:

```text
CLAUDE.md
README.md
.gitignore
.env.example
compose.yaml
openapi.yaml: servers/security summary only
backend/go.mod
backend/cmd/api/main.go
backend/cmd/seed/main.go
backend/internal/config/config.go
backend/internal/config/config_test.go
backend/internal/objectstorage/minio.go
backend/internal/objectstorage/storage.go
backend/internal/objectstorage/storage_test.go
frontend/package.json
frontend/pnpm-lock.yaml
frontend/vite.config.ts
frontend/src/shared/config/env.ts
frontend/src/test/config.test.ts
```

Inspect adjacent files only for a demonstrated compile/config blocker. Do not read previous prompts, dataset image bytes, unrelated tests, or Git history. Do not use subagents.

# Scope and pinned runtime policy

Create or modify only what production packaging and its documented configuration require, preferably:

```text
backend/Dockerfile
frontend/Dockerfile
frontend/nginx.conf
.dockerignore
compose.yaml
compose.production.yaml
.env.example
.gitignore
README.md
backend/internal/config/config.go
backend/internal/config/config_test.go
backend/internal/objectstorage/minio.go
backend/internal/objectstorage/storage.go
backend/internal/objectstorage/storage_test.go
frontend/src/shared/config/env.ts
frontend/src/test/config.test.ts
```

Use concrete, available image tags—never `latest`. Build with Go 1.26.x, Linux Node 24.x and pnpm 10.34.4 matching the project. Prefer a pinned unprivileged Nginx Alpine image. Record chosen tags in the final report. Add no Go, npm, proxy, orchestration, or application dependency.

# Phase 1 — real container-safe object-storage URLs

One endpoint is insufficient in local containers: API I/O must reach `minio:9000`, while presigned URLs must contain a hostname reachable by the host browser and seed container.

1. Extend S3 configuration with optional `SCOUT_S3_PUBLIC_ENDPOINT` and `SCOUT_S3_PUBLIC_SECURE`. When absent, both default exactly to the existing internal endpoint/secure values, preserving native-development and production behavior.
2. Validate the public endpoint with the same strict bare `host[:port]` rules. Never allow scheme, path, query, fragment, credentials, whitespace, or invalid port. Never include access/secret keys in errors.
3. Keep one internal MinIO client for bucket checks and original reads. When public settings differ, construct a second narrowly owned client only for presigned PUT/GET generation; reuse one client when settings are equal. Region/credentials/bucket/TTL remain identical.
4. Presigning must not require a public-endpoint network round trip. Supply the configured region so URL generation remains deterministic/offline. Internal storage operations must never accidentally use the public hostname.
5. Add focused config/storage regression tests proving defaults, strict public validation, internal operations staying internal, and returned upload/download URLs using the public host/scheme without leaking credentials.
6. Local Compose should use internal `minio:9000` and public `minio.localhost:<published-port>`. Give the MinIO service the matching network alias so the seed container resolves it; verify the host browser/curl resolves `*.localhost` to loopback. If the host environment does not support this reserved localhost subdomain, document one explicit fallback rather than silently emitting an unreachable URL.

# Phase 2 — production images and proxy

7. Add a root `.dockerignore` that excludes Git metadata, local env/secrets, node_modules/dist/caches, test artifacts, dataset, local binaries, thumbnail cache, editor files, and recovery archives while retaining all sources, lockfiles, `openapi.yaml`, and configuration needed by both builds.
8. API Dockerfile:
   - multi-stage build from repository-root context;
   - download modules from `go.mod/go.sum` before source copy for cacheability;
   - build static `scout-api` and `scout-seed` binaries with reproducible flags;
   - runtime contains CA certificates and only required binaries/directories;
   - run as a fixed non-root UID/GID;
   - pre-create writable thumbnail-cache path with correct ownership;
   - expose only API port and use exec-form entrypoint/signals.
9. Web Dockerfile:
   - multi-stage Node 24/pnpm 10 build using frozen lockfile;
   - generate OpenAPI types, typecheck, and build;
   - build with `VITE_SCOUT_API_BASE_URL=/api` and the explicitly supplied demo API key;
   - copy only `dist/` and Nginx config into a pinned unprivileged runtime.
10. Extend frontend config to accept a safe root-relative base such as `/api` in addition to existing absolute HTTP(S) URLs. Reject protocol-relative `//host`, traversal, backslashes, credentials, query, and fragment. Normalize trailing slash. Keep absolute validation unchanged and add focused tests. Thumbnail/Data API builders must work unchanged with the relative base.
11. Nginx:
   - serve hashed assets with immutable long caching and `index.html` with revalidation/no-cache;
   - SPA fallback to `index.html`;
   - proxy `/api/` to API with the prefix stripped while preserving query, status, streaming/range/cache/ETag headers;
   - set bounded proxy/connect/read timeouts and request body size;
   - forward standard host/forwarded headers, never synthesize/log API keys;
   - use an access log format that omits query strings and headers/secrets;
   - add pragmatic security headers/CSP compatible with same-origin API and HTTP(S) presigned images;
   - run unprivileged on a non-privileged port.
12. Do not proxy original object bytes through Scout merely to hide the S3 hostname. Originals remain direct presigned object-storage reads; thumbnails remain CDN-friendly through the public API route.

# Phase 3 — Compose topologies and resource bounds

13. Turn `compose.yaml` into the documented clean-clone local stack: MinIO, idempotent bucket init, API, web, and a one-shot seed service/profile. Preserve the named MinIO volume.
14. API local service:
   - database bind-mounted read-only;
   - named persistent thumbnail-cache volume;
   - internal/public S3 endpoints wired correctly;
   - health check on `/healthz`;
   - waits for successful bucket init;
   - `GOMAXPROCS=1`, bounded `GOMEMLIMIT`, thumbnail concurrency `1`;
   - read-only root filesystem, required tmpfs, non-root user, `no-new-privileges`, bounded PIDs;
   - API may bind a loopback host port for smoke/debug, while normal users open the web URL.
15. Web local service waits for API health, exposes one documented loopback host port, has its own health check, read-only filesystem/tmpfs requirements, non-root user, bounded PIDs, CPU, and memory.
16. Seed service reuses the backend image and `scout-seed` binary, mounts only `dataset/images` read-only, uses `http://api:8080`, bounded concurrency default `2`, and never prints the API key or signed URLs. It must successfully PUT to the public presigned hostname from inside the Compose network.
17. Add `compose.production.yaml` for API+web only, with external S3 values required via environment and no MinIO/seed service. Keep SQLite read-only and thumbnail cache persistent. API+web combined limits must fit approximately `1 CPU / 512 MiB–1 GiB`; target a documented 512–640 MiB combined ceiling, with MinIO explicitly external.
18. Use Compose-supported `cpus`, `mem_limit`, `pids_limit`, restart, stop-grace-period, healthcheck, and volume settings—not Swarm-only limits that ordinary Compose ignores.
19. Do not publish MinIO console/API or backend ports on all interfaces in local defaults. Do not place credentials in image layers beyond the unavoidable browser-visible assignment API key; clearly label that key as a demo limitation.

# Phase 4 — clean-clone and repository hygiene

20. Remove the blanket `dataset/` ignore. Do not edit any dataset file. Verify all 50 JPEGs and `predictions.db` remain byte-for-byte unchanged and become visible to Git so the user can commit them for clean-clone reproducibility.
21. Add precise ignores for `/backend/api`, local seed/API binaries, frontend/build/test caches, coverage, temp DB/cache files, Docker override files containing secrets, OS/editor artifacts, and recovery archives. Never ignore required lockfiles, prompts, Dockerfiles, Compose, OpenAPI, or dataset.
22. Do not run `git add`, commit, push, rewrite history, remove old commits, or delete recovery backups. The historic `backend/api` blob remains a known task-018 hygiene item; report it explicitly.
23. Ensure generated/build outputs remain untracked after every check. Do not copy `.env` into Docker contexts or images.

# Phase 5 — README rewrite

24. Rewrite README as the actual submission guide while retaining a concise product explanation and assignment-relevant design:
   - prerequisites and exact clean-clone quick start;
   - copy env, build/start local stack, health wait, rerunnable seed, open web URL;
   - native backend/frontend development commands;
   - complete backend/frontend test/lint/build commands and optional integration tags;
   - local and external-S3 production Compose commands;
   - environment-variable table, distinguishing internal/public S3 endpoints and browser-visible API key;
   - Data API auth, public thumbnail contract, metrics/health endpoints;
   - architecture/data flow from SQLite → presigned MinIO originals → generated/cached thumbnails → gallery/viewer;
   - cursor pagination/filter semantics and bbox/DPR geometry;
   - cache key, singleflight, atomic write/eviction, resource budgets, CDN behavior;
   - structured logs/request IDs/metrics;
   - trade-offs and security limitations, especially static browser API key, SQLite read-only, local disk cache, one-instance singleflight, no optional map unless task 016 is later completed;
   - reset/cleanup commands that distinguish `down` from destructive `down -v`.
25. Every documented command must be executed or mechanically validated in this task. Do not claim deployment, browser/visual behavior, resource measurements, or clean-clone success that was not observed.

# Phase 6 — verification

Run each language gate once after code is stable:

```bash
cd backend
gofmt -w internal/config internal/objectstorage
go test -race ./...
go vet ./...
go build -o /tmp/scout-api ./cmd/api
go build -o /tmp/scout-seed ./cmd/seed
cd ..

pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend generate:api
pnpm --dir frontend test --run
pnpm --dir frontend typecheck
pnpm --dir frontend lint --max-warnings=0
pnpm --dir frontend build
```

Then validate packaging:

1. `docker compose config` and `docker compose -f compose.production.yaml config` with explicit non-secret placeholder external-S3 values.
2. Build API and web images from a clean Docker cache boundary sufficient to prove Dockerfiles do not depend on local node_modules/dist/binaries.
3. Start the local stack with the documented command and wait for MinIO/API/web health.
4. Run seed twice; both runs must succeed without changing SQLite/dataset.
5. Through the public web origin, verify index/assets, `/api/healthz`, authenticated photo page, one thumbnail 200, repeat/ETag/304, and `/api/metrics`.
6. Fetch one returned `originalUrl` from the host and prove the seed container can PUT through the same public presigned hostname.
7. Inspect container users, read-only mounts/filesystems, health, restart/resource/PID limits, and one no-stream CPU/memory snapshot. Do not include MinIO in API+web production budget.
8. Stop with non-destructive `docker compose down`; preserve named data/cache volumes.
9. Re-run `git diff --check`, `git status --short`, confirm no build artifacts, and verify dataset hashes/integrity unchanged.

If Docker/network access is unavailable, do not fabricate results: complete static files/config validation, report the exact blocked commands, and leave runtime smoke for task 018.

# Acceptance criteria

- A documented clean clone with the committed dataset can build, start, seed, and serve the working gallery/viewer from one web URL.
- Browser and seed container receive reachable presigned original URLs while API storage I/O uses the internal S3 endpoint.
- API/web images are multi-stage, minimal, non-root, health-checked, and resource bounded; production topology excludes MinIO.
- SQLite is read-only, thumbnail cache is persistent/bounded, and no catalog-wide load is introduced.
- README accurately describes verified commands, architecture, trade-offs, security, and cleanup.
- Existing backend/frontend gates remain clean and no secret/artifact/dataset mutation is introduced.

# Final report

Report changed files, exact image tags, internal/public S3 design, local/production topology, resource/security settings, README sections, dataset hash/integrity result, language/Docker/smoke commands and outcomes, measured container snapshot, blocked checks, untracked files that must be committed, and known task-018 history hygiene. Do not begin task 018 or rewrite Git history.
