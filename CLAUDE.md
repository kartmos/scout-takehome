# Scout project instructions

## Source of truth

- Read `README.md` and `openapi.yaml` before implementing a task.
- Follow the assignment exactly. Do not add product features that were not requested.
- Keep the solution understandable for a junior+/middle developer.
- Prefer simple code over unnecessary abstractions.

## Required stack and constraints

- Backend: Go is preferred.
- Frontend: React and TypeScript are required.
- Use the provided `dataset/predictions.db` directly; do not replace it with another database.
- Originals must be stored in MinIO, not served from `dataset/images`.
- Do not modify `openapi.yaml` unless the user explicitly requests it.
- Respect the runtime limit of about 1 vCPU and 512 MB–1 GB RAM.
- Treat the OpenAPI `limit` as a page-size limit only: default 50, maximum 200. Never impose a total photo-count limit; the real catalog is much larger than the 50-photo sample.
- Keep memory, disk, connection pools, goroutines and background work explicitly bounded. Do not load or render the entire catalog at once.
- Object storage is external to the production service budget. The API, thumbnail engine, frontend/static server and local thumbnail cache must fit on the small box.
- Make image delivery CDN/cache friendly for clients across continents; do not solve geographic latency by keeping unbounded local state.
- Do not introduce microservices, Redis, queues or Kubernetes unless explicitly requested.

## Working agreement

- Work on one small task at a time.
- Before editing, restate the goal, scope and acceptance criteria.
- Change only files required for the current task.
- Explain every new production dependency.
- Add or update relevant tests.
- Run the relevant formatter, tests, lint and type checks when available.
- Inspect the diff before reporting completion.
- Report checks that could not be run.
- Never commit or push unless the user explicitly asks.

## Important correctness areas

- Photo filters must match on the same prediction, while responses include all predictions.
- Bounding boxes use normalized `[0,1]` coordinates and must map to the rendered image size.
- DPR affects the requested image pixels, not the CSS coordinates of the overlay.
- Thumbnail generation must validate inputs, avoid duplicate work and use bounded concurrency.
- Thumbnail disk caching must have a configurable size bound and eviction policy; a cache hit path must not decode the original.
- Backend errors must have correct status codes and a typed body.
- UI must have loading, empty and error states.

## Prompt workflow

Task prompts prepared with Codex are stored in `.claude/prompts/`.
Run a task with:

`/implement-task @.claude/prompts/<prompt-file>.md`

After implementation, run `/backend-review` or `/frontend-review` as appropriate.
