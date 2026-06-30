# Goal

Finish backend observability, create the deferred backend test suite for tasks 009–011, and run the complete backend acceptance gate. Prove seed → Data API → MinIO → thumbnail behavior, bounded cache/concurrency, safe metrics, race cleanliness, vet, and builds. Do not implement frontend or silently fix unrelated review findings.

# Prerequisite

Run this prompt only after tasks `009`, `010`, and `011` are implemented, compile, and have been reviewed by Codex at production-diff level. If any is absent or does not compile, stop and report the missing prerequisite instead of recreating it inside task 012.

# Context manifest

Start with `git status --short` and a scoped diff. Read:

- `CLAUDE.md`;
- README requirements for tests, logs, metrics, thumbnails, runtime, and global clients;
- OpenAPI only when asserting the three Data API operations/errors;
- production files created/changed by tasks 009–011;
- existing test helpers only when extending their package;
- `.env.example`, `compose.yaml`, `backend/go.mod`;
- `.claude/skills/backend-review/SKILL.md` only to understand the separate review gate; do not invoke it inside this implementation context.

Verified facts:

- tests were intentionally deferred from 009–011 to this block gate;
- existing tests through task 008 already cover domain, repository, HTTP infrastructure, Data API, object storage, and configuration; do not duplicate them;
- object storage remains external to the API memory budget;
- thumbnail generation concurrency defaults to `1`, maximum `2`; cache disk use is bounded;
- task 013 must not start until this implementation gate and separate backend review are clean.

Use targeted reads. Do not reread previous prompts, frontend, dataset images except the one selected by integration smoke, or Git history. Do not use subagents.

# Dependency and scope

Add the official Prometheus Go client as direct pinned dependencies resolved by `go mod tidy`, using `github.com/prometheus/client_golang/prometheus` and `promhttp`. Do not add another metrics framework.

Production changes may touch only observability/wiring directly needed for metrics, preferably:

```text
backend/internal/observability/
backend/internal/httpapi/middleware.go
backend/internal/httpapi/router.go
backend/internal/thumbnail/
backend/cmd/api/main.go
backend/go.mod
backend/go.sum
```

Test changes may create or extend focused `_test.go` files for seed, thumbnail core/cache/endpoint, metrics, and one build-tagged backend integration smoke. Modify adjacent production code only when a new test demonstrates a real defect; keep each fix narrow and report it.

# Phase 1 — metrics production code

1. Expose public `GET /metrics` through `promhttp.Handler`; it must not require the Data API key. Preserve public `/healthz`, public thumbnail delivery, authenticated Data API, CORS, recovery, and request IDs.
2. Use an injected/custom Prometheus registerer/gatherer so router construction and tests cannot panic from duplicate global registration.
3. Record HTTP request total, duration, and error/status outcome. Labels may contain only bounded values such as normalized route pattern, method, and status/status-class.
4. Record thumbnail cache hit/miss, generation total/error, and generation duration. If useful, record eviction total and current cache bytes/entries as gauges.
5. Never use photo ID, UUID, cursor, raw path, query, URL, cache key, filename, error text, user agent, origin, request ID, or API key as a metric label.
6. Instrument through narrow interfaces/hooks around the actual operation boundaries. Avoid package globals, duplicate counting, high-frequency logging, and one goroutine per observation.
7. `/metrics` must not expose secrets or signed URLs. Scraping it must not require MinIO or trigger thumbnail generation.

# Phase 2 — deferred focused tests

Add concise, table-driven tests for behavior not already covered before task 009. Prefer shared helpers and generated in-memory JPEGs; do not create a giant duplicate transport matrix.

## Seed client

8. With temporary files and `httptest`, cover exact upload-link request/auth/body, exact signed PUT headers/body, and proof that `X-API-Key` never reaches object storage.
9. Cover rerun of the same key, invalid/duplicate/unsupported/symlink input before network calls, redirect rejection, sanitized failure output, cancellation, and worker concurrency never exceeding configuration.
10. Use channels/atomics, not sleeps, for scheduling/concurrency assertions.

## Thumbnail request/core

11. Cover width/DPR/quality defaults and boundaries, malformed/empty/unknown/non-finite inputs, deterministic rounding, aspect ratio, no-upscale behavior, output-pixel ceiling, and equivalent canonical keys.
12. Generate small JPEGs in memory to cover correct output dimensions/media type, direct no-resize path, corrupt/non-JPEG input, decoded-vs-database dimension mismatch, oversized source metadata, safe error categories, context cancellation, and original stream close.
13. Prove generation concurrency does not exceed configured `1`/`2`, waiting cancellation does not consume a slot, and no goroutine remains blocked.

## Cache and endpoint

14. With `t.TempDir`, cover hit bypassing generator/storage, miss commit, stable ETag, 304, Content-Length/Type/Cache-Control, range behavior if implemented, public route, and unchanged Data API auth.
15. Prove many simultaneous identical misses invoke generation exactly once and every successful caller reads the same complete bytes. Run this test under the race detector.
16. Cover failed/cancelled generation cleanup, no partial entry, oversized output rejection, startup stale-temp cleanup, symlink/path safety, restart reconciliation, recency/eviction, and cache bytes staying at or below configured maximum.
17. Avoid flaky filesystem timestamp assumptions: inject a clock or deterministic metadata seam only if production code lacks one and a narrow change improves design.

## Metrics

18. Gather from a fresh registry and verify expected HTTP/cache/generation series change once per operation.
19. Inspect metric descriptors/labels and prove no dynamic identifier/query/cache key/URL appears. Verify `/metrics` is public and does not call repository/storage/generator.

# Phase 3 — one real backend smoke

20. Add one `integration` build-tagged smoke using environment configuration and real local API/MinIO. Ordinary `go test ./...` remains offline.
21. Select one dataset JPEG deterministically, then exercise only public contracts:
    - request upload link with API key;
    - PUT original bytes with required signed headers;
    - GET the photo and verify ID/predictions/original URL;
    - fetch original URL and compare bytes;
    - request one thumbnail and verify JPEG content type, decoded dimensions, ETag;
    - repeat thumbnail request and verify identical bytes/ETag;
    - send `If-None-Match` and verify 304;
    - scrape `/metrics` and verify required metric families exist without secrets.
22. Never log signed URLs/API key. Bound response bodies and timeouts. Reruns may overwrite the same object. Do not delete dataset, DB rows, bucket, or named volume.
23. Add one concurrent same-thumbnail smoke burst large enough to demonstrate duplicate suppression but small enough for the target machine. Verify all responses and one representation; internal generation-count proof belongs to unit tests/metrics.

# Phase 4 — complete backend gate

24. Keep test runtime practical: no arbitrary sleeps, huge fixture images, repeated full suites, or `-count=10`. Run each full gate command once.
25. Ensure all tests are deterministic/offline except the tagged smoke. Remove temporary binaries, coverage files, cache directories, test databases, and other artifacts from the repository.
26. Run API for smoke with `GOMAXPROCS=1` and `GOMEMLIMIT=512MiB` to exercise intended resource posture. Object storage remains external. This is a backend bound check, not the final container acceptance from task 018.
27. Inspect final dependencies and remove unused/direct modules. Do not change README, frontend, production containers, or Git history in this stage.

# Out of scope

- Frontend, bbox DOM geometry, viewer/map, production Docker/Nginx, README rewrite, CDN deployment, distributed cache, or new backend features unrelated to metrics/test findings.
- Rewriting already meaningful tests from tasks 001–008 or inflating coverage with trivial getters/constructors.
- Invoking `/backend-review` inside this context, editing the reviewer skill, commits, or pushes.

# Acceptance criteria

- `/metrics` exposes low-cardinality HTTP and thumbnail metrics without secrets.
- Deferred seed/thumbnail/cache/endpoint tests cover required contracts, concurrency, cancellation, atomicity, and bounds.
- One real ingest→read→thumbnail smoke passes through API and MinIO and is safely rerunnable.
- Full ordinary tests remain offline; race, vet, API/seed builds, integration smoke, resource posture, and diff hygiene pass.
- No unresolved test-discovered production defect or generated artifact remains.
- The final report explicitly hands off to a separate clean `/backend-review` context.

# Verification

During implementation, run only the affected package test while stabilizing each phase. After stable code, run the full gate once:

```bash
cd backend
gofmt -w cmd internal integration
go mod tidy
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/scout-api ./cmd/api
go build -o /tmp/scout-seed ./cmd/seed
cd ..
git diff --check
```

Then start Compose and the API with `.env.example`, `GOMAXPROCS=1`, and `GOMEMLIMIT=512MiB`; use a cleanup trap, bounded health wait, and run exactly once:

```bash
cd backend
go test -tags=integration ./integration
```

Stop API and `docker compose down` without `-v` even on failure. Do not rerun unchanged successful full commands.

# Separate reviewer gate

Do not self-review or invoke the reviewer from this implementation context. After this prompt finishes successfully, the user must start a fresh Claude Code context:

```text
/clear
/backend-review
```

The reviewer is read-only and returns severity-ordered findings with file/line references. Do not mark task 012 `DONE` until Codex checks the implementation diff and reviewer report. If actionable findings exist, create `012.1-backend-hardening.md`; otherwise close 012 and proceed to frontend task 013.

# Final report

Report production/test files, metric families/labels, deferred test coverage, smoke/resource results, full gate results, narrow production fixes made from tests, remaining gaps, and the exact reviewer command. Do not begin frontend work.
