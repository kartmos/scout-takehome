---
name: backend-review
description: Reviews the Scout backend for assignment compliance, correctness, safety, and production risks.
disable-model-invocation: true
---

Perform a read-only backend review. Do not edit files, create fixes, invoke
subagents, or start frontend work.

Start with `git status --short` and the backend diff. Review the current backend
implementation, not only uncommitted lines, when a changed component depends on
earlier committed code. Read only the production/test files needed to validate a
concrete concern; do not inventory or restate the repository.

## Review priorities

### Assignment and API contract

- `README.md` and relevant `openapi.yaml` operations are implemented exactly;
- status codes, JSON field names/nullability, typed errors, cursor pagination and
  same-prediction filter semantics are correct;
- seed is rerunnable and uploads through the public upload-link API;
- required thumbnail behavior, metrics and ingest-to-read smoke exist.

### Security and observability

- API keys, S3 credentials, signed URLs, request/response bodies and stack traces
  never leak through errors, logs or metrics;
- request IDs and structured logs remain useful without raw query strings;
- metric labels are statically bounded: never IDs, raw paths, cursors, URLs,
  cache keys or error text;
- auth boundaries are correct for Data API, health, metrics and thumbnails.

### Correctness and bounded resources

- contexts, cancellation and server shutdown lifetimes are correct, including
  shared singleflight work and independently cancelled callers;
- goroutines, worker pools, HTTP bodies, files, DB rows and object streams are
  bounded and always released;
- SQLite remains read-only; queries are parameterized, keyset pagination is
  stable, and page/prediction loading avoids N+1 behavior;
- image dimensions/pixels, decode/encode concurrency and output bytes are bounded;
- cache keys/paths are safe, writes are atomic, partial files are cleaned, disk
  budget and startup reconciliation account for every cache-owned entry;
- identical thumbnail misses generate once, while hit/miss/generation metrics
  have documented and internally consistent semantics;
- ETag, 304, range and Cache-Control behavior are stable and CDN-safe.

### Tests and dependencies

- tests cover assignment-critical behavior: seed, parsing/dimension math,
  cancellation/concurrency, cache atomicity/eviction/singleflight, API errors,
  metrics cardinality and real ingest→read→thumbnail smoke;
- concurrency tests are deterministic and race-safe rather than sleep-based;
- tests actually exercise production paths instead of manually incrementing
  metrics or asserting only that code did not panic;
- dependencies and abstractions are necessary, focused and used in production or
  meaningful tests.

## Finding threshold

Report a finding only when there is a concrete failure mode, contract violation,
security/resource risk, misleading operational signal, or important missing
regression test. Do not report subjective style preferences, harmless trade-offs,
or duplicate variants of the same root cause.

Order findings by severity:

1. **Critical** — security exposure, data corruption, unusable startup, or core
   assignment failure.
2. **High** — incorrect API/data behavior, race, resource escape/leak, broken
   pagination/ingest/cache correctness.
3. **Medium** — realistic reliability, observability, cancellation, performance,
   or important test gap.
4. **Low** — small but actionable cleanup with a concrete maintenance or behavior
   benefit.

For each finding include one tight file/line range, the failure scenario, impact,
and minimal fix direction. Merge findings that share one root cause.

If there are no actionable findings, say so explicitly, list the areas checked in
one compact paragraph, and mention only genuine remaining verification gaps.
