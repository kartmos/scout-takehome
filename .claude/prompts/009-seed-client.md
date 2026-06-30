# Goal

Implement a small rerunnable seed CLI that uploads `dataset/images/*.jpg` through the public Data API upload-link operation. Keep file handling, concurrency, HTTP bodies, redirects, and output bounded; do not add tests or run integration smoke in this stage.

# Context manifest

Start with `git status --short` and inspect only:

- `CLAUDE.md`;
- README dataset layout and the upload-link Data API row;
- OpenAPI `createUploadLink`, `UploadLinkRequest`, and `UploadLink` only;
- `.env.example`;
- `backend/go.mod`.

Verified facts:

- task 008 serves authenticated `POST /photos/{photoId}/upload-link` with `X-API-Key`;
- upload-link returns a signed PUT URL, method, required headers, and expiry;
- the object key is the exact photo UUID, so uploading the same file again safely overwrites it;
- dataset images are JPEG files named `<canonical-photo-uuid>.jpg`;
- this is an external client and must not import backend `internal/httpapi`, repository, or object-storage implementation packages;
- task 012 owns all backend tests, race checks, ingest-to-read smoke, review, and corrective findings.

Do not read previous prompts, test files, thumbnail/frontend code, the full OpenAPI file, Compose implementation, or SQLite implementation. Do not use subagents or restate the task.

# Scope

Create only the production seed implementation, preferably:

```text
backend/internal/seed/client.go
backend/cmd/seed/main.go
```

Modify `.env.example` only if a required public seed setting is absent. Modify an adjacent production file only for a demonstrated compile blocker. Do not create or modify test, fixture, smoke, script, Compose, or documentation files.

# Requirements

1. Provide a CLI runnable from repository root. Configure API base URL, API key, images directory, concurrency, and request timeout through clear flags or `SCOUT_*` environment variables.
2. Defaults: local API, `dataset/images`, concurrency `2`, and a reasonable named timeout. Concurrency must be positive and capped at `4`.
3. Validate configuration before work. Accept only absolute `http`/`https` API URLs without userinfo, query, fragment, or unexpected path. Never print the API key or signed URLs.
4. Enumerate only regular `.jpg` files in the configured directory, without recursion or symlink following. Reject unreadable/empty directories, unsupported entries, duplicate IDs, and non-canonical UUID filenames before uploading anything.
5. Keep memory bounded: retain only file metadata and stream each image from disk. Never load the full dataset or complete images into memory.
6. Use a fixed worker pool with the configured concurrency. Stop scheduling after cancellation, wait for started workers, and collect their failures. Support SIGINT/SIGTERM through context cancellation.
7. For each image, call `POST /photos/{id}/upload-link` with `X-API-Key`, `Content-Type: application/json`, and `{"contentType":"image/jpeg"}`. Bound response bodies, require 200, and decode the documented fields.
8. Validate returned method `PUT`, future expiry, safe required header names/values, and an absolute `http`/`https` signed URL without userinfo. Do not log its query string.
9. Upload by streaming the file with known `Content-Length` and exactly the required signed headers. Never send `X-API-Key` to object storage.
10. Disable redirects for API and PUT clients so API credentials and signatures cannot move to another host.
11. Treat API and non-2xx PUT failures as failed items. Bound and sanitize error excerpts; never expose response bodies, credentials, API keys, or URLs in output.
12. Reruns overwrite the same UUID keys safely. Print a compact final summary (`discovered/succeeded/failed`) and return nonzero on validation error, cancellation, or any failed upload.
13. Keep package boundaries small: CLI owns environment/flags/signals/output; reusable seed logic owns discovery, worker scheduling, HTTP calls, and streaming.
14. Use only the Go standard library unless a production requirement is impossible without another dependency.

# Out of scope

- Any unit, integration, smoke, race, repeated, fixture, or generated test code.
- Running `go test`, `go test -race`, `go vet`, Docker, MinIO, API server, seed uploads, or external smoke.
- Thumbnail work, metrics, frontend, database writes, bucket creation/policy, multipart/resumable upload, retries/backoff, or catalog-wide verification.
- Changes to Data API contracts, commits, pushes, or unrelated cleanup.

# Acceptance criteria

- Production code contains a bounded seed CLI capable of uploading valid dataset JPEGs through the public API with concurrency `2` and safe reruns.
- Files are streamed, concurrency and responses are bounded, redirects are disabled, and secrets/signed URLs are never printed.
- No tests or smoke code are added or run in this stage.
- The production seed target formats and compiles.

# Verification

Do not create or run tests. Perform only code hygiene:

```bash
cd backend
gofmt -w cmd/seed internal/seed
go build -o /tmp/scout-seed ./cmd/seed
cd ..
git diff --check
git status --short
```

Inspect only the scoped production diff. Do not run the seed client.

# Final report

Report only changed production files, CLI/config contract, safety/concurrency decisions, build result, and assumptions. Explicitly confirm that tests and smoke were deferred to task 012. Do not start task 010.
