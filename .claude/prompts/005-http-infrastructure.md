# Goal

Build the reusable HTTP infrastructure for Scout: correlation IDs, centralized OpenAPI-compatible error responses, API-key authentication middleware, panic recovery, structured access logs, controlled CORS, and bounded server headers. Keep `/healthz` public and do not add photo business endpoints yet.

# Context

Before editing, read:

- `CLAUDE.md`
- `README.md`
- `openapi.yaml`
- `.claude/prompts/ROADMAP.md`
- `.claude/prompts/002-backend-bootstrap.md`
- `.claude/prompts/003-domain-models-and-errors.md`
- `.claude/prompts/004-sqlite-repository.md`
- `.claude/prompts/004.1-sqlite-repository-hardening.md`
- all current files under `backend/`
- the current diff and `git status --short`

The backend currently has a public `GET /healthz`, typed `apperror.AppError`, JSON `slog`, safe server timeouts, and a read-only SQLite repository. Photo/upload/thumbnail routes do not exist yet.

Before editing, briefly restate the goal, middleware order, error mapping, scope, acceptance criteria, and expected changed files.

# System technology

No new system technology or third-party Go dependency is introduced. Use the Go 1.26 standard library only. Do not install or upgrade anything.

# Scope

Create HTTP infrastructure files under `backend/internal/httpapi/`, for example:

```text
backend/internal/httpapi/errors.go
backend/internal/httpapi/errors_test.go
backend/internal/httpapi/middleware.go
backend/internal/httpapi/middleware_test.go
backend/internal/httpapi/requestid.go
```

Modify only as required:

```text
backend/internal/config/config.go
backend/internal/config/config_test.go
backend/internal/httpapi/router.go
backend/internal/httpapi/router_test.go
backend/cmd/api/main.go
```

Additional focused test files inside these existing packages are allowed. Do not modify domain, application-error, repository, dataset, frontend, or OpenAPI files.

# Requirements

## Configuration

1. Add a required `SCOUT_API_KEY` configuration value.
2. Reject a missing, empty, or whitespace-only API key. Do not provide an insecure production default.
3. Never include the API key in an error, log, config dump, or test failure output.
4. Add `SCOUT_CORS_ALLOWED_ORIGINS` as a comma-separated allowlist with a development default of `http://localhost:5173`.
5. Trim entries, reject malformed origins, remove duplicates, and avoid exposing mutable shared backing storage.
6. Allowed origins must be exact `http` or `https` origins with scheme and host only: no credentials, path other than `/`, query, or fragment.
7. Add `SCOUT_HTTP_MAX_HEADER_BYTES` with a named small-box-safe default of `65536` bytes.
8. Parse it as a positive integer and reject zero, negative, malformed, or values above `1048576`.
9. Populate `http.Server.MaxHeaderBytes` in `cmd/api/main.go`.
10. Do not add generic request-body middleware yet. Body limits will be applied by the first body-consuming handler so errors retain the OpenAPI shape.
11. Update config tests without weakening existing timeout/address tests.

## Correlation/request ID

12. Use the response header `X-Request-ID`.
13. Install request-ID middleware before logging, recovery, authentication, and error handling need the ID.
14. Accept an incoming ID only when it is 1–128 characters and contains ASCII letters, digits, `.`, `_`, or `-`.
15. Generate a cryptographically random standard-library ID when the incoming value is missing or invalid. Do not add a UUID dependency.
16. Store the ID in context through an unexported typed key and provide a small accessor.
17. Return the same ID in the response header.
18. Invalid incoming IDs are replaced, not echoed or returned as client errors.

## Centralized OpenAPI error writer

19. Implement one centralized error writer mapping errors to HTTP status and JSON.
20. Map `apperror` kinds exactly:
    - validation → `400`, code `ValidationError`, required `details`;
    - authentication → `401`, code `AuthenticationRequired`;
    - not found → `404`, code `NotFound`, required `resource_id`;
    - internal → `500`, code `InternalServerError`.
21. Unknown errors map to the same safe 500 response.
22. Every body contains the request ID and safe public message.
23. Match `openapi.yaml` field names exactly, including snake_case.
24. Set `Content-Type: application/json` and `Cache-Control: no-store` before status.
25. Marshal the full response before `WriteHeader`.
26. Never expose wrapped causes, stack traces, API keys, headers, or panic values to clients.
27. Log internal/unknown failures once with request ID and retained cause. Do not log client-safe errors as server failures in the writer.
28. Generate a fallback request ID if the writer is called without middleware.

## API-key authentication

29. Implement reusable middleware for protected Data API handlers.
30. Read only `X-API-Key` as defined by OpenAPI.
31. Missing and invalid keys use the centralized writer and return typed 401 responses.
32. Compare keys with `crypto/subtle` constant-time comparison after length-safe preparation.
33. Do not distinguish missing from incorrect keys publicly and never log either key.
34. Keep `/healthz` public.
35. Do not globally protect future thumbnail delivery or `/metrics`; protection will be explicit for the three Data API operations.
36. Test auth middleware with local handlers. Do not add a fake production route.

## Panic recovery

37. Recover downstream panics and emit a typed safe 500 if headers are uncommitted.
38. Log request ID and panic at error level without exposing the value to clients.
39. Re-panic `http.ErrAbortHandler` so net/http keeps its intended behavior.
40. If a panic occurs after output has begun, do not append JSON to a partial body; log and abort safely.
41. Test panic before write, after write, and `http.ErrAbortHandler`.

## Structured access logging

42. Log one completion event per request with request ID, method, route pattern when available (otherwise path without query), status, response bytes, and duration.
43. Never log raw query strings, bodies, API keys, Authorization/Cookie headers, presigned URLs, or all headers.
44. Use info for normal responses, warn for 4xx, and error for 5xx.
45. The response recorder must expose `Unwrap() http.ResponseWriter` and preserve behavior required by later streaming endpoints. Test `http.Flusher` access through `http.NewResponseController`.
46. Do not add metrics in this task.

## CORS

47. Allow only exactly configured origins; never reflect arbitrary origins.
48. For allowed origins set exact `Access-Control-Allow-Origin`, append `Vary: Origin`, and expose `X-Request-ID`.
49. Handle valid preflight before authentication with `204 No Content`.
50. Preflight allows `GET`, `POST`, `OPTIONS` and headers `Content-Type`, `X-API-Key`, `X-Request-ID`.
51. Add correct preflight `Vary` values.
52. Do not set `Access-Control-Allow-Credentials`.
53. Disallowed origins receive no allow headers and continue through normal routing/auth.
54. Do not add backend `PUT` CORS: uploads go directly to MinIO, configured later.

## Router and middleware order

55. Make router construction accept explicit dependencies/options such as logger, origins, and API key; do not use package globals.
56. Validate required dependencies or make invalid states impossible.
57. Order middleware so request ID exists first, CORS preflight precedes auth, access logging records recovered failures, and recovery uses the centralized writer.
58. Keep the production route set unchanged: only `GET /healthz` exists after this task.
59. Preserve health JSON, method/404 behavior, startup logging, timeouts, and graceful shutdown.

## Tests

60. Add table-driven config tests for API key, origins, and max header bytes.
61. Test accepted, replaced, bounded, and echoed request IDs.
62. Test every application-error mapping and an unknown error: exact status, headers, code, request ID, required fields, and no cause leakage.
63. Test missing/wrong/correct API key and ensure secrets never appear in response or logs.
64. Test recovery and access-log status/level/fields.
65. Test simple and preflight CORS for allowed/disallowed origins, `Vary`, methods/headers, and absent credentials.
66. Test `/healthz` without API key and with returned request ID.
67. Test logs do not contain an API key or raw query secret.
68. Keep tests deterministic with no SQLite, MinIO, Docker, or listener.

# Out of scope

- Photo, upload-link, thumbnail, metrics, readiness, debug, or fake production routes.
- Opening or wiring SQLite in `main`.
- MinIO/S3, Docker, seed, or frontend work.
- Body parsing and body-limit error mapping.
- Editing OpenAPI, domain, application-error, repository, dataset, or previous prompts.
- Third-party dependencies, commits, or pushes.

# Acceptance criteria

- Future errors can use one OpenAPI-compatible writer with matching request ID.
- `/healthz` remains public with its exact success body.
- API-key middleware is reusable without an unrequested production route.
- Request IDs are safe, bounded, returned, and present in logs/error bodies.
- Recovery emits a safe typed 500 and access logging records it.
- CORS is allowlist-based and preflight requires no API key.
- Logs contain useful metadata but no secrets, raw query, headers, or bodies.
- `MaxHeaderBytes` is bounded for the small-box runtime.
- Existing domain/repository behavior remains passing.
- No new dependency or build artifact is introduced.

# Verification

From `backend/`, run:

```bash
gofmt -w cmd/api internal/config internal/httpapi
go mod tidy
go test ./...
go test -race ./...
go test -count=10 ./internal/httpapi ./internal/config
go vet ./...
go build -o /tmp/scout-api ./cmd/api
```

Manual smoke:

```bash
SCOUT_API_KEY=local-development-key go run ./cmd/api
curl -i http://localhost:8080/healthz
curl -i -H 'X-Request-ID: smoke-test-1' http://localhost:8080/healthz
```

Confirm health requires no API key, returns `X-Request-ID`, and shutdown is graceful. Then from repository root:

```bash
git diff --check
git status --short
```

Inspect the complete diff before reporting completion.

# Final report

Report:

1. files changed;
2. config variables and validation;
3. middleware order and route policy;
4. error mapping and request-ID behavior;
5. verification/manual smoke results;
6. assumptions or checks not completed.

Do not proceed to MinIO, storage, photo handlers, thumbnails, or metrics.
