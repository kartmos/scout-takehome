# Goal

Implement a read-only SQLite repository for Scout photos with deterministic cursor pagination, correct same-prediction filter semantics, batched prediction loading, and typed error behavior.

# Context

Before editing, read:

- `CLAUDE.md`
- `README.md`
- `openapi.yaml`
- `.claude/prompts/001-dataset-audit.md`
- `.claude/prompts/002-backend-bootstrap.md`
- `.claude/prompts/003-domain-models-and-errors.md`
- all existing files under `backend/`
- the actual schema of `dataset/predictions.db`
- the current repository diff and `git status --short`

Relevant verified facts:

- `dataset/predictions.db` passes SQLite integrity and foreign-key checks;
- the supplied database must remain unchanged;
- `photos.captured_at` is RFC 3339 text;
- all photo IDs are canonical UUID strings;
- every photo currently has predictions, but the repository must also support photos with none;
- the existing index is `idx_pred_photo` on `predictions(photo_id)`;
- do not add indexes, migrations, tables, or other changes to the supplied database;
- photo filters must be satisfied by the same prediction while returned photos must include all their predictions;
- `domain.Photo` intentionally does not contain `OriginalURL` because storage enrichment comes later.

Before editing, briefly restate the goal, scope, acceptance criteria, expected changed files, and the SQL pagination/filter strategy.

# Dependency introduced in this task

This task introduces the pure-Go SQLite driver:

```text
modernc.org/sqlite
```

Add it as a pinned Go module dependency and commit the resulting `go.mod` and `go.sum` changes. It must not require CGO, GCC, or any system SQLite development package.

No new system technology is introduced. Do not install or upgrade system packages.

# Scope

Create:

```text
backend/internal/repository/sqlite/repository.go
backend/internal/repository/sqlite/cursor.go
backend/internal/repository/sqlite/repository_test.go
backend/internal/repository/sqlite/cursor_test.go
```

Modify only as required:

```text
backend/go.mod
backend/go.sum
```

Additional test helper files inside `backend/internal/repository/sqlite/` are allowed when they materially improve readability.

# Requirements

## Repository API and lifecycle

1. Implement a concrete read-only repository backed by `database/sql` and `modernc.org/sqlite`.
2. Provide a constructor/open function accepting a database file path.
3. Resolve and validate the path, open SQLite in read-only mode, ping it, and return a descriptive wrapped error on failure.
4. Configure a small named maximum connection count appropriate for one small box. Do not leave an unexplained magic number.
5. Provide `Close()` and make ownership of the database handle clear.
6. Every query method must accept `context.Context` and use context-aware database calls.
7. Do not expose `*sql.DB` publicly.
8. Do not execute migrations, DDL, DML, `VACUUM`, index creation, or write-oriented PRAGMAs against the supplied database.
9. Do not copy or replace `dataset/predictions.db` in production code.

## Operations

10. Implement:
    - `PhotoExists(ctx, photoID)`;
    - `GetPhoto(ctx, photoID)`;
    - `ListPhotos(ctx, params)` returning `domain.PhotoPage`.
11. Define a focused `ListPhotosParams` containing:
    - opaque cursor string;
    - limit;
    - optional class ID;
    - optional minimum confidence.
12. Use named constants matching OpenAPI pagination rules:
    - default limit `50` when the caller supplies zero;
    - minimum explicit limit `1`;
    - maximum limit `200`.
13. Reject negative or over-maximum limits as validation errors.
14. Validate photo IDs, class IDs, minimum confidence, and cursor input using existing domain/application error facilities.
15. `GetPhoto` must return a typed not-found application error for a valid UUID absent from the database.
16. Invalid input must return a typed validation application error.
17. Database, scanning, timestamp parsing, and invalid persisted-data failures must retain their cause through a typed internal application error without leaking it in `Error()`.

## Row mapping and validation

18. Map `photos` rows into `domain.Photo` and `predictions` rows into `domain.Prediction`.
19. Parse `captured_at` as RFC 3339/RFC 3339 Nano without silently accepting malformed timestamps.
20. Validate mapped photo metadata and predictions before returning them.
21. A malformed row in the database must produce an internal error, not a client validation error.
22. `GetPhoto` must return every prediction for the photo in deterministic order.
23. A photo with no predictions must return a non-nil empty prediction slice where practical, rather than JSON-relevant behavior leaking into the repository.

## List query and filter semantics

24. Sort photos deterministically by:

```sql
captured_at DESC, id DESC
```

25. Implement keyset pagination, not `OFFSET` pagination.
26. Fetch `limit + 1` photo rows to determine whether another page exists.
27. The cursor boundary must use both `captured_at` and `id`:

```sql
captured_at < cursorCapturedAt
OR (captured_at = cursorCapturedAt AND id < cursorID)
```

28. Both optional filters must be applied inside one correlated `EXISTS` subquery over a single prediction alias. Never use separate `EXISTS` clauses that could match different predictions.
29. Filter behavior must be:
    - no filters: all photos;
    - class only: at least one prediction of that class;
    - confidence only: at least one prediction with `confidence >= minConfidence`;
    - both: at least one same prediction satisfying both conditions.
30. Confidence comparison is inclusive.
31. After selecting a page of photos, load all predictions for those photo IDs in one batched query. Do not issue one prediction query per photo.
32. Return all predictions for each matched photo, not only predictions satisfying the filters.
33. Build SQL from a small fixed set of trusted clauses and bind all values as parameters. Never concatenate user-provided values into SQL.
34. Fully close and check all `Rows` objects before starting a dependent query or returning.

## Cursor format

35. Implement an opaque, versioned cursor using unpadded base64url around a small JSON payload containing the boundary `captured_at` value and photo ID.
36. Keep cursor encode/decode helpers private to the repository package unless tests require same-package access.
37. Decode strictly:
    - reject invalid base64;
    - reject malformed or trailing JSON;
    - reject unsupported cursor versions;
    - reject missing fields;
    - reject malformed timestamp;
    - reject malformed UUID.
38. An empty cursor means the first page.
39. Emit an empty `NextToken` on the final page.
40. Build the next token from the last returned item only when the `limit + 1` row proves another page exists.

## Tests

41. Tests must create isolated temporary SQLite fixture databases with the same schema as the supplied database.
42. It is acceptable for tests to create and write their own temporary fixture database. They must never mutate `dataset/predictions.db`.
43. Fixture data must include:
    - multiple photos sharing the same `captured_at` value;
    - a photo with no predictions;
    - a photo with multiple predictions;
    - all necessary class/confidence combinations to expose incorrect separate-prediction matching;
    - deterministic valid UUIDs and timestamps.
44. Test open failure and read-only behavior.
45. Test `PhotoExists` for present, absent, and malformed IDs.
46. Test `GetPhoto` mapping, all predictions, no-prediction behavior, not found, malformed ID, malformed timestamp, and invalid persisted bbox/confidence.
47. Test list default limit and explicit limit validation.
48. Test every filter combination and inclusive confidence boundary.
49. Include the critical negative test: one prediction matches the class and a different prediction matches confidence, but no single prediction matches both; that photo must be excluded.
50. Verify that a matched photo still returns predictions that did not satisfy the filter.
51. Test multi-page traversal with no duplicates or omissions, including equal timestamps resolved by ID.
52. Test final-page empty token and all malformed cursor cases.
53. Test context cancellation where practical.
54. Verify the supplied database remains unchanged by this task.

# Out of scope

- Wiring the repository into `cmd/api` or adding database environment configuration.
- HTTP handlers, request parsing, response DTOs, error mapping, middleware, or API routes.
- `originalUrl` generation or any MinIO/S3 integration.
- Upload links, seed client, thumbnails, metrics, or frontend code.
- Modifying domain or application-error APIs unless a demonstrated blocker makes a minimal compatible adjustment necessary; explain any such adjustment before making it.
- Changing the supplied database schema or adding recommended indexes.
- Modifying `README.md`, `openapi.yaml`, `CLAUDE.md`, dataset files, or existing prompts.
- Adding testcontainers, mocking frameworks, migration tools, query builders, or ORMs.
- Committing or pushing changes.

# Acceptance criteria

- The repository opens the supplied SQLite file read-only and never changes it.
- `GetPhoto` and `PhotoExists` have correct typed error behavior.
- Pagination is stable keyset pagination using timestamp plus ID.
- Filters use one prediction alias and reproduce the exact OpenAPI semantics.
- Returned matched photos contain all predictions.
- Prediction loading is batched and does not create an N+1 query pattern.
- Invalid persisted data becomes a safe internal error with its underlying cause retained.
- Tests are deterministic, isolated, and do not require MinIO, Docker, network access, or the supplied dataset.
- Existing bootstrap/domain/application-error tests remain passing.
- Only allowed source/module files are changed.
- No generated executable, database, coverage file, or other build artifact remains in the repository.

# Verification

From `backend/`, run:

```bash
gofmt -w internal/repository/sqlite
go mod tidy
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/scout-api ./cmd/api
```

Run the repository package repeatedly to catch ordering or state leakage:

```bash
go test -count=10 ./internal/repository/sqlite
```

Confirm the supplied database was not modified and no build artifact was left in `backend/`. Then, from the repository root, run:

```bash
git diff --check
git status --short
```

Inspect the complete diff before reporting completion.

# Final report

Report:

1. files and module dependencies created or changed;
2. repository API and read-only connection strategy;
3. exact pagination and same-prediction filter strategy;
4. verification commands and results;
5. confirmation that `dataset/predictions.db` was unchanged;
6. assumptions or checks that could not be completed.

Do not proceed to HTTP wiring, MinIO, upload links, seed logic, or thumbnails.
