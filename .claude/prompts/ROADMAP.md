# Scout implementation roadmap

Этот файл фиксирует фактическое состояние проекта и актуальную последовательность промтов. Он обновляется после проверки каждого выполненного этапа.

## Статусы

- `DONE` — промт выполнен, код и проверки просмотрены Codex.
- `HARDENING` — основная реализация готова, но quality review выявил корректировки.
- `READY` — промт создан и готов к запуску в Claude Code.
- `PLANNED` — этап согласован, но промт ещё не создан.
- `OPTIONAL` — бонусный этап, выполняется после обязательного MVP.

## Фактическое состояние

- Ветка: `feat/scout-mvp`.
- Go: `1.26.4`.
- Backend использует Go standard library и `modernc.org/sqlite v1.53.0`.
- Реализованы HTTP bootstrap, `/healthz`, конфигурация, graceful shutdown, domain models, application errors и SQLite repository.
- `go test -race ./...` и `go vet ./...` проходят после этапа 004.
- Docker, MinIO, Node.js, pnpm и frontend ещё не добавлены.

## Runtime и масштабирование

Целевая машина из ТЗ: примерно `1 vCPU` и `512 MB–1 GB RAM`. Object storage является внешним; в этот бюджет должны помещаться API, thumbnail engine, frontend/static proxy и локальный thumbnail cache.

`GET /photos?limit=` не ограничивает размер каталога. Это только размер одной страницы:

- default: `50`;
- minimum: `1`;
- maximum: `200`;
- суммарное количество фотографий не ограничивается;
- все страницы доступны через cursor pagination.

Для всех следующих промтов действуют обязательные ограничения:

1. Backend не загружает весь каталог в память и не использует `OFFSET`; одна операция ограничена текущей страницей максимум в 200 photos и их predictions.
2. SQLite pool остаётся маленьким; goroutines, HTTP bodies, seed workers и thumbnail workers имеют явные пределы.
3. Thumbnail generation по умолчанию выполняется с concurrency `1`; увеличение максимум до `2` допускается только конфигурацией и тестом памяти. Один decoded original 2560×1440 занимает примерно 14 MiB без учёта дополнительных буферов.
4. Одинаковые thumbnail requests объединяются через singleflight. Cache hit не читает и не декодирует original.
5. Дисковый thumbnail cache имеет configurable max bytes, атомарную запись и eviction. Он не может расти бесконечно.
6. Gallery использует lazy image loading, responsive candidates и bounded rendered DOM. Она не запрашивает автоматически весь каталог.
7. Bonus map не имеет права предварительно загружать все страницы большого каталога. Она работает с уже загруженными/видимыми данными либо требует отдельно спроектированного bounded API.
8. Metrics используют low-cardinality labels; photo ID, cursor и URL не попадают в labels.
9. Для клиентов на разных континентах thumbnails получают корректные `Cache-Control`, `ETag` и стабильные cache keys, чтобы перед сервисом можно было поставить CDN.
10. Финальная приёмка включает запуск с ограничением `1 CPU` и memory limit, а не только функциональные тесты на машине разработчика.

## Выполненные и текущие этапы

| Этап | Статус | Результат | Проверка |
|---|---|---|---|
| `001-dataset-audit` | `DONE` | Проверены SQLite schema, integrity, 50 photos, 92 predictions, bbox, классы и соответствие 50 изображений БД | Dataset согласован с README/OpenAPI; аномалий нет |
| `002-backend-bootstrap` | `DONE` | Go module, config, HTTP server, `/healthz`, timeouts, JSON slog, graceful shutdown | `go test`, `go vet`, build и ручной smoke прошли |
| `003-domain-models-and-errors` | `DONE` | Domain photo/prediction/bbox/class models, validation, UUID checks, typed safe application errors | Unit tests, race detector и vet прошли |
| `004-sqlite-repository` | `DONE` | Read-only repository, Get/Exists/List, batched predictions, keyset pagination, same-prediction filters | Проверен вместе с hardening 004.1; race tests и vet проходят |
| `004.1-sqlite-repository-hardening` | `DONE` | Safe SQLite URI, реальная read-only проверка, exact cursor timestamp, strict JSON EOF, stable prediction ordering | Фактический diff проверен; `go test -race ./...`, repeated tests, vet и diff check прошли |

Backend foundation через этап 004.1 закрыт. Текущий следующий этап — `005-http-infrastructure`.

## Скорректированный план следующих промтов

| № | Статус | Этап | Что реализуется | Новая технология на этапе |
|---|---|---|---|---|
| `005` | `READY` | HTTP infrastructure | API-key middleware, correlation ID, centralized OpenAPI error writer, recovery, request logs, CORS, bounded headers/timeouts; `/healthz` остаётся публичным | Нет |
| `006` | `PLANNED` | MinIO development environment | Docker Compose для локального external object storage, MinIO, bucket initialization, healthcheck, `.env.example`; MinIO не включается в production service memory budget | Установка Docker Engine + Compose plugin происходит только здесь |
| `007` | `PLANNED` | Object storage adapter | MinIO/S3 client, presigned PUT/GET, object read, typed storage failures, configuration | Go MinIO/S3 dependency; системной установки нет |
| `008` | `PLANNED` | Upload-link API | `POST /photos/{photoId}/upload-link`, OpenAPI DTO, auth, validation, 400/401/404/500 | Нет |
| `009` | `PLANNED` | Seed client | Идемпотентная загрузка `dataset/images` через upload-link API, configurable concurrency с безопасным default `2`, итоговый exit code | Нет |
| `010` | `PLANNED` | Photo read API | `GET /photos`, `GET /photos/{photoId}`, repository wiring, presigned `originalUrl`, exact OpenAPI JSON; `limit` только page size 1–200, без total catalog cap | Нет |
| `011` | `PLANNED` | Thumbnail request contract | Интерфейс endpoint, width/DPR/quality parsing, safe pixel/dimension bounds, effective size, no upscale, canonical cache key | Нет |
| `012` | `PLANNED` | Thumbnail generator | Streaming MinIO read where possible, JPEG decode/resize/encode, aspect ratio, cancellation, configurable generation concurrency default `1` and maximum `2` | `golang.org/x/image`; без libvips/ImageMagick |
| `013` | `PLANNED` | Bounded thumbnail cache and endpoint | Configurable max cache bytes, eviction, atomic rename, singleflight, hit path without decode, ETag, CDN-friendly Cache-Control, public thumbnail route | `golang.org/x/sync/singleflight` |
| `014` | `PLANNED` | Metrics and backend smoke | HTTP rate/latency/errors, thumbnail metrics, low-cardinality labels, `/metrics`, ingest→read→thumbnail integration and bounded-concurrency smoke | Prometheus Go client |
| `015` | `PLANNED` | Frontend bootstrap | React 19, TypeScript strict, Vite, pnpm, CSS Modules, Vitest, Testing Library, feature folders | Установка Node.js LTS и pnpm происходит только здесь |
| `016` | `PLANNED` | Generated API and client | `openapi-typescript`, typed fetch/RTK Query, normalized API errors | Redux Toolkit и openapi-typescript |
| `017` | `PLANNED` | Shared filters | class/minConfidence state, selectors, reset pagination, reducer tests | Нет |
| `018` | `PLANNED` | Bbox geometry | Pure coordinate functions for normal and `object-fit: contain` layouts; DPR-independent CSS coordinates | Нет |
| `019` | `PLANNED` | Responsive photo card | `srcset`, `sizes`, lazy loading, async decode, thumbnail URL candidates, ResizeObserver, bbox overlay, broken-image state | Нет |
| `020` | `PLANNED` | Paginated gallery | Responsive grid, cursor loading without total cap, IntersectionObserver, bounded/virtualized rendered DOM, controlled page cache, loading/empty/error/retry states; запрет auto-fetch всех страниц | Возможно `@tanstack/react-virtual`, только если выбран после обоснования |
| `021` | `PLANNED` | Full-size viewer | Accessible dialog, contain geometry, predictions list, loading/error behavior | Нет |
| `022` | `OPTIONAL` | Greenhouse map | 40×40 m canvas, zoom/pan, shared filters, markers, near-location selection; без загрузки всего каталога в browser memory | `react-konva` и `konva` устанавливаются только здесь |
| `023` | `PLANNED` | Production runtime | Multi-stage images, Nginx/reverse proxy, API/web resource limits, healthchecks, bounded cache volume; MinIO остаётся отдельным dev/external object-storage profile | Docker уже установлен на 006 |
| `024` | `PLANNED` | Documentation | Clean-clone README, env, seed, tests, architecture, cache design, trade-offs | Нет |
| `025` | `PLANNED` | Final acceptance | Full checks, clean-clone run, API/UI matrix, pagination beyond 200 total photos, concurrent thumbnail test, container run near 1 CPU and 512 MB–1 GB, final reviews | Нет |

## Quality gates

Каждый основной этап считается выполненным только после всех пунктов:

1. Claude Code выполнил один конкретный prompt и показал итоговый diff.
2. Codex просмотрел фактический код, а не только отчёт Claude.
3. Formatter, focused tests, race/type checks, lint и build для затронутой части прошли.
4. Не осталось executable, coverage, temp DB или других build artifacts.
5. После backend/frontend логического блока выполнен независимый review skill.
6. Только после закрытия замечаний статус меняется на `DONE`.

## Обязательная repository hygiene до публичной отправки

Эти пункты не меняют функциональный порядок промтов, но должны быть закрыты до публикации:

1. Бинарник `backend/api` был добавлен в commit `3a99ee3`, а затем удалён. Он остаётся в Git history. До push/public submission историю нужно переписать или собрать чистую ветку без бинарника.
2. Добавить `/backend/api` и другие локальные build outputs в `.gitignore`, чтобы проблема не повторилась.
3. `dataset/` сейчас игнорируется, но ТЗ требует запуск из чистого клона с seed/ingest. Размер dataset около 41.6 MiB, поэтому его можно закоммитить один раз обычным Git, если лицензия задания это разрешает. Иначе README должен содержать воспроизводимый способ получить dataset; без одного из этих вариантов clean-clone requirement не выполнен.
4. Не изменять и не индексировать `dataset/predictions.db`; приложение открывает его read-only.

## Запуск текущего этапа

```text
/clear
/implement-task @.claude/prompts/005-http-infrastructure.md
```
