---
name: backend-review
description: Reviews Scout backend changes for correctness and assignment compliance.
disable-model-invocation: true
---

Perform a read-only review of the current backend diff. Do not edit files.

Check:

- compliance with `README.md` and `openapi.yaml`;
- correct API status codes and typed error bodies;
- API-key handling, correlation IDs and structured logs without secrets;
- SQLite query correctness, cursor pagination and filter semantics;
- MinIO ingest and repeatable seed behavior;
- thumbnail validation, cache keys, duplicate-work prevention and bounded concurrency;
- metrics required by the assignment;
- error handling, resource leaks and missing tests;
- unnecessary abstractions or dependencies.

Return actionable findings ordered by severity. Include file and line references. If no problems are found, state what was checked and any remaining testing gaps.
