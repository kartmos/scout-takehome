# Goal

Perform the final submission-hardening pass over the current Scout working tree.
Close the remaining strict-audit findings without weakening accepted behavior:

1. make catalog pagination avoid a full-table temporary sort;
2. return the typed centralized API error shape for unknown routes and unsupported
   HTTP methods;
3. make map-driven near filtering complete for catalogs larger than the current
   200-marker visualization bound;
4. make thumbnail cache-miss metrics truthful when generation fails;
5. fix Linux/macOS Docker prerequisites in README;
6. remove the remaining explicit `any` and actionable visual magic numbers;
7. preserve and verify the pending AMD64/ARM64 Docker/CI work, including QEMU.

Do not commit or push. The user will make the final commit only after this task
reports a clean gate and `READY FOR REVIEW`.

# Current working state

- Work on the current `main` branch and preserve all existing user changes.
- Begin with `git status --short`, record `HEAD`, and inspect the complete scoped
  diff. The working tree currently includes pending task-019 changes in both
  Dockerfiles, both workflows, README, ROADMAP, and the untracked task-019 prompt.
- `.github/workflows/ci.yml` already requires
  `docker/setup-qemu-action@v3` before Buildx. Preserve it.
- Task 019 remains `PENDING CI`; do not mark it `DONE` before the pushed GitHub
  Actions jobs actually pass.
- The public repository currently points to the previous committed HEAD. Do not
  treat local uncommitted files as already submitted.
- The local Compose stack, double seed, frontend/health/metrics smoke checks, and
  backend integration test with the race detector have previously passed. Rerun
  the final relevant gates; do not rely on that report as proof.
- Do not rewrite history, reset unrelated work, stage files, commit, push, publish
  images, change credentials, or expose `.env` values.

# Sources of truth

Read completely before editing:

```text
TODO.md
openapi.yaml
CLAUDE.md
```

Then inspect only the relevant implementation and tests listed below. `TODO.md`
wins for product behavior; `openapi.yaml` wins for the Data API wire contract.
Any additive map/location API must be documented and generated consistently, but
must not change existing response fields or filter semantics.

# Context manifest

```text
README.md
compose.yaml
compose.production.yaml
backend/Dockerfile
frontend/Dockerfile
.github/workflows/ci.yml
.github/workflows/integration.yml
.claude/prompts/ROADMAP.md

backend/internal/repository/sqlite/repository.go
backend/internal/repository/sqlite/cursor.go
backend/internal/repository/sqlite/repository_test.go
backend/internal/repository/sqlite/cursor_test.go
backend/internal/httpapi/router.go
backend/internal/httpapi/errors.go
backend/internal/httpapi/router_test.go
backend/internal/httpapi/photos.go
backend/internal/httpapi/photos_test.go
backend/internal/thumbnail/cache.go
backend/internal/thumbnail/service.go
backend/internal/thumbnail/service_test.go
backend/internal/observability/metrics.go
backend/internal/observability/metrics_test.go
backend/integration/smoke_test.go

frontend/src/pages/gallery/GalleryPage.tsx
frontend/src/features/map/GreenhouseMap.tsx
frontend/src/features/map/mapGeometry.ts
frontend/src/entities/photo/PhotoCard.tsx
frontend/src/shared/lib/thumbnailCandidates.ts
frontend/src/shared/api/scoutApi.ts
frontend/src/test/setup.ts
frontend/src/test/GreenhouseMap.test.tsx
frontend/src/test/GalleryPage.test.tsx
frontend/src/test/PhotoCard.test.tsx
frontend/src/entities/api/__generated__/schema.ts
```

Inspect one directly connected file only when a demonstrated compile/test blocker
requires it. Do not perform unrelated refactors or redesign accepted UI.

# Finding 1 — scalable cursor pagination

The current query orders by `(captured_at DESC, id DESC)`, but the supplied
read-only database has no matching index. SQLite therefore performs `SCAN p` plus
`USE TEMP B-TREE FOR ORDER BY` for every page. Cursor syntax alone is not enough
if each page rescans and sorts the whole catalog.

1. Preserve direct, read-only use of `dataset/predictions.db`; do not mutate,
   replace, copy, or migrate the provided database.
2. Use a stable ordering supported by an existing index. Prefer primary-key
   ordering by canonical photo `id`, with an opaque versioned cursor carrying the
   last ID, unless inspection proves another existing indexed ordering.
3. Preserve:
   - default limit 50 and allowed range 1..200;
   - opaque cursor validation and deterministic page boundaries;
   - no duplicates or skipped rows across pages;
   - optional class/confidence filters matching the same prediction;
   - all predictions returned for every matched photo;
   - no total catalog cap.
4. Update cursor implementation/tests only as needed. Reject stale/invalid cursor
   shapes with the existing typed validation error.
5. Add a query-plan regression test using the supplied schema or a representative
   fixture. Prove the unfiltered pagination path does not use a temporary B-tree
   for ordering and uses the primary-key index/range seek after a cursor.
6. Do not add an in-memory full-catalog sort or load all IDs before paging.

# Finding 2 — typed JSON for 404 and 405

The OpenAPI description requires every API error to contain `request_id`,
`message`, and machine-readable `code`, shaped centrally without stack traces.
Go `ServeMux` currently emits plain-text built-in responses for unknown routes and
method mismatches.

1. Route unknown paths and unsupported methods through the existing centralized
   `WriteError`/typed-error path.
2. Preserve automatic request ID generation/propagation and include the same ID in
   the response header and JSON body.
3. Unknown route:
   - HTTP 404;
   - JSON `NotFound` error using a safe resource identifier;
   - no internal route table or stack details.
4. Unsupported method:
   - HTTP 405;
   - typed JSON error with a stable machine-readable code;
   - correct `Allow` header;
   - no plain-text `http.Error` body.
5. If OpenAPI lacks an explicit MethodNotAllowed schema/response, add the smallest
   consistent typed schema and response definition, regenerate frontend types,
   and verify no unrelated schema drift. Do not misuse `ValidationError` merely to
   avoid documenting 405.
6. Add direct tests for unknown route and wrong method asserting status,
   content-type, complete typed body, matching request ID, `Allow`, and absence of
   stack/internal details.
7. Preserve `/healthz`, `/metrics`, auth middleware, CORS behavior, and all existing
   photo/thumbnail routes.

# Finding 3 — complete near filtering with a bounded map

The map intentionally visualizes at most 200 markers and discloses that bound.
That is acceptable as a rendering/resource bound, but the current client-side
near filter also searches only those 200 records. On a larger catalog, clicking a
location can omit matching photos after the first marker page.

Implement the smallest scalable correction:

1. Keep map rendering and DOM work explicitly bounded. Do not cursor-crawl the
   entire catalog into React and do not remove the existing “first 200” disclosure.
2. Move the applied near-location gallery filter to the backend so results are
   complete and cursor-paginated:
   - accept a complete tuple such as `nearX`, `nearY`, and `nearRadius`;
   - require all three together and reject partial tuples;
   - validate finite coordinates inside `0..40` and a positive bounded radius;
   - use the existing inclusive 3 m definition from the UI;
   - combine location with class/confidence filters conjunctively;
   - retain same-prediction semantics for class + confidence;
   - return all predictions for matched photos;
   - preserve cursor pagination and stable indexed ordering.
3. Add this as an additive, documented extension to `GET /photos` in
   `openapi.yaml`, regenerate `schema.ts`, and wire RTK Query using generated
   types. Do not change existing clients that omit the location tuple.
4. GalleryPage must use the server response for an applied location instead of
   filtering `mapPhotos` client-side. Normal cursor pagination may remain visible
   for location results, or use a clearly bounded incremental “load more” flow;
   it must never pretend a 200-record map sample is the complete near result.
5. Expanded-map draft preview may remain an explicitly approximate count derived
   from the bounded marker sample, but label it as a preview/sample when
   `hasMore` is true. Applying the draft must switch to the complete server query.
6. Preserve compact immediate apply, expanded draft/apply/cancel, real `x,y`
   marker placement, Konva zoom/pan, selected radius, shared class/confidence
   state, viewer result scoping, and clear-location behavior.
7. Add repository/API tests for location validation, radius boundary inclusion,
   pagination, and composition with class/confidence. Add focused GalleryPage/map
   tests proving applied filtering uses the API result and no longer derives the
   final gallery solely from the 200-marker sample.
8. Do not add a second database, GIS dependency, unbounded response, or full
   catalog aggregation endpoint in this final pass.

# Finding 4 — truthful cache-miss metrics

The metric describes an initial cache lookup miss, but the service currently calls
`OnCacheMiss` only after `GetOrCreate` succeeds. Failed cold generations therefore
increment generation errors without incrementing misses.

1. Count exactly one cache miss whenever the initial lookup misses, regardless of
   later generation success or failure.
2. Preserve exactly one hit for a true initial hit and avoid double counting
   singleflight waiters.
3. Keep generation duration/error metrics truthful and independent from hit/miss.
4. Prefer an explicit cache result/outcome contract over parsing errors or doing a
   second disk lookup.
5. Add tests for successful hit, successful miss, failed miss, and concurrent
   same-key requests. Assert metric hooks precisely.

# Finding 5 — README platform prerequisites

README currently presents Docker Desktop as the general prerequisite while also
claiming native Linux support.

1. State clearly:
   - Linux: current Docker Engine with Compose v2;
   - Intel/Apple Silicon macOS: current Docker Desktop;
   - Windows, if retained: Docker Desktop with WSL2 integration.
2. Keep the architecture-neutral `docker compose up -d --build --wait` command.
3. Do not claim ARM64 CI success before the remote job actually passes.
4. Preserve production external-S3 requirements and unsupported-architecture
   disclosure.

# Finding 6 — remaining stack recommendations

## Explicit any

Remove the `globalThis as any` escape in `frontend/src/test/setup.ts` without
weakening the mock. Define the smallest typed global/constructor interface or use
`vi.stubGlobal` with proper types. Remove the local eslint suppression when it is
no longer required. Do not introduce `unknown as any` elsewhere.

## Visual magic numbers

Extract meaningful named constants for repeated or semantic visual tuning values,
including:

- greenhouse background opacity;
- bbox matched/dimmed opacity and outline/foreground stroke widths;
- map axis/legend offsets that express layout rather than one-off arithmetic.

Do not mechanically outlaw every numeric literal. Loop bounds, zero checks,
coordinate formulas, array indices, and the already named `CARD_SIZES` CSS source
string are not improved by a global `no-magic-numbers` rule. Do not add that noisy
lint rule in this final pass. Keep behavior and visuals pixel-identical.

# Preserve task-019 multi-platform work

1. Keep both existing workflows; do not create a third workflow.
2. Keep QEMU before Buildx on the x64 multi-platform build job.
3. Keep native integration runners for `linux/amd64` and `linux/arm64`, architecture
   assertions, double seed, smoke checks, race integration, failure logs, and
   always-cleanup.
4. Keep both Dockerfile builder stages on `$BUILDPLATFORM` and runtime stages
   target-native.
5. No registry login, image publication, commit, or push.
6. Keep roadmap task 019 `PENDING CI`; add task 020 as `READY FOR REVIEW` only
   after every locally available gate passes. Do not claim remote CI success.

# Focused implementation tests

During development, run only directly affected packages/files:

```bash
go test -race ./internal/repository/sqlite ./internal/httpapi ./internal/thumbnail ./internal/observability
pnpm --dir frontend test --run \
  src/test/GalleryPage.test.tsx \
  src/test/GreenhouseMap.test.tsx \
  src/test/PhotoCard.test.tsx
```

Then inspect the scoped diff before proceeding to the full gate.

# Full final gate

Run every command from a clean command environment and report each exit status.
Do not weaken flags, omit explicit test files, or hide warnings.

## Backend

```bash
cd backend
go test -race ./...
go vet ./...
go build ./...
cd ..
```

## Frontend

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend generate:api
git diff --exit-code -- frontend/src/entities/api/__generated__/schema.ts
pnpm --dir frontend test --run
pnpm --dir frontend typecheck
pnpm --dir frontend lint --max-warnings=0
pnpm --dir frontend build
```

## Static/runtime configuration

```bash
docker compose config
docker compose -f compose.production.yaml config
git diff --check
```

Supply safe temporary environment values for production Compose interpolation;
never print real `.env` contents.

## Runtime acceptance

Use the current local stack without deleting existing user volumes:

```bash
docker compose up -d --build --wait --wait-timeout 120
docker compose --profile seed run --rm seed
docker compose --profile seed run --rm seed
curl -fsS http://127.0.0.1:8090/ >/dev/null
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/metrics | grep scout_
```

Run the integration suite exactly as documented:

```bash
cd backend
SCOUT_API_URL=http://127.0.0.1:8080 \
SCOUT_API_KEY=scout-dev-api-key-do-not-use-in-production \
SCOUT_DATASET_DIR="$PWD/../dataset/images" \
go test -race -tags integration ./integration
cd ..
```

If the host toolchain cannot run the race detector, use an ephemeral official Go
container attached to the Compose network and report that execution environment.
Stop only containers created by this task and preserve named volumes.

## Multi-platform build

Confirm both application Dockerfiles still build for:

```text
linux/amd64
linux/arm64
```

Use non-publishing Buildx output/cache. Do not push images. If this is prohibitively
slow after no Dockerfile changes beyond the already verified task-019 diff, record
the exact prior local evidence and leave remote verification pending; do not call
task 019 complete.

# Final consistency audit

Before reporting:

1. Run `git status --short` and inspect the complete diff.
2. Confirm only intended files changed.
3. Confirm both task prompts `019` and `020` are present and ready to be included
   in the user's final commit as AI artifacts.
4. Search for:
   - explicit `any` and new lint suppressions;
   - plain-text backend error paths;
   - client-only final near filtering over `mapPhotos`;
   - full-catalog frontend cursor crawling;
   - temporary B-tree ordering in pagination tests;
   - cache hit/miss double counting;
   - secrets, signed URLs, `.env`, build outputs, or test artifacts;
   - README/ROADMAP claims stronger than evidence.
5. Confirm no staged files, commit, push, registry login, or image publication
   occurred.

# Acceptance criteria

- Cursor pagination uses an existing index and does not sort the full catalog per
  page.
- All backend 404/405 responses use the typed centralized JSON error contract.
- Applied location filtering is complete, server-side, and cursor-paginated even
  though map rendering remains bounded.
- Failed cold thumbnail generation increments one cache miss.
- Linux/macOS/Windows Docker prerequisites are accurate.
- No explicit `any` remains and meaningful visual tuning values are named.
- Existing gallery, bbox, viewer, map, API, thumbnail, runtime, and multi-platform
  behavior does not regress.
- Full backend/frontend gates and runtime acceptance pass.
- Task 019 remains pending remote CI; no unverified success is claimed.
- No commit or push occurs.

# Final report

Report:

- exact files changed;
- pagination query-plan evidence;
- typed 404/405 behavior;
- complete server-side near-filter semantics and UI behavior;
- cache metric behavior;
- README/stack cleanup;
- focused and full test counts;
- runtime/seed/integration results;
- multi-platform build evidence and remote CI status;
- exact `git status --short`;
- any residual risk before the user's final commit.

Finish with exactly one of:

```text
READY FOR FINAL COMMIT
CHANGES REQUIRED
BLOCKED
```
