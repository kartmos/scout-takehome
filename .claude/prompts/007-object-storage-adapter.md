# Goal

Add a tested S3-compatible original-photo storage adapter backed by the official MinIO Go SDK. It must create signed PUT/GET URLs, open original objects as cancellable streams, validate configuration and keys, and expose safe typed storage errors. Do not add HTTP handlers or wire it into `main` yet.

# Context manifest

Read only:

- `CLAUDE.md`;
- `.env.example`;
- `backend/go.mod`;
- `backend/internal/config/config.go` and `config_test.go`;
- `backend/internal/apperror/error.go` only to understand existing error boundaries;
- the UUID validator and `Photo` comment in `backend/internal/domain/photo.go`;
- OpenAPI operation `createUploadLink` and schemas `UploadLinkRequest`, `UploadLink`, and `Photo` only;
- root status and a diff scoped to files listed below.

Verified facts:

- task 006 provides healthy local MinIO and private bucket `scout-photos`;
- `.env.example` already defines endpoint as `host:port`, credentials, bucket and secure flag;
- an original object key is exactly its canonical photo UUID, with no extension or prefix;
- `domain.Photo` intentionally does not contain `OriginalURL`;
- task 008 will own HTTP/auth/request validation; task 010 will enrich photo DTOs; task 012 will consume original streams.

Do not read previous prompts, unrelated backend packages, the full OpenAPI file, README, dataset, or Compose file. Do not use subagents.

# Dependency

Add the official Apache-2.0 MinIO/S3 client as a pinned direct dependency:

```text
github.com/minio/minio-go/v7 v7.2.1
```

Official API reference: <https://pkg.go.dev/github.com/minio/minio-go/v7>. No system package is required.

# Scope

Create a focused package, preferably:

```text
backend/internal/objectstorage/storage.go
backend/internal/objectstorage/minio.go
backend/internal/objectstorage/errors.go
backend/internal/objectstorage/storage_test.go
backend/internal/objectstorage/integration_test.go
```

Modify only:

```text
backend/internal/config/config.go
backend/internal/config/config_test.go
backend/go.mod
backend/go.sum
.env.example
```

An equivalent small file split inside `internal/objectstorage` is acceptable.

# Requirements

## Configuration

1. Load and validate `SCOUT_S3_ENDPOINT`, `SCOUT_S3_ACCESS_KEY`, `SCOUT_S3_SECRET_KEY`, `SCOUT_S3_BUCKET`, and `SCOUT_S3_SECURE`.
2. The endpoint is SDK form `host[:port]`: reject scheme, credentials, path, query, fragment, whitespace-only, and malformed host/port values.
3. Require nonblank access/secret values without ever including them in errors or test failure output.
4. Validate a DNS-compatible bucket name (3–63 characters, lowercase letters/digits/dot/hyphen, alphanumeric ends, no adjacent dots, not an IP address).
5. Parse `SCOUT_S3_SECURE` strictly as boolean. Do not silently downgrade malformed values to HTTP.
6. Add optional `SCOUT_S3_REGION`, default `us-east-1`.
7. Add upload/download URL TTLs with named defaults of 15 minutes. Parse positive durations and reject values outside the SDK’s 1-second to 7-day signing range.
8. Add these variables and development defaults to `.env.example`; keep credentials clearly development-only.
9. Extend table-driven config tests without weakening existing HTTP/config coverage.

## Adapter contract

10. Define a small original-storage interface usable by later handlers and thumbnail generation. It must support:
    - presigning upload for photo ID + content type;
    - presigning download for photo ID;
    - opening the original as `io.ReadCloser`;
    - checking bucket availability without creating or changing it.
11. Upload result must carry URL, required signed headers (at least exact `Content-Type`), and expiry time for the future `UploadLink` response.
12. Use `PresignHeader` (or an equivalent SDK API) so the provided nonblank content type is part of the PUT signature. Reject CR/LF or otherwise unsafe header values.
13. Download signing needs no public bucket policy. Do not expose credentials except inside the signed URL returned to the caller.
14. Use configured TTLs internally; callers must not choose arbitrary signing lifetimes.
15. Derive the object key only from a canonical UUID and use the exact UUID as the key. Reject malformed IDs before SDK calls; never accept paths, prefixes, slashes, `..`, or caller-selected bucket names.
16. Construct the SDK client with static V4 credentials, region and secure flag. Construction must not create buckets, policies, objects, or background goroutines.
17. Keep the concrete SDK client private. Use a minimal internal seam/interface so unit tests need no Docker or network.

## Streaming and errors

18. `OpenOriginal` must honor context cancellation and return a stream the caller owns and must close. Never read the whole image into memory.
19. Account for MinIO `GetObject` being lazy: do not report success for a missing/unreachable object before the first network result is known. Close any SDK object on validation/stat failure.
20. Wrap read-time failures as well as open/stat failures without buffering the stream. Preserve `io.EOF`, context cancellation/deadline matching, and safe close semantics.
21. Define a typed storage error with operation and category such as invalid input, object not found, bucket unavailable, and internal/upstream failure. Preserve the underlying cause for `errors.Is/As`.
22. Recognize standard S3/MinIO not-found response codes without string-matching full error messages. Do not classify a missing configured bucket as a missing photo object.
23. `Error()` must never reveal credentials, signed query strings, complete presigned URLs, endpoint userinfo, or response bodies. Do not log inside the adapter.

## Tests

24. Unit-test config defaults/overrides and malformed endpoint, boolean, bucket, TTL, missing credentials, and secret non-leakage.
25. Unit-test exact key derivation, signed PUT headers, configured TTL use, GET signing, bucket check, context propagation, not-found classification, safe error text, stream close, lazy/read errors, and no SDK call after invalid input.
26. Use deterministic seams for clock and SDK behavior where needed; do not use sleeps.
27. Add a build-tagged integration test (`integration`) that uses the existing `SCOUT_S3_*` environment, never hardcoded credentials. Against task-006 MinIO it must:
    - check the configured bucket;
    - upload unique JPEG-like bytes through the presigned PUT with required headers;
    - read them through both presigned GET and `OpenOriginal`;
    - verify bytes and clean up its unique object even on failure.
28. The ordinary `go test ./...` suite must skip integration cleanly and require neither Docker nor network.

# Out of scope

- HTTP routes/DTOs, auth wiring, repository wiring, seed client, image validation/decoding, thumbnails, cache, metrics, frontend, bucket creation or policies.
- Modifying Compose, README, OpenAPI, domain models, application-error kinds, or `cmd/api/main.go`.
- Logging credentials or signed URLs; commits or pushes.

# Acceptance criteria

- Later code can presign exact-key PUT/GET operations and stream originals through one small interface.
- Configuration is strict and safe for local MinIO or external S3-compatible storage.
- Missing objects and infrastructure failures are distinguishable through typed safe errors.
- Streams are bounded, cancellable, closable, and never eagerly buffered.
- Unit tests are offline; the tagged integration test passes against task-006 MinIO.
- Existing backend tests remain passing and no build artifact remains.

# Verification

During implementation, run focused package/config tests. After stable code, run once:

```bash
cd backend
gofmt -w internal/config internal/objectstorage
go mod tidy
go test ./...
go test -race ./...
go test -count=10 ./internal/objectstorage ./internal/config
go vet ./...
go build -o /tmp/scout-api ./cmd/api
cd ..
docker compose --env-file .env.example up -d
cd backend
set -a && . ../.env.example && set +a
go test -tags=integration ./internal/objectstorage
cd ..
docker compose --env-file .env.example down
git diff --check
git status --short
```

Use a cleanup trap or equivalent so MinIO is stopped and the integration object is removed if the tagged test fails. Do not delete the named volume.

# Final report

Report only: changed files/dependency, config contract, adapter API/key strategy, error/stream behavior, unit/integration verification, and checks not run. Do not proceed to task 008.
