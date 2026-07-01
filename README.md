# Scout — Greenhouse Pest & Disease Monitoring

Scout is an early-warning system for greenhouse crops. Cameras photograph the crop around the clock; a computer-vision model flags pests and diseases with bounding boxes; growers open Scout to see what was found, filter to a class or confidence, and pinpoint where in the greenhouse it is happening.

This repo is the full implementation: a Go data API with an on-demand thumbnail engine, a React/TypeScript gallery/viewer, and a Docker Compose stack that wires them together.

---

## Prerequisites

- Docker Desktop (WSL integration enabled on Windows)
- Go 1.26+ (native backend dev)
- Node.js 24 + pnpm 10.34.4 (native frontend dev)
- `dataset/` present and committed (Git-visible; must be staged and committed before publishing for clean-clone reproduction)

---

## Quick start (clean clone)

```bash
# 1. Clone and enter the repo
git clone <repo-url> scout && cd scout

# 2. Copy and configure environment variables
cp .env.example .env
# Edit .env: change SCOUT_API_KEY (required), leave MinIO credentials as-is for local dev.

# 3. Build images and start the local stack (MinIO + API + web)
docker compose up --build -d

# 4. Wait for all services to be healthy
docker compose ps   # all should be "healthy" within ~60 s

# 5. Seed photos into MinIO (re-runnable; idempotent per photo ID)
docker compose --profile seed run --rm seed

# 6. Open the gallery
# Visit http://localhost:8090 in a browser (or the SCOUT_WEB_PORT you set in .env)
```

Running seed a second time is safe — existing objects are overwritten in place and the SQLite database is not modified.

---

## Native development (no Docker)

### Backend

```bash
cp .env.example .env          # adjust values; MinIO must be reachable at SCOUT_S3_ENDPOINT

# Run the data API (load env vars from .env, then override the database path)
cd backend
set -a && . ../.env && set +a
SCOUT_DATABASE_PATH=../dataset/predictions.db go run ./cmd/api

# Run the seed tool
go run ./cmd/seed \
  --api-url http://localhost:8080 \
  --images-dir ../dataset/images
```

### Frontend

```bash
cd frontend
pnpm install
pnpm dev          # Vite dev server at http://localhost:5173
```

The frontend reads `VITE_SCOUT_API_BASE_URL` and `VITE_SCOUT_API_KEY` from `../.env` (the repo root `.env`) via `envDir: '../'` in `vite.config.ts`.

---

## Test, lint, and build commands

### Backend

```bash
cd backend

# Format
gofmt -w .

# Unit + race tests
go test -race ./...

# Integration tests (requires SCOUT_S3_* and a running MinIO — skipped otherwise)
go test -race -tags integration ./...

# Static analysis
go vet ./...

# Build binaries
go build -o /tmp/scout-api  ./cmd/api
go build -o /tmp/scout-seed ./cmd/seed
```

### Frontend

```bash
cd frontend      # or use --dir frontend from repo root

pnpm install --frozen-lockfile   # install exact locked deps

pnpm generate:api   # regenerate types from ../openapi.yaml (run after API schema changes)
pnpm typecheck      # tsc -b (strict, no emit)
pnpm test --run     # vitest
pnpm lint --max-warnings=0
pnpm build          # production build → dist/
```

---

## Production deployment

### Build images

```bash
# Build API image (context: repo root)
docker build -f backend/Dockerfile -t scout-api:0.1.0 .

# Build web image (context: repo root); supply the demo API key at build time
docker build -f frontend/Dockerfile \
  --build-arg VITE_SCOUT_API_KEY=<your-api-key> \
  -t scout-web:0.1.0 .
```

### Run with external S3 (AWS, Cloudflare R2, etc.)

```bash
export SCOUT_API_IMAGE=scout-api:0.1.0
export SCOUT_WEB_IMAGE=scout-web:0.1.0
export SCOUT_API_KEY=<your-strong-key>
export SCOUT_CORS_ALLOWED_ORIGINS=https://your-domain.example.com
export SCOUT_S3_ENDPOINT=s3.amazonaws.com         # or bucket.s3.region.amazonaws.com
export SCOUT_S3_ACCESS_KEY=<access-key-id>
export SCOUT_S3_SECRET_KEY=<secret-access-key>
export SCOUT_S3_BUCKET=<bucket-name>
export SCOUT_S3_SECURE=true
export SCOUT_S3_REGION=us-east-1

docker compose -f compose.production.yaml up -d
```

The production stack excludes MinIO. Object storage is entirely external and does not consume the small-box CPU/memory budget.

---

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `SCOUT_API_KEY` | ✓ | — | API authentication key (X-API-Key header) |
| `SCOUT_DATABASE_PATH` | ✓ | — | Path to `predictions.db` |
| `SCOUT_HTTP_ADDR` | | `:8080` | API listen address |
| `SCOUT_CORS_ALLOWED_ORIGINS` | | `http://localhost:5173` | Comma-separated allowed CORS origins |
| `SCOUT_S3_ENDPOINT` | ✓ | — | `host:port` for internal API→MinIO/S3 I/O |
| `SCOUT_S3_PUBLIC_ENDPOINT` | | (= `SCOUT_S3_ENDPOINT`) | `host:port` embedded in presigned URLs for browser + seed access |
| `SCOUT_S3_PUBLIC_SECURE` | | (= `SCOUT_S3_SECURE`) | TLS for presigned URLs (`true`/`false`) |
| `SCOUT_S3_ACCESS_KEY` | ✓ | — | S3 access key (never logged) |
| `SCOUT_S3_SECRET_KEY` | ✓ | — | S3 secret key (never logged) |
| `SCOUT_S3_BUCKET` | ✓ | — | S3 bucket name |
| `SCOUT_S3_SECURE` | ✓ | — | TLS for internal endpoint (`true`/`false`) |
| `SCOUT_S3_REGION` | | `us-east-1` | S3 region (used offline for presigning) |
| `SCOUT_S3_UPLOAD_TTL` | | `15m` | Presigned PUT lifetime (1s–168h) |
| `SCOUT_S3_DOWNLOAD_TTL` | | `15m` | Presigned GET lifetime (1s–168h) |
| `SCOUT_THUMBNAIL_CACHE_DIR` | | `/tmp/scout-thumb-cache` | Thumbnail disk cache root |
| `SCOUT_THUMBNAIL_CACHE_MAX_BYTES` | | `268435456` (256 MiB) | Max cache disk usage in bytes |
| `SCOUT_THUMBNAIL_GENERATION_CONCURRENCY` | | `1` | Max concurrent thumbnail generations (1–2) |
| `VITE_SCOUT_API_BASE_URL` | ✓ (Vite) | — | API base URL for the browser bundle (`/api` or absolute) |
| `VITE_SCOUT_API_KEY` | ✓ (Vite) | — | API key baked into the browser bundle (see Security) |
| `SCOUT_MINIO_API_PORT` | | `9000` | Local MinIO host port |
| `SCOUT_MINIO_CONSOLE_PORT` | | `9001` | Local MinIO console host port |
| `SCOUT_WEB_PORT` | | `8090` | Web frontend host port |
| `SCOUT_API_PORT` | | `8080` | API direct host port (debug/smoke only) |
| `SCOUT_SEED_CONCURRENCY` | | `2` | Seed upload concurrency |

### Internal vs public S3 endpoints

The API uses two separate MinIO clients:

- **Internal** (`SCOUT_S3_ENDPOINT`): used for bucket health checks and reading originals during thumbnail generation. Never appears in URLs returned to clients.
- **Public** (`SCOUT_S3_PUBLIC_ENDPOINT`): used only for presigning PUT/GET URLs. Must be reachable by both the host browser and any container (e.g. the seed) that performs the actual upload. Defaults to the internal endpoint when unset.

In the local Compose stack, `minio.localhost` is a Docker network alias for the MinIO container, so presigned URLs with `minio.localhost:9000` resolve correctly from both inside the Docker network and the host (where `*.localhost` maps to `127.0.0.1` per RFC 6761).

---

## Data API

**Auth**: `GET /photos`, `GET /photos/{id}`, and `POST /photos/{id}/upload-link` require `X-API-Key: <SCOUT_API_KEY>`. The thumbnail (`GET /photos/{id}/thumbnail`), health (`GET /healthz`), and metrics (`GET /metrics`) routes are public.

| Method | Path | Description |
|---|---|---|
| `GET` | `/photos` | Cursor-paginated list with predictions, position, and `originalUrl` |
| `GET` | `/photos/{id}` | Single photo |
| `POST` | `/photos/{id}/upload-link` | Presigned PUT URL for uploading the original |
| `GET` | `/photos/{id}/thumbnail` | On-demand thumbnail (see below) |
| `GET` | `/healthz` | `{"status":"ok"}` — no auth required |
| `GET` | `/metrics` | Prometheus text exposition (no auth in local build) |

### Filters (`GET /photos`)

`classId` and `minConfidence` are optional and combine on a **single prediction**: a photo matches if at least one of its predictions has the specified class and confidence ≥ minConfidence. Matching photos always include **all** of their predictions, not just the matching one.

`cursor` is opaque; pass the `next_token` from the previous response. `limit` is the page size (default 50, max 200). There is no total-count limit; the catalog is unbounded.

### Thumbnail contract

```
GET /photos/{id}/thumbnail?width=<px>[&dpr=<1|2|3>][&quality=<1-100>]
```

- Width is in CSS pixels; DPR scales the actual pixel request (1×, 2×, 3×).
- Response is JPEG; `Cache-Control: public, max-age=31536000, immutable` (CDN-friendly).
- `ETag` and `304 Not Modified` are supported.
- Originals are 2560×1440; thumbnails are generated at any width up to 2048 CSS px.
- Thumbnails are **not** proxied through the API for originals; the `originalUrl` field points directly to a presigned S3/MinIO URL.

---

## Architecture and data flow

```
Browser
  │
  ├── GET /api/photos        → Nginx proxy → API → SQLite
  │                                                (predictions.db, read-only)
  ├── GET /api/photos/{id}/thumbnail
  │       │
  │       ▼
  │   Thumbnail cache (disk, LRU eviction)
  │       │ miss
  │       ▼
  │   MinIO / S3 (OpenOriginal via internal endpoint)
  │       │
  │       ▼ JPEG decode + resize
  │       ▼ write to cache (atomic rename)
  │       ▼
  │   Response (immutable Cache-Control, ETag)
  │
  └── GET <presigned-S3-URL> → MinIO / S3 (direct, public endpoint)
```

**SQLite** is mounted read-only; no writes occur during API operation. The thumbnail cache is a separate volume that survives container restarts.

**Presigned URLs** are short-lived (default 15 min) and require no API auth on the S3 side. The download URL for a photo original is embedded in every `GET /photos` response as `originalUrl`.

**Thumbnail engine**: generation uses a singleflight group — duplicate requests for the same key block until the first completes, then all receive the cached result. This prevents redundant decode/encode work under concurrent load. Concurrency is bounded to 1–2 workers to cap peak memory from JPEG decoding of 2560×1440 originals.

**Thumbnail disk cache**: LRU eviction with configurable byte budget. Cache keys encode the canonical photo ID, resolved output pixel dimensions (CSS width × DPR), quality, output format, and a generator version/hash to invalidate stale entries on code changes. A cache hit path skips decoding entirely. Atomic `rename(2)` prevents partial writes. On a cache miss, the singleflight deduplicates concurrent generation for the same key.

**Bounding boxes**: `bbox_xmin`, `bbox_ymin`, `bbox_xmax`, `bbox_ymax` are normalized `[0, 1]` coordinates relative to the original image dimensions. To draw them correctly at any thumbnail size:
```
rendered_x = bbox_xmin * rendered_width
rendered_y = bbox_ymin * rendered_height
```
DPR scales the pixel request; CSS coordinates remain the same. The overlay uses CSS coordinates, not device pixels.

---

## Observability

### Structured logs

All API logs are JSON (slog) with a `request_id` field on every request log line. Errors are logged with `"level":"ERROR"` and never include credentials or stack traces.

### Metrics (`/metrics`)

Prometheus text format. Key metrics:

| Metric | Description |
|---|---|
| `scout_http_requests_total` | Requests by method, path, status |
| `scout_http_request_duration_seconds` | Latency histogram |
| `scout_thumbnail_cache_hits_total` | Cache hit count |
| `scout_thumbnail_cache_misses_total` | Cache miss count |
| `scout_thumbnail_cache_evictions_total` | LRU eviction count |
| `scout_thumbnail_cache_bytes` | Current cache disk usage |
| `scout_thumbnail_generation_duration_seconds` | Generation time histogram |
| `scout_thumbnail_generation_errors_total` | Generation error count |

---

## Resource budget

| Component | Local (compose.yaml) | Production (compose.production.yaml) |
|---|---|---|
| API | 0.5 CPU / 256 MiB | 0.75 CPU / 384 MiB |
| Web (nginx) | 0.25 CPU / 64 MiB | 0.25 CPU / 64 MiB |
| MinIO | 0.5 CPU / 256 MiB | external |
| **Total (prod API+web)** | — | **~1 CPU / 448 MiB** |

The thumbnail cache (default 256 MiB) is a **disk volume**, not memory. Disk budget comes from the host volume, not the container RAM limit. `GOMEMLIMIT` is set to leave practical native/runtime headroom below the container `mem_limit` (production: `GOMEMLIMIT=320MiB` with a `384m` container limit).

---

## Security limitations

- **Static browser API key**: `VITE_SCOUT_API_KEY` is baked into the JS bundle at build time and is visible to anyone who loads the page. This is a structural limitation of the single-key demo design. A production system would use per-user tokens and server-side auth.
- **SQLite read-only**: `predictions.db` is mounted read-only; the API cannot corrupt or modify it. All writes at ingest time go through the seed binary and the object storage API.
- **No rate limiting**: the API has no built-in rate limiting. Use a reverse proxy or WAF for production workloads.
- **Single-instance singleflight**: the thumbnail dedup singleflight is in-process only. With multiple API replicas, duplicate generation is possible across instances. For this project's target of one small box this is not a concern.
- **No optional greenhouse map**: task 016 (map view) is not implemented.

---

## Cleanup and reset

```bash
# Stop and remove containers; preserve named volumes (minio-data, thumb-cache)
docker compose down

# Stop and remove containers AND all named volumes (destroys uploaded photos and thumbnail cache)
docker compose down -v

# Re-run seed after a volume wipe (safe; it's idempotent)
docker compose --profile seed run --rm seed
```

To re-seed without destroying data (e.g. after adding photos):

```bash
docker compose --profile seed run --rm seed
```

---

## Known task-018 items

- The historic `backend/api` binary blob committed in an earlier task remains in git history. It is a known hygiene item to be cleaned up in task 018 (history rewrite / `git filter-repo`). The file itself is excluded from the working tree by `.gitignore`.
- `dataset/` files (`dataset/images/*.jpg`, `dataset/predictions.db`) are Git-visible (not in `.gitignore`) but not yet committed. They must be staged and committed before the repo is published for clean-clone reproduction.
- **Docker runtime unverified**: image builds, the full Compose stack, browser smoke, and resource measurements listed above have not been runtime-tested in this workspace because the Docker daemon (WSL integration) is unavailable. Compose configuration has been statically validated with `docker.exe compose ... config`. Full runtime acceptance belongs to task 018.
