# Goal

Add the core Scout domain models, domain validation helpers, and typed application errors that later repository and HTTP tasks can reuse without coupling the domain to HTTP or storage.

# Context

Before editing, read:

- `CLAUDE.md`
- `README.md`
- `openapi.yaml`
- `.claude/prompts/001-dataset-audit.md`
- `.claude/prompts/002-backend-bootstrap.md`
- all existing files under `backend/`
- the current repository diff and `git status --short`

Verified dataset facts relevant to this task:

- there are six documented prediction classes and all six occur in the dataset;
- confidence and bbox coordinates are normalized to `[0,1]`;
- valid dataset bboxes include exact boundary values such as `xMin = 0` and `yMax = 1`;
- `x` and `y` are positions on a fixed 40 by 40 metre plane;
- `h` is non-negative camera height;
- photo IDs are UUID strings;
- `captured_at` values are RFC 3339 timestamps;
- originals will later live in object storage, so `originalUrl` is not persisted domain metadata.

Before editing, briefly restate the goal, scope, acceptance criteria, and expected changed files.

# System technology

No new system technology is introduced in this task. Go 1.26.4 is already installed. Do not install or upgrade anything.

# Scope

Create only:

```text
backend/internal/domain/photo.go
backend/internal/domain/photo_test.go
backend/internal/apperror/error.go
backend/internal/apperror/error_test.go
```

`backend/go.mod` may change only if required by `go mod tidy`; no third-party dependency is expected or allowed.

# Requirements

## Domain models

1. In package `domain`, define a named string type `ClassID`.
2. Define constants for exactly these known values:
   - `powdery_mildew`
   - `mirid`
   - `whitefly_aphid`
   - `miner_tuta`
   - `thrips`
   - `spider_mites`
3. Provide a helper that reports whether a `ClassID` is known.
4. If a helper returns all known class IDs, it must not expose mutable shared backing storage.
5. Define `BoundingBox` with `XMin`, `YMin`, `XMax`, and `YMax` fields using `float64`.
6. Define `Prediction` with:
   - `ClassID`;
   - `Confidence` as `float64`;
   - `BoundingBox`.
7. Define `Photo` with persisted/domain data only:
   - `ID` as string;
   - `X`, `Y`, and `H` as `float64`;
   - `Width` and `Height` as integers;
   - `CapturedAt` as `time.Time`;
   - `Predictions` as a slice of `Prediction`.
8. Do not put `OriginalURL` in `domain.Photo`. It is derived later from object storage and belongs in the API response model or service result.
9. Do not expose the prediction database primary key in the public domain model because it is absent from the OpenAPI `Prediction` schema.
10. Define a small page/result model only if it is immediately useful to the upcoming repository task. Do not add speculative generic pagination abstractions.

## Domain validation

11. Define the greenhouse size as a named constant of 40 metres.
12. Implement validation helpers or methods for:
   - known class IDs;
   - confidence in inclusive range `[0,1]`;
   - every bbox coordinate in inclusive range `[0,1]`;
   - strict ordering `XMin < XMax` and `YMin < YMax`;
   - photo UUID shape;
   - `X` and `Y` in inclusive range `[0,40]`;
   - non-negative `H`;
   - positive width and height;
   - non-zero `CapturedAt`.
13. Reject `NaN` and positive or negative infinity for all floating-point domain values.
14. Exact bbox boundaries `0` and `1` are valid and must not be clamped or rejected.
15. Validation must not silently normalize, mutate, round, or clamp values.
16. UUID validation must use the Go standard library only. Accept canonical hyphenated UUID text case-insensitively and reject malformed length, separator placement, or non-hex characters. Do not require a particular UUID version.
17. Validation errors must identify the invalid field and issue clearly enough for later conversion into OpenAPI validation details.
18. Keep domain validation independent of JSON, HTTP status codes, environment variables, SQLite, and MinIO.

## Typed application errors

19. In package `apperror`, define a closed set of error kinds corresponding to later API behavior:
   - validation;
   - authentication required;
   - not found;
   - internal.
20. Define a `FieldViolation` value containing `Field` and `Issue` strings.
21. Define a typed application error that can carry, where applicable:
   - kind;
   - safe public message;
   - validation violations;
   - not-found resource ID;
   - wrapped internal cause.
22. Provide small explicit constructors for validation, authentication, not-found, and internal errors.
23. Constructors must defensively copy caller-owned violation slices.
24. The error must support `errors.As`; internal errors must support `errors.Is` through `Unwrap`.
25. `Error()` must return only the safe public message. It must never append or expose a wrapped internal cause.
26. Internal errors must use a stable generic public message rather than the cause text.
27. Reject or safely normalize invalid constructor input such as an empty public message, missing resource ID, empty validation details, or a nil internal cause. Choose one simple consistent policy and test it; do not panic.
28. Do not add `request_id` to application errors. Request IDs are request-scoped HTTP transport data and will be attached by a later centralized error writer.
29. Do not add HTTP status codes or OpenAPI JSON response structs to `apperror` in this task.

## Tests

30. Add table-driven tests for every known and unknown class ID.
31. Test valid bbox cases, including exact `0` and `1` boundaries.
32. Test each invalid bbox coordinate, reversed/equal corners, `NaN`, and infinities.
33. Test confidence boundaries, out-of-range values, `NaN`, and infinities.
34. Test valid upper/lower greenhouse boundaries and every invalid photo metadata condition.
35. Test valid upper- and lowercase canonical UUIDs plus malformed UUID cases.
36. Test every application error constructor, safe `Error()` output, kind, details/resource ID, defensive slice copying, `errors.As`, `errors.Is`, and non-leakage of internal cause text.

# Out of scope

- Changing the existing health route, server lifecycle, or configuration behavior.
- HTTP response DTOs, JSON tags, status mapping, error writer, middleware, or handlers.
- SQLite drivers, SQL queries, repositories, schema changes, migrations, or indexes.
- MinIO, S3, upload links, seed logic, thumbnails, metrics, or frontend code.
- Adding `OriginalURL` to persisted domain data.
- Modifying `README.md`, `openapi.yaml`, `CLAUDE.md`, dataset files, or existing prompt files.
- Adding third-party dependencies.
- Committing or pushing changes.

# Acceptance criteria

- Domain types express the supplied photo and prediction data without transport or storage coupling.
- All six class IDs are represented exactly as documented.
- Validation accepts inclusive boundaries required by the contract and rejects non-finite values.
- UUID validation is deterministic and standard-library-only.
- Application errors distinguish the four required categories without containing HTTP concerns.
- Internal cause text cannot leak through `Error()`.
- Validation detail slices cannot be mutated through the original constructor input.
- Existing bootstrap behavior and tests remain unchanged and passing.
- Only files allowed by this prompt are changed.
- No new dependency is added and no placeholder TODO remains.

# Verification

From `backend/`, run:

```bash
gofmt -w internal/domain internal/apperror
go mod tidy
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/api
```

Then, from the repository root, run:

```bash
git diff --check
git status --short
```

Inspect the complete diff before reporting completion.

# Final report

Report:

1. files created or changed;
2. domain types and validation rules added;
3. application error kinds and safe-message behavior;
4. verification commands and results;
5. assumptions or checks that could not be completed.

Do not proceed to HTTP error mapping, SQLite, MinIO, or any later task.
