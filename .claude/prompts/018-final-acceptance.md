# Goal

Perform the final, evidence-based acceptance of the completed Scout submission from a clean clone: repository/history hygiene, deterministic backend/frontend gates, real Docker image builds, local MinIO ingest, API/proxy/UI behavior, pagination at catalog scale, concurrent thumbnail/cache behavior, and the documented 1 CPU / bounded-memory deployment. Do not add product features or silently repair findings; record blockers precisely and create a narrowly scoped follow-up only when acceptance exposes a real defect.

# Safety and prerequisites

This task contains destructive-risk operations and must begin read-only.

1. Run `git status --short`, record the current branch/HEAD, and inspect only the submission diff/status.
2. Docker Desktop Linux engine with Ubuntu WSL integration must answer `docker version` and `docker compose version`. A client-only installation is not enough. If the daemon is unavailable, stop and report the exact enable/start steps; do not claim any runtime acceptance.
3. All intended source, Docker/Compose, prompts, lockfiles, OpenAPI, README, and dataset files must already be committed on the candidate submission branch. The working tree must be clean. `dataset/` must contain 50 JPEGs and `predictions.db` in `HEAD`, not merely as untracked local files. If not, stop and show the exact uncommitted/untracked paths. Do not stage or commit without explicit user authorization in the active chat.
4. The historic `backend/api` binary blob must be absent from the candidate public history. Audit first. Do not run `git filter-repo`, rebase, reset, force-push, delete refs/backups, or otherwise rewrite history without explicit user approval in the active chat. Before any approved rewrite, create a verified backup ref outside the rewritten branch and report the recovery command.
5. Never print `.env`, credentials, API keys, signed query strings, or presigned URLs. Redact secrets and URL queries in all evidence.
6. Preserve `dataset/predictions.db` and all 50 JPEGs byte-for-byte. Use temporary clones, databases, volumes, and cache directories for acceptance fixtures.

Required local toolchain:

- Go `1.26.x`;
- Linux Node.js `24.x` and pnpm `10.34.4` inside WSL—not Windows shims;
- Docker Compose v2 with a running Linux daemon;
- enough free disk for clean clone, image layers, dataset, and temporary volumes.

If any prerequisite fails, stop at preflight. A partial static report is not final acceptance.

# Context manifest

Read only:

```text
CLAUDE.md
README.md
.gitignore
.dockerignore
.env.example
compose.yaml
compose.production.yaml
backend/Dockerfile
frontend/Dockerfile
frontend/nginx.conf
openapi.yaml: endpoint/schema summary needed for the matrix only
.claude/prompts/ROADMAP.md
```

Inspect implementation/tests only to explain a failed gate or a demonstrated acceptance mismatch. Do not reread previous prompts, recursively dump the repository, inspect dataset image bytes, or use subagents.

# Phase 1 — immutable baseline and Git hygiene

1. Record branch, HEAD, clean status, tracked-file count, and tool versions.
2. Verify required files are tracked, including both lockfiles and every dataset file. Confirm exactly 50 canonical-UUID `.jpg` files.
3. Record SHA-256 for `dataset/predictions.db` and a deterministic aggregate SHA-256 manifest for the 50 images. Repeat both after all tests.
4. Run SQLite integrity/read-only checks without modifying the source database: `PRAGMA integrity_check`, expected 50 photos/92 predictions, valid bbox ranges/classes, and no journal/WAL side files.
5. Audit tracked files and all reachable history for binaries, archives, `.env`, credentials, private keys, node_modules, dist, coverage, temporary DB/cache files, and the known historic `backend/api` blob. Distinguish example/demo credentials from real secrets.
6. Verify `.gitignore` does not hide required Dockerfiles, Compose, OpenAPI, lockfiles, prompts, or dataset.
7. Create a temporary clone from the candidate local repository at the exact recorded HEAD. Do not use the dirty source directory for acceptance. Confirm the clone is clean and contains the dataset.

If clean-clone or history hygiene fails, stop before Docker runtime and report the blocker. Never “test” uncommitted working-tree files as a clean clone.

# Phase 2 — clean-clone language gates

Inside the temporary clean clone, install only from committed manifests/lockfiles and run each gate once:

```bash
cd backend
go test -race ./...
go vet ./...
go build -trimpath -o /tmp/scout-accept-api ./cmd/api
go build -trimpath -o /tmp/scout-accept-seed ./cmd/seed
cd ..

pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend generate:api
git diff --exit-code -- frontend/src/shared/api/schema.ts
pnpm --dir frontend test --run
pnpm --dir frontend typecheck
pnpm --dir frontend lint --max-warnings=0
pnpm --dir frontend build
```

Record exact test counts and versions. Generated schema must be unchanged. Remove temporary binaries/build output before the final clone cleanliness check.

# Phase 3 — Compose and image acceptance

1. Copy `.env.example` to a temporary untracked `.env`, replace demo values with non-printed acceptance values, and keep the file outside evidence/output.
2. Validate local and production Compose models with explicit placeholder production S3 values. Inspect effective entrypoints, healthchecks, users, mounts, read-only/tmpfs settings, loopback/public port bindings, restart/grace settings, CPU, memory, PIDs, and persistent volumes.
3. Render local Compose once with default MinIO ports and once with a non-default API port. Prove the MinIO healthcheck configures a matching alias and `mc ready` receives that alias; prove seed executes `scout-seed` directly.
4. Build both images from the clean clone with a clean enough cache boundary to prove they do not depend on host `node_modules`, `dist`, binaries, or ignored secrets. Record final image IDs/tags/sizes and runtime users. Do not use `latest`.
5. Scan image history/config for accidental `.env`, dataset, source tree, credentials, build tools in runtime layers, and unexpected writable paths. Do not print secret values.
6. Verify the API and web image healthcheck commands exist in their runtime images and execute as the declared non-root users.

# Phase 4 — real local-stack smoke

Start the documented local stack from the clean clone. Wait for MinIO, init, API, and web health with a bounded timeout; capture sanitized diagnostics on failure.

1. Run seed twice through the documented Compose profile. Both runs must succeed; SQLite and dataset hashes must remain unchanged.
2. Confirm exactly 50 originals are addressable in MinIO without exposing object credentials or signed queries.
3. Through the public web origin, verify:
   - SPA index 200 with no-cache/revalidation and security headers;
   - one actual `/assets/` JS and CSS response with immutable one-year caching and security headers;
   - unknown SPA route falls back to index;
   - `/api/healthz` and `/api/metrics` proxy correctly;
   - access logs omit query strings, API keys, and signed parameters.
4. Exercise the Data API matrix through `/api`:
   - missing/wrong/correct API key behavior on all three protected operations;
   - public health, metrics, and thumbnail behavior;
   - list defaults and limits `1`, `200`, `0`, `201`, malformed cursor, class, confidence, and same-prediction filter semantics;
   - get known/unknown/malformed photo ID;
   - upload-link known/unknown/malformed ID and strict body/content type;
   - typed error body/status/request ID without internal leakage.
5. Follow pagination across the real 50-photo dataset with a smaller page size. Assert stable ordering, no duplicate/missing IDs, opaque cursor behavior, and absent `next_token` on the final page.
6. Verify a redacted returned `originalUrl` uses the public endpoint and can be fetched from the host. Verify a seed-container upload can use the same public hostname while API object reads remain on the internal endpoint.

# Phase 5 — scale and thumbnail/cache acceptance

The sample dataset has only 50 rows, so validate catalog size above 200 with an isolated temporary SQLite copy/fixture—never by editing `dataset/predictions.db`.

1. Reuse the production schema and generate at least 225 valid photos with deterministic canonical IDs/timestamps in a temporary DB. Predictions may be minimal but must include enough same/different-prediction cases for filters. Do not add permanent source or dataset files.
2. Start an isolated API instance against that temporary read-only DB and the acceptance object store. Traverse pages using `limit=200` and smaller limits. Prove all 225+ unique rows are reachable, order is stable, cursors terminate, and no OFFSET/total cap appears.
3. For one real seeded image, verify thumbnail parsing boundaries for width, DPR, quality, malformed UUID, unknown UUID, unsupported methods, and query parameters.
4. Verify dimensions/no-upscale/JPEG response, ETag, Cache-Control, repeat cache hit, conditional `304`, and byte-identical cached response.
5. Delete only the isolated thumbnail cache, then launch a bounded burst of concurrent identical requests plus a small set of distinct variants. Assert:
   - all identical requests succeed with identical bytes/ETag;
   - one shared generation occurs for the identical cold key according to documented metrics/semantics;
   - configured generation concurrency remains `1` by default;
   - no race, temp-file leak, unbounded goroutine growth, or cache corruption occurs;
   - disk cache max bytes and eviction behavior remain bounded.
6. Repeat one thumbnail after API restart to prove persistent-volume cache reuse without decoding the original again.

# Phase 6 — UI and accessibility acceptance

Use a real browser against the public web origin; do not infer UI correctness from unit tests alone.

1. Verify initial loading, populated gallery, empty filter result, recoverable API error, and broken-image fallback.
2. Verify cursor paging, class/confidence filters, reset, full final-page label semantics, and absence of stale prior-page/filter data.
3. At representative mobile, tablet, and desktop viewports, inspect responsive columns, lazy images, `srcset` candidates, and bounding boxes aligned to visible objects.
4. Open the viewer and verify contained-image bbox alignment, predictions, previous/next bounds, Escape/close, focus restoration, keyboard-only operation, and no nested interactive controls.
5. Check browser console/network for React errors, failed resources, secret-bearing URLs, unexpected duplicate requests, and obvious accessibility violations. Record screenshots only if they contain no secrets or signed queries.
6. Do not require or claim the optional greenhouse map; task 016 remains intentionally unimplemented.

# Phase 7 — resource and production-topology acceptance

1. Inspect the effective production API+web limits: combined CPU must be about `1`, combined memory within the documented 512–640 MiB ceiling, PIDs bounded, API `GOMAXPROCS=1`, practical `GOMEMLIMIT` headroom, thumbnail concurrency `1`, SQLite read-only, and cache volume persistent/bounded. External object storage is excluded from this budget.
2. Run API+web under the declared limits against the acceptance S3/MinIO endpoint. Capture `docker stats --no-stream` at idle, during paginated list traffic, and during the bounded thumbnail burst.
3. Confirm health remains stable, no OOM/restart occurs, generation stays responsive, and memory/CPU observations fit the documented limits. Measurements are evidence, not promises of global performance.
4. Validate graceful stop/restart, persistent cache, read-only filesystem enforcement, non-root users, and no privilege escalation.
5. Validate production Compose contains API+web only and no bundled MinIO/seed service.

# Phase 8 — teardown and final audit

1. Stop acceptance stacks with non-destructive `docker compose down`; remove only explicitly named temporary acceptance containers/volumes after evidence is recorded. Do not delete developer/user volumes.
2. Recompute dataset/database hashes and integrity; they must match Phase 1 exactly.
3. Remove temporary `.env`, DBs, binaries, dist/node_modules, screenshots containing sensitive URLs, and clean-clone directory.
4. Confirm the candidate source working tree and committed clean clone remain clean. Run `git diff --check` and repeat secret/artifact/history checks.
5. Compare every README quick-start/test/production/cleanup claim with what was actually observed. Correct documentation only if the mismatch is purely documentary and explicitly report it; any product/runtime defect becomes a focused follow-up rather than an unreviewed acceptance-time implementation.

# Acceptance criteria

- Exact committed HEAD reproduces from a clean clone with the committed dataset and frozen dependencies.
- Full backend/frontend gates pass with generated API types unchanged.
- API/web images build, run non-root, stay healthy, and contain no secret/build/dataset leakage.
- Local stack starts, seeds twice, serves the gallery, originals, thumbnails, metrics, and typed API behavior through one web origin.
- Internal/public S3 routing works from API, seed container, and host browser.
- Cursor pagination reaches every row in an isolated catalog above 200 entries.
- Thumbnail singleflight/cache/ETag/eviction/concurrency behavior survives real concurrent traffic.
- Browser gallery/viewer/bbox/responsive/accessibility checks pass.
- API+web respect declared ~1 CPU and bounded-memory/PID/filesystem limits without OOM/restart in the acceptance workload.
- Dataset remains byte-identical; repository and reachable history contain no forbidden secret/artifact, including historic `backend/api`.
- README matches observed commands and limitations; no blocked check is presented as passed.

# Final report

Report candidate branch/HEAD, clean-clone source, tool/image versions and IDs, tracked dataset/hash results, exact gate/test counts, Compose/image security and limits, seed/API/proxy/original/thumbnail/browser matrix, >200 pagination evidence, concurrent cache metrics, resource snapshots, teardown/final Git hygiene, README corrections, and every blocked or failed check. Redact credentials and signed URL queries.

Finish with exactly one verdict:

- `ACCEPTED` — every mandatory criterion passed and public-history hygiene is clean;
- `BLOCKED` — a prerequisite such as Docker, committed dataset, clean clone, or explicit history-rewrite approval is missing;
- `CHANGES REQUIRED` — acceptance found a reproducible defect, followed by a concise finding list suitable for one focused hardening prompt.

Do not commit, push, publish, rewrite history, or mark the roadmap complete without explicit user authorization and a fully `ACCEPTED` result.
