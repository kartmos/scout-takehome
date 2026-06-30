# Goal

Create the minimal Go backend foundation for Scout: a buildable HTTP service with environment-based configuration, a health endpoint, production-safe server timeouts, structured startup/shutdown logging, and graceful shutdown.

# Context

Before editing, read:

- `CLAUDE.md`
- `README.md`
- `openapi.yaml`
- `.claude/prompts/001-dataset-audit.md`
- the dataset audit result supplied by the user, if it is available in the current conversation
- the current repository tree and `git status --short`

Important verified dataset facts for later tasks:

- `dataset/predictions.db` is valid and must remain unchanged;
- it contains 50 photos and 92 predictions;
- all 50 image filenames correspond one-to-one with photo IDs;
- this task must not access the database or image files.

Before editing, briefly restate the goal, scope, acceptance criteria, and expected changed files.

# Prerequisite

Go 1.26.4 is already installed. Confirm it with `go version`, then continue. Do not install or upgrade any system technology in this task.

# Scope

Create only the initial backend files under `backend/`:

```text
backend/
├── cmd/api/main.go
├── internal/config/config.go
├── internal/config/config_test.go
├── internal/httpapi/router.go
├── internal/httpapi/router_test.go
└── go.mod
```

`go.sum` may be created only if the Go toolchain needs it. Prefer the Go standard library exclusively, so it will normally not be needed yet.

# Requirements

## Go module

1. Initialize `backend/` as a Go module named `scout`.
2. Declare Go 1.26 in `go.mod`.
3. Use the Go standard library only.
4. Keep all non-entrypoint packages under `backend/internal/`.

## Configuration

5. Implement a small typed configuration struct loaded from environment variables.
6. Support these variables and defaults:
   - `SCOUT_HTTP_ADDR`, default `:8080`;
   - `SCOUT_HTTP_READ_HEADER_TIMEOUT`, default `5s`;
   - `SCOUT_HTTP_READ_TIMEOUT`, default `15s`;
   - `SCOUT_HTTP_WRITE_TIMEOUT`, default `30s`;
   - `SCOUT_HTTP_IDLE_TIMEOUT`, default `60s`;
   - `SCOUT_SHUTDOWN_TIMEOUT`, default `10s`.
7. Define defaults as named constants. Do not scatter magic numbers.
8. Parse durations with `time.ParseDuration`.
9. Reject empty `SCOUT_HTTP_ADDR`, invalid durations, zero durations, and negative durations with descriptive errors that identify the variable.
10. Do not call `os.Exit`, `log.Fatal`, or panic inside the config package.

## HTTP router and health endpoint

11. Use `http.ServeMux`; do not add a third-party router.
12. Implement:

```http
GET /healthz
```

13. A successful response must be:
   - status `200 OK`;
   - `Content-Type: application/json`;
   - body `{"status":"ok"}` followed by a newline.
14. Non-GET requests to `/healthz` must return `405 Method Not Allowed` and include `Allow: GET`.
15. Unknown routes must return `404 Not Found`.
16. Keep route construction in `internal/httpapi`, separate from `main.go`.
17. Do not implement OpenAPI error bodies for `/healthz`; typed API error handling belongs to a later task.

## Application lifecycle

18. `cmd/api/main.go` must:
   - load and validate configuration;
   - create a JSON `slog` logger writing to stdout;
   - construct `http.Server` with all configured timeouts;
   - start the server;
   - handle `SIGINT` and `SIGTERM` using `signal.NotifyContext`;
   - call `Server.Shutdown` with the configured timeout;
   - distinguish `http.ErrServerClosed` from a real server failure;
   - return a non-zero process exit code for configuration, startup, or shutdown failures.
19. Keep executable exit handling at the boundary: prefer a `run() error` or `run() int` pattern so lifecycle logic remains understandable and testable.
20. Log concise structured events for server start, shutdown start, shutdown completion, and fatal failures.
21. Never log the full environment.

## Code quality and tests

22. Add table-driven config tests covering defaults, overrides, malformed duration, zero duration, negative duration, and empty address.
23. Add router tests covering successful health response, content type, body, method rejection, `Allow` header, and unknown route.
24. Use idiomatic error wrapping with `%w` where appropriate.
25. Keep the implementation simple and suitable for extension in later tasks.

# Out of scope

- Reading or modifying `dataset/predictions.db`.
- Adding SQLite or repository code.
- Adding MinIO, S3, Docker, or seed logic.
- Implementing `/photos`, `/photos/{photoId}`, upload links, or thumbnails.
- API-key authentication, CORS, correlation IDs, recovery middleware, typed API errors, metrics, or request logging.
- Frontend files.
- Dockerfiles, Compose files, Makefiles, task runners, CI, or deployment configuration.
- Adding third-party production or test dependencies.
- Changing `README.md`, `openapi.yaml`, `CLAUDE.md`, the dataset, or existing prompt files.
- Committing or pushing changes.

# Acceptance criteria

- The backend starts with `go run ./cmd/api` from `backend/`.
- `GET http://localhost:8080/healthz` returns exactly the documented JSON response.
- `POST /healthz` returns 405 with `Allow: GET`.
- An unknown route returns 404.
- Invalid configuration prevents startup and produces a useful error without a stack trace.
- `SIGINT` and `SIGTERM` trigger bounded graceful shutdown.
- Server timeouts are populated from validated configuration.
- All tests pass without network services or dataset access.
- Only files allowed by this prompt are changed.
- The implementation uses no third-party dependencies and contains no placeholder TODOs.

# Verification

From `backend/`, run:

```bash
gofmt -w cmd internal
go mod tidy
go test ./...
go vet ./...
go build ./cmd/api
```

Then perform a brief manual smoke check:

```bash
go run ./cmd/api
curl -i http://localhost:8080/healthz
curl -i -X POST http://localhost:8080/healthz
curl -i http://localhost:8080/does-not-exist
```

Stop the process with `Ctrl+C` and confirm the logs show graceful shutdown.

Finally, from the repository root, run:

```bash
git diff --check
git status --short
```

Inspect the complete diff before reporting completion.

# Final report

Report:

1. files created;
2. configuration variables and defaults;
3. verification commands and their results;
4. manual health/shutdown check results;
5. assumptions or checks that could not be completed.

Do not proceed to SQLite, MinIO, or any later implementation task.
