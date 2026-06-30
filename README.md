# Scout Service: Greenhouse Pest & Disease Monitoring

In a greenhouse, pests and diseases spread fast and quietly. Scout is the early-warning system: cameras
photograph the crop around the clock, a model flags pests and diseases with bounding boxes, and growers
open Scout to see what was found, filter to what they care about, and pinpoint where in the greenhouse
it is happening — before it spreads.

Build the whole service:

- A **backend** that ingests and serves the photos, with an **on-demand thumbnail engine**.
- A **web app** with:
  - a **gallery** — the photos with their detection boxes drawn on; reasonably polished, with the ability
    to open a photo full-size and inspect its predictions;
  - a **greenhouse map** of where the problems are (a strong bonus).

Submit your solution as a **public GitHub repo** with the whole thing; it must run from a clean clone (a
README with the steps, seed/ingest included). Send us the link.

## TODO

1. **Load the data into local object storage.** Implement `POST /photos/{photoId}/upload-link` (presigned
   PUT) and a seed client that uploads every `dataset/images/` photo into **MinIO** (re-runnable; the key is
   the photo id). Serve `GET /photos` (cursor + filters) and `GET /photos/{photoId}` straight from
   `predictions.db`. Each photo carries its position, predictions, and an `originalUrl`.

2. **Thumbnail engine — your design** *(the most important part)*. Return any photo at the size, DPR, and
   quality a client asks for, generated on demand (the interface is yours). Originals are 2560×1440, and the
   gallery asks for many, at several sizes, at once. Generation is the expensive part — don't pay for the
   same thumbnail twice. One small box (see **Runtime**).

3. **Gallery** *(bbox overlay graded hardest)*.
   - A scrolling, paginated grid of responsive thumbnails (ask your endpoint for the widths and DPRs you need
     via `srcset`/`sizes`), with each photo's bounding boxes drawn correctly at every size and DPR. Boxes are
     normalized [0,1] — map them onto the rendered size, not the original. Filter by class and confidence;
     open a photo to see it large with its predictions.

4. *(Additional, for strong frontenders :))* **Greenhouse map.** A floor plan (40×40 m) with each photo at
   its `x,y` on a Konva canvas (zoom and pan); show where the predictions are. Click a spot to filter the
   gallery to photos near it (you decide what "near" means). The class filter drives both views (shared
   state).

**Tests:** the bbox coordinate transform (the crux), thumbnail-request parse/validate + coordinate math, a
key component or reducer, and one ingest-then-read backend smoke. Runs from a clean clone.

## Stack

- **Backend: Go preferred.** Rust or Node.js are accepted.
- **Frontend: React and TypeScript are required.** The rest is a strong recommendation (swap something?
  say why):

  React 19 · Vite · pnpm · Redux Toolkit · openapi-typescript · react-konva · CSS Modules · vitest ·
  feature-based folders · `interface` over `type` · no `any` · no magic numbers.

## Additional Requirements

- **Errors.** Backend: right HTTP status + typed `Error` body, 4xx vs 5xx, shaped in one place, no leaked
  stack traces. UI: real loading, empty, and error states (never a blank screen or broken-image grid).
- **Logs.** Structured backend logs, one request traceable by a correlation id, sane levels, no secrets.
- **Metrics.** `/metrics` (or similar): rate, latency, errors, plus thumbnail cache hit/miss and generation
  time.

## Runtime

- One small box: ~1 vCPU, 512MB–1GB. Everything you build runs here — object storage (MinIO local,
  S3 in prod) is the one thing that's external
- Clients are spread across continents

## Repo

```
README.md      this assignment
openapi.yaml   the data-API contract
dataset/       50 photos + predictions.db
```

No scaffold — `git init` and build it. Use AI tools however you like; commit your AI setup
(`CLAUDE.md`/`AGENTS.md`, skills, agents) — we read it as a first-class artifact.

## Dataset

All photos come from a single greenhouse — call it **AlfaGreen** — a fixed **40×40 m** plane. A photo's
`x`/`y` place it on that plane (and on the map); `h` is the camera height above the floor.

- 50 photos here are a sample — the real catalog is far bigger; build for volume, not for 50
- Image files are raw material to ingest; the service serves originals from object storage, not this folder.
- bbox is relative to the original image; multiply corners by the render size.

```
dataset/
├── images/         50 greenhouse JPEGs, 2560×1440  (filename = <photo id>.jpg)
└── predictions.db  SQLite. Your database: read it, don't build your own.
```

```sql
photos(id, x, y, h, width, height, captured_at)
  -- x, y = location in greenhouse (m), for the map; h = camera height (m); width, height = pixels
predictions(id, photo_id, class_id, confidence, bbox_xmin, bbox_ymin, bbox_xmax, bbox_ymax)
  -- class_id: powdery_mildew, mirid, whitefly_aphid, miner_tuta, thrips, spider_mites
  -- bbox: normalized [0,1], (xmin,ymin) top-left to (xmax,ymax) bottom-right
```

## Data API

See [`openapi.yaml`](./openapi.yaml).

| Method | Path | What you get |
|---|---|---|
| `POST` | `/photos/{photoId}/upload-link` | presigned PUT URL; push the original to object storage |
| `GET` | `/photos` | photos (predictions + position + `originalUrl`), cursor-paginated, filters `classId` / `minConfidence` |
| `GET` | `/photos/{photoId}` | one photo |

Filters are optional and combine on a single prediction: a photo matches if one of its predictions is that
class with confidence ≥ `minConfidence`. You always get all of a photo's predictions. Thumbnail delivery is
yours to design, not in the contract.
