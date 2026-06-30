# Goal

Implement production-only bounded disk caching and a public thumbnail endpoint around task 010. Identical misses collapse to one generation, hits never open/decode originals, writes are atomic, disk use stays bounded, and responses support CDN/browser caching. Do not add metrics or tests.

# Context manifest

Read only `CLAUDE.md`, README thumbnail/runtime paragraphs, and these production contracts:

```text
backend/internal/thumbnail/
backend/internal/httpapi/router.go
backend/internal/httpapi/errors.go
backend/internal/repository/sqlite/repository.go
backend/internal/config/config.go
backend/cmd/api/main.go
backend/go.mod
.env.example
```

From the repository, inspect only exported `GetPhoto` API/constants needed for wiring. Verified facts:

- task 010 owns request parsing, canonical hashed identity, generation limits, and safe typed errors;
- thumbnail delivery is a separate public image route suitable for `<img>` and CDN use;
- cache hits must never call `OpenOriginal` or decode JPEG;
- task 012 owns metrics, all backend tests/race/smoke, review, and corrective findings.

Do not inspect test files, previous prompts, frontend, dataset, or Compose. Do not use subagents.

# Dependency and scope

Add `golang.org/x/sync/singleflight` as a direct pinned dependency resolved by `go mod tidy`. Do not add a cache database or framework.

Create focused production files, preferably:

```text
backend/internal/thumbnail/cache.go
backend/internal/thumbnail/service.go
backend/internal/httpapi/thumbnails.go
```

Modify only required production wiring/config/dependency files: `router.go`, `config.go`, `main.go`, `go.mod`, `go.sum`, `.env.example`. Do not create or modify `_test.go`, fixtures, smoke scripts, metrics, frontend, README, OpenAPI, or Compose.

# Production contract

1. Add public `GET /photos/{photoId}/thumbnail?width=<int>&dpr=<number>&quality=<int>`. It must not require `X-API-Key`; existing Data API auth and `/healthz` remain unchanged.
2. Reject unknown/repeated/empty query values and invalid UUIDs. Delegate width/DPR/quality semantics to task 010; do not duplicate parsing rules.
3. Load photo metadata through a narrow repository interface and resolve canonical representation before cache access. Preserve typed 400/404/500 JSON errors before image bytes are committed.
4. Configure cache directory and maximum bytes with explicit safe development defaults and a positive bounded maximum. Document via `.env.example` that production needs a persistent writable volume.
5. Store JPEGs only under task-010 hashed keys. Never use raw IDs, query values, URLs, or path fragments as filesystem paths.
6. A hit opens/streams the cached file only, without generator/storage calls or whole-file memory buffering.
7. Collapse identical misses with `singleflight` by canonical key. The leader writes one unique temp file; context-aware followers wait and then independently open the committed entry.
8. Write atomically in the cache filesystem: create temp, stream generation, enforce output byte ceiling, sync/close as appropriate, validate nonzero size, rename. Remove temp files on every failure/cancellation.
9. Keep cache at or below max bytes. Maintain bounded metadata, serialize index/eviction mutation, and evict least-recently-used or oldest-accessed entries until under budget. Never store image bytes in the index.
10. On startup create the directory with restrictive permissions, refuse unsafe/symlink roots, remove stale temp files safely, index valid entries, and evict overflow before serving. Never touch paths outside the cache root.
11. Update recency without synchronous metadata writes on every hit, one goroutine per request, or unbounded queues.
12. An entry larger than the total cache budget fails without commit. Generation output is byte-limited so a bad result cannot fill disk.
13. Derive a stable quoted `ETag` from canonical identity/version. Matching `If-None-Match` returns 304/no body. Hits and misses use the same ETag.
14. Successful 200 responses use `Content-Type: image/jpeg`, exact `Content-Length`, `ETag`, and `Cache-Control: public, max-age=31536000, immutable`.
15. Stream committed files with `http.ServeContent` or equivalent range-capable standard-library behavior. Never append JSON after image output starts.
16. Map thumbnail invalid/corrupt/not-found/internal categories through the existing safe error writer. Missing database photo or original returns typed 404; cache/infrastructure failure returns safe 500.
17. Wire repository/generator/cache into `main`. Initialize cache before listen, close owned resources on shutdown, and fail startup safely on unusable cache config/directory.
18. Task-010 semaphore remains the only generation concurrency control; do not add a worker pool that multiplies decoded-image concurrency.
19. Never log raw query strings, photo IDs as metrics-like keys, cache keys/paths, signed URLs, or secrets.

# Out of scope

- Metrics, tests, race/singleflight checks, integration/load smoke, cache warming, distributed locks, CDN deployment, frontend, or Docker volume wiring.
- Changing task-010 semantics except a narrow compile-time interface adjustment that is reported.
- In-memory image cache, purge API, stale-while-revalidate, caller formats, or object-version tracking.

# Acceptance criteria

- Production code exposes a public bounded thumbnail endpoint with stable identity and CDN-friendly responses.
- Identical misses collapse to one atomic generation; hit/304 paths never decode originals.
- Cache cannot remain above configured budget after reconciliation/eviction; temp/partial files are cleaned safely.
- Main/config wiring compiles and Data API auth structure remains unchanged.
- No tests or metrics are added or run; verification is deferred to task 012.

# Verification

Do not create, inspect, or run tests. Perform only:

```bash
cd backend
gofmt -w internal/thumbnail internal/httpapi internal/config cmd/api
go mod tidy
go build ./internal/thumbnail ./internal/httpapi
go build -o /tmp/scout-api ./cmd/api
cd ..
git diff --check
git status --short
```

Do not start API, MinIO, or thumbnail generation.

# Final report

Report changed production files/dependency, endpoint, cache/singleflight/eviction design, HTTP caching/errors, startup ownership, build result, and assumptions. Confirm tests/metrics were deferred to task 012. Do not begin task 012.
