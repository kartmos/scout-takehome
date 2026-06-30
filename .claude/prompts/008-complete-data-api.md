# Goal

Implement and wire the complete authenticated Data API from `openapi.yaml`: create an original-photo upload link, list a bounded cursor page of photos, and get one photo. Use exact OpenAPI JSON shapes, enrich every photo with a short-lived presigned `originalUrl`, preserve the existing typed-error boundary, and keep `/healthz` public.

# Context manifest

Start with `git status --short` and a diff scoped to the allowed files. Read only:

- `CLAUDE.md`;
- OpenAPI operations `createUploadLink`, `listPhotos`, `getPhoto`, parameter `PhotoId`, and schemas `UploadLinkRequest`, `UploadLink`, `PhotoPage`, `Photo`, `Prediction`, `BoundingBox`;
- `backend/cmd/api/main.go`;
- `backend/internal/config/config.go` and its tests;
- `backend/internal/httpapi/router.go`, `middleware.go`, `errors.go`, and directly related tests;
- exported repository API/constants in `backend/internal/repository/sqlite/repository.go`;
- DTO inputs in `backend/internal/domain/photo.go`;
- `backend/internal/objectstorage/storage.go`, exported error helpers/categories, and the `New` signature;
- `.env.example`.

Verified facts:

- tasks 004/004.1 provide read-only `PhotoExists`, `GetPhoto`, `ListPhotos`, keyset pagination, default limit `50`, and page limits `1..200`;
- repository filters already use one matching prediction while returning all predictions;
- tasks 005/005.1 provide request IDs, logs, recovery, CORS, `Authenticate`, and centralized `WriteError`;
- tasks 007/007.1 provide `OriginalStorage`, strict S3 config, presigned PUT/GET, safe typed errors, and `objectstorage.New`;
- object keys are exact canonical UUIDs; `domain.Photo` intentionally has no URL;
- `PresignDownload` is local signing and does not prove that an object was uploaded;
- `limit` bounds one page, not the catalog; task 009 owns seed and the external ingest/read smoke.

Do not read previous prompts, the full README, dataset contents, unrelated repository internals, or Compose implementation. Do not use subagents or restate the task.

# Context budget

- Use targeted symbol search and the smallest coherent sections.
- Implement checkpoints in order and run focused tests while iterating.
- Run the full final gate once after all operations and wiring are stable.
- Inspect one directly connected file only if the allowed context proves insufficient, and explain why.

# Scope

Create focused files under `backend/internal/httpapi`, for example:

```text
backend/internal/httpapi/photos.go
backend/internal/httpapi/photos_test.go
backend/internal/httpapi/dto.go
```

Modify only:

```text
backend/internal/httpapi/router.go
backend/internal/httpapi/router_test.go
backend/internal/httpapi/errors.go
backend/internal/httpapi/errors_test.go
backend/internal/config/config.go
backend/internal/config/config_test.go
backend/cmd/api/main.go
.env.example
```

An equivalent small split inside `internal/httpapi` is acceptable. Modify an adjacent test helper only if compilation requires it. Do not change repository, domain, object-storage, OpenAPI, README, Compose, or dependency files unless a demonstrated defect blocks the contract; report such a blocker instead of redesigning those packages.

# Architecture constraints

1. Define narrow consumer-side interfaces for only the repository/storage calls handlers use. Unit tests use fakes and no SQLite, MinIO, Docker, or network.
2. Extend `RouterConfig` with required repository and storage dependencies. Register all three routes with the existing `Authenticate` per route; do not protect `/healthz` or valid CORS preflight.
3. Keep domain models persistence-focused. Use explicit HTTP DTOs instead of adding JSON tags or `OriginalURL` to `domain.Photo`.
4. Keep parsing, DTO enrichment, and transport testable. Add no framework, generated server, reflection validator, global mutable dependency, or second error writer.
5. Retain only the current page: at most 200 photos plus predictions. Never load all pages, use `OFFSET`, count the catalog, or launch unbounded goroutines.
6. Prefer a simple bounded loop for local download signing. Any concurrency must have a small explicit bound, preserve order, cancel on failure, and be justified by tests.

# Checkpoint 1 — configuration and production wiring

1. Add required `SCOUT_DATABASE_PATH`. Reject absent, empty, or whitespace-only values; do not hide a working-directory-dependent default.
2. Add `SCOUT_DATABASE_PATH`, `SCOUT_API_KEY`, and local HTTP values needed to run the API to `.env.example`. Use a development-only key and a root-relative database path for commands run at repository root. Preserve S3 variables and never print secrets.
3. In `main`, load HTTP/database and S3 config, open SQLite, construct storage, and inject both into `NewRouter`.
4. Fail startup with nonzero status if config, repository open, storage construction, or a bounded initial `CheckBucket` fails. Log only safe context, never secrets, bodies, signed URLs, or signed query strings.
5. Close the repository on every path after open, including listen failure and shutdown. Make close failure observable without hiding an earlier primary failure.
6. Preserve server timeouts, signals, shutdown, logs, and exit semantics. Never create buckets, policies, objects, schema, or migrations.

Focused check:

```bash
cd backend
gofmt -w internal/config cmd/api
go test ./internal/config ./cmd/api
```

If `cmd/api` has no tests, compile it instead of inventing subprocess-heavy tests.

# Checkpoint 2 — strict upload-link transport

7. Register `POST /photos/{photoId}/upload-link` with `ServeMux` path values. Validate a canonical UUID before any dependency call.
8. Require `application/json`, allowing valid media-type parameters. Missing, malformed, or other media types return typed 400.
9. Bound the body with `http.MaxBytesReader` and a named limit no larger than 8 KiB. Decode exactly one object with `DisallowUnknownFields`. Typed 400 cases include empty/malformed/oversized bodies, unknown fields, trailing JSON, missing/blank `contentType`, and CR/LF injection. Never echo body contents.
10. Confirm the photo exists before signing. A valid absent ID returns the existing typed 404 with that ID and makes no storage call.
11. Call `PresignUpload` with the exact ID/content type. Return exact `UploadLink` JSON: `url`, constant `method: "PUT"`, all returned signed `headers`, and RFC3339 `expiresAt`; expose no internal fields.
12. Convert storage failures once: caller-invalid input becomes validation; bucket/upstream/internal failures become `apperror.NewInternal` retaining the safe typed cause. Never leak endpoints or signed URLs.

Focused check:

```bash
gofmt -w internal/httpapi
go test ./internal/httpapi -run 'Upload|Router|Auth|CORS|Error'
```

# Checkpoint 3 — list/get and exact DTOs

13. Register authenticated `GET /photos` and `GET /photos/{photoId}`. Missing/wrong keys return the same existing 401 before dependency calls.
14. Accept only `cursor`, `limit`, `classId`, `minConfidence`. Reject unknown parameters and repeated values, including repeated empty values, as typed 400.
15. Bound each query value with named limits before parsing. Allow legitimate opaque cursors but prevent avoidable allocation. An explicitly empty supported parameter is invalid, not absent.
16. Parse `limit` strictly as unsigned base-10 integer `1..200`; omission uses repository default `50`. Reject signs, decimals, whitespace, overflow, zero, and out-of-range values.
17. Validate `classId` against six domain classes. Parse `minConfidence` as finite decimal `[0,1]`; reject NaN, infinities, whitespace, and malformed values. Preserve cursor bytes exactly in `sqlite.ListPhotosParams`.
18. Get validates UUID before calls, uses `GetPhoto`, and preserves repository typed 404 for a valid absent ID.
19. For every returned photo, call `PresignDownload` with its ID and map exact `Photo` JSON: `id`, `x`, `y`, `h`, `width`, `height`, RFC3339 `capturedAt`, matching `originalUrl`, and non-null `predictions`. Prediction fields are exactly `classId`, `confidence`, `bbox`; bbox fields are exactly `xMin`, `yMin`, `xMax`, `yMax`.
20. List returns `{ "items": [...] }`; add `next_token` only when nonempty. Preserve photo/prediction order and encode empty items as `[]`, never `null`.
21. Marshal complete success JSON before committing headers. Data API successes use `Content-Type: application/json` and `Cache-Control: no-store` because responses contain presigned URLs. Do not log bodies or URLs.
22. Route every failure through `WriteError`, preserving request IDs and existing 400/401/404/500 shapes. Do not duplicate internal logging in handlers.
23. Pass request context to every call. Stop on cancellation or first enrichment failure and never emit a partial page.

Focused check:

```bash
gofmt -w internal/httpapi
go test ./internal/httpapi -run 'List|GetPhoto|DTO|Router|Auth|Error'
```

# Required tests

Table-driven deterministic tests must cover:

1. exact success JSON for upload, list, empty/final/non-final pages, and get;
2. `predictions: []`, conditional `next_token`, timestamps, and exact DTO names;
3. public health/preflight versus auth on all Data API routes;
4. all body/content-type failures specified above;
5. malformed/absent/missing photo IDs and no storage call for invalid/missing IDs;
6. every query boundary, unknown/repeated/empty parameters, unknown class, non-finite confidence, cursor preservation;
7. omitted/default limit, accepted `1`/`200`, rejected `0`/`201`;
8. exact repository parameters and request-context propagation;
9. one matching download sign per photo, stable order, and no signing for empty pages;
10. safe repository/storage 500s with no secret, endpoint, cause, or signed-URL leakage;
11. first enrichment failure stops work and emits no partial success;
12. `NewRouter` rejects missing dependencies while old middleware/recovery/logging tests remain valid.

Use fixed clocks/fakes. No sleeps, real credentials, Docker, MinIO, supplied DB, or network ports. Do not loosen existing assertions.

# Out of scope

- Seed client, PUT execution, object-existence probing, integration smoke, or dataset traversal.
- Thumbnails, cache, metrics, frontend, map, production containers, or README.
- OpenAPI changes, generated code, frameworks, migrations, DB writes, bucket creation/policies, public objects, or domain JSON tags.
- `OFFSET`, total counts/caps, later-page preloading, retries, background workers, new dependencies, commits, or pushes.

# Acceptance criteria

- Production startup validates HTTP/database/S3 config, opens SQLite read-only, performs bounded bucket check, serves three authenticated operations, and closes resources.
- Success/error bodies match the named OpenAPI schemas and retain correlation IDs.
- Upload signing only occurs for known IDs; list/get add correct presigned URLs without mutating domain models.
- Bodies, query inputs, page memory, and calls are bounded; catalogs over 200 remain reachable through `next_token`.
- Health/preflight remain public; all Data API operations require `X-API-Key`.
- Offline tests, race detector, vet, build, and diff check pass.

# Verification

After focused checkpoint checks, run once:

```bash
cd backend
gofmt -w cmd/api internal/config internal/httpapi
go test ./...
go test -race ./...
go test -count=10 ./internal/httpapi ./internal/config
go vet ./...
go build -o /tmp/scout-api ./cmd/api
cd ..
git diff --check
git status --short
```

Do not run the server, mutate `dataset/predictions.db`, or require MinIO; external smoke belongs to task 009. Inspect the final diff only for allowed files and report checks that could not run.

# Final report

Report only: changed files, config/startup ownership, route/auth wiring, parsing/DTO decisions, error mapping, verification, and assumptions. Do not begin task 009.
