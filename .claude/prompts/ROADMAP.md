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
- HTTP infrastructure этапов 005/005.1 реализована и проверена, включая recovery diagnostics.
- `go test -race ./...` и `go vet ./...` проходят после этапа 005.1.
- Docker Compose и локальный MinIO development environment добавлены; Node.js, pnpm и frontend ещё не добавлены.

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
| `005-http-infrastructure` | `DONE` | API-key middleware, request ID, centralized errors, recovery, access logs, CORS, bounded headers | Проверен вместе с hardening 005.1; full tests, race и vet проходят |
| `005.1-http-infrastructure-hardening` | `DONE` | Один panic diagnostic с request ID/value, безопасный 500 без утечки, отдельный access-completion log | Фактический diff проверен; full tests, race, vet и diff check прошли |
| `006-minio-development-environment` | `DONE` | Local MinIO Compose, persistent volume, healthcheck, idempotent private bucket init, `.env.example` | Фактические файлы проверены; Compose config, health и повторный init проходят |
| `007-object-storage-adapter` | `HARDENING` | Strict S3 config, presigned PUT/GET, cancellable original stream, typed safe storage errors | Реализация и тесты проходят; review выявил endpoint/cleanup/dependency hygiene gaps |
| `007.1-object-storage-hardening` | `READY` | Strict endpoint path, bucket access classification, observable integration cleanup, shared UUID validation | Узкий экономный промт создан; ожидает выполнения и проверки |

HTTP foundation и MinIO development environment через этап 006 закрыты. Текущий следующий этап — `007-object-storage-adapter`.

## Скорректированный план следующих промтов

После этапа 007 план укрупнён по принципу «один общий рабочий контекст — один промт». Связанные операции реализуются последовательными checkpoint-блоками внутри одного запуска Claude Code, с focused tests после каждого checkpoint и полным quality gate в конце. Это сокращает повторное чтение одних и тех же router/config/OpenAPI/frontend файлов, но не объединяет наиболее рискованные подсистемы в один огромный этап.

Обязательная последовательность после 007 сокращена с 17 отдельных промтов до 10. Bonus map остаётся отдельным optional-промтом. Корректирующие суффиксы `.1` создаются только по реальным замечаниям review, а не заранее.

| № | Статус | Этап | Что реализуется | Новая технология на этапе |
|---|---|---|---|---|
| `005` | `DONE` | HTTP infrastructure | API-key middleware, correlation ID, centralized OpenAPI error writer, recovery, request logs, CORS, bounded headers/timeouts; `/healthz` остаётся публичным | Нет |
| `005.1` | `DONE` | HTTP infrastructure hardening | Исправление panic diagnostics без duplicate internal log; безопасный typed 500 и regression tests | Нет |
| `006` | `DONE` | MinIO development environment | Docker Compose для локального external object storage, MinIO, bucket initialization, healthcheck, `.env.example`; MinIO не включается в production service memory budget | Docker Desktop + WSL Integration настроены |
| `007` | `HARDENING` | Object storage adapter | MinIO/S3 client, presigned PUT/GET, object read, typed storage failures, configuration | `github.com/minio/minio-go/v7 v7.2.1`; системной установки нет |
| `007.1` | `READY` | Object storage hardening | Четыре review-исправления без изменения публичного adapter API | Нет |
| `008` | `PLANNED` | Complete Data API | За один общий HTTP/application context: repository/storage wiring, bounded request parsing, exact OpenAPI DTOs и все три операции — upload-link, list photos, get photo; per-route auth, typed errors, presigned `originalUrl`; page size 1–200 без total cap | Нет |
| `009` | `PLANNED` | Seed client and Data API smoke | Идемпотентный seed через публичный upload-link API с concurrency default `2`, строгая сверка dataset, exit codes; ingest→list/get end-to-end smoke через MinIO и SQLite | Нет |
| `010` | `PLANNED` | Thumbnail core | В одном package context: width/DPR/quality contract, safe effective dimensions/pixel bounds, canonical key, no upscale, streaming JPEG decode/resize/encode, cancellation, generation semaphore default `1` max `2` | `golang.org/x/image`; без libvips/ImageMagick |
| `011` | `PLANNED` | Bounded thumbnail cache and endpoint | Disk cache max bytes + eviction, atomic writes, singleflight, cache-hit path без original decode, public endpoint, ETag/304 и CDN-friendly Cache-Control | `golang.org/x/sync/singleflight` |
| `012` | `PLANNED` | Observability and backend acceptance | HTTP rate/latency/errors, thumbnail hit/miss/generation metrics, low-cardinality `/metrics`; ingest→read→thumbnail smoke, concurrent duplicate suppression, bounded-resource checks и независимый backend review | Prometheus Go client |
| `013` | `PLANNED` | Frontend foundation | React 19/Vite/strict TypeScript/pnpm/CSS Modules/Vitest/Testing Library, feature folders; generated OpenAPI types, typed client/RTK Query, normalized errors, shared class/confidence filters and reducer tests | Установка Node.js LTS и pnpm; Redux Toolkit и openapi-typescript |
| `014` | `PLANNED` | Responsive paginated gallery | Pure bbox/contain geometry tests, responsive photo card with `srcset`/`sizes`, lazy/async images, ResizeObserver overlay; cursor gallery, bounded/virtualized DOM/page cache, shared filters, loading/empty/error/retry states | Возможно `@tanstack/react-virtual`, только после обоснования |
| `015` | `PLANNED` | Full-size viewer and UI hardening | Accessible dialog, contain-mode bbox overlay, predictions, keyboard/focus behavior; responsive polish, broken-image/retry behavior, integration tests и независимый frontend review | Нет |
| `016` | `OPTIONAL` | Greenhouse map | 40×40 m canvas, zoom/pan, markers, near-location selection и shared filters; только уже загруженные/bounded данные, без загрузки всего каталога | `react-konva` и `konva` только здесь |
| `017` | `PLANNED` | Production runtime and documentation | Multi-stage API/web images, Nginx/reverse proxy, healthchecks, CPU/RAM/cache-volume limits; external MinIO profile; clean-clone README, env/seed/test/run steps, architecture/cache/trade-offs | Docker уже установлен на 006 |
| `018` | `PLANNED` | Final acceptance | Clean-clone reproduction, full API/UI matrix, pagination beyond 200 total photos, concurrent thumbnails, 1 CPU + 512 MB–1 GB run, secret/artifact/history hygiene и финальные backend/frontend reviews | Нет |

## Границы объединения

Чтобы сокращение промтов не снижало качество:

1. `008` объединяет Data API операции, потому что они используют один router/DTO/config/repository/storage context; seed остаётся в `009`, так как это отдельный внешний клиент и end-to-end gate.
2. Parsing и generation объединены в `010`, но cache/endpoint остаются в `011`: ошибки размеров и ошибки конкурентного disk cache требуют разных focused test suites.
3. Metrics добавляются только после готового endpoint в `012`; этот же этап закрывает backend независимым review до начала frontend.
4. Frontend bootstrap/client/state объединены в `013`; bbox/card/gallery — в `014`, потому что их корректность проверяется вместе на реальном rendered size. Viewer/accessibility остаются отдельным polish gate `015`.
5. Production packaging и README объединены в `017`, поскольку документация должна описывать фактические контейнеры. Финальная приёмка `018` всегда отдельная и не реализует новые функции.

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
/implement-task @.claude/prompts/007.1-object-storage-hardening.md
```
