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
| `007-object-storage-adapter` | `DONE` | Strict S3 config, presigned PUT/GET, cancellable original stream, typed safe storage errors | Проверен вместе с hardening 007.1; unit, race, vet и MinIO integration проходят |
| `007.1-object-storage-hardening` | `DONE` | Strict endpoint path, bucket access classification, observable integration cleanup, shared UUID validation | Фактический diff проверен; все четыре исправления и полный quality gate проходят |
| `008-complete-data-api` | `DONE` | Repository/storage wiring, strict bounded parsing, exact OpenAPI DTOs, upload/list/get routes | Фактический diff проверен; focused tests, build и diff check проходят; полный gate прошёл у Claude |
| `009-seed-client` | `READY` | Bounded rerunnable public-API seed client without tests | Только production code и build; ingest→read smoke перенесён в 012 |
| `010-thumbnail-core` | `READY` | Production thumbnail parser/resolver/JPEG generator with bounded concurrency | Промт создан без тестов; `golang.org/x/image` добавляется при выполнении |
| `011-thumbnail-cache-and-endpoint` | `READY` | Bounded atomic disk cache, singleflight and public CDN-friendly endpoint | Промт создан без тестов; `golang.org/x/sync/singleflight` добавляется при выполнении |
| `012-observability-and-backend-gate` | `READY` | Prometheus metrics, deferred backend tests, race/vet/build, real MinIO smoke and reviewer handoff | Запускается только после 009–011; затем отдельный `/backend-review` и при необходимости 012.1 |

Complete Data API через этап 008 закрыт. Текущий следующий этап — `009-seed-client`.

## Скорректированный план следующих промтов

После этапа 008 действует принцип «implementation-промты пишут только production code». Внутри этапов 009–011 и 013–014 разрешены formatter, compile/typecheck и diff check, но запрещены создание и запуск тестов. Полный backend test/review gate выполняется на 012, frontend gate — на 015; замечания закрываются корректирующими 012.1/015.1 только по факту.

Обязательная последовательность после 007 сокращена с 17 отдельных промтов до 10. Bonus map остаётся отдельным optional-промтом. Корректирующие суффиксы `.1` создаются только по реальным замечаниям review, а не заранее.

| № | Статус | Этап | Что реализуется | Новая технология на этапе |
|---|---|---|---|---|
| `005` | `DONE` | HTTP infrastructure | API-key middleware, correlation ID, centralized OpenAPI error writer, recovery, request logs, CORS, bounded headers/timeouts; `/healthz` остаётся публичным | Нет |
| `005.1` | `DONE` | HTTP infrastructure hardening | Исправление panic diagnostics без duplicate internal log; безопасный typed 500 и regression tests | Нет |
| `006` | `DONE` | MinIO development environment | Docker Compose для локального external object storage, MinIO, bucket initialization, healthcheck, `.env.example`; MinIO не включается в production service memory budget | Docker Desktop + WSL Integration настроены |
| `007` | `DONE` | Object storage adapter | MinIO/S3 client, presigned PUT/GET, object read, typed storage failures, configuration | `github.com/minio/minio-go/v7 v7.2.1`; системной установки нет |
| `007.1` | `DONE` | Object storage hardening | Четыре review-исправления без изменения публичного adapter API | Нет |
| `008` | `DONE` | Complete Data API | За один общий HTTP/application context: repository/storage wiring, bounded request parsing, exact OpenAPI DTOs и все три операции — upload-link, list photos, get photo; per-route auth, typed errors, presigned `originalUrl`; page size 1–200 без total cap | Нет |
| `009` | `READY` | Seed client | Идемпотентный seed через публичный upload-link API с concurrency default `2`, bounded streaming и exit codes; без тестов и smoke | Нет |
| `010` | `READY` | Thumbnail core | Production-only: width/DPR/quality contract, safe bounds, canonical key, no upscale, streaming JPEG resize/encode, cancellation, generation semaphore default `1` max `2`; без тестов | `golang.org/x/image`; без libvips/ImageMagick |
| `011` | `READY` | Bounded thumbnail cache and endpoint | Production-only: disk cache max bytes + eviction, atomic writes, singleflight, public endpoint, ETag/304 и CDN-friendly Cache-Control; без тестов | `golang.org/x/sync/singleflight` |
| `012` | `READY` | Observability and full backend gate | Metrics плюс полный backend suite: thumbnail parsing/math, cache concurrency, ingest→read→thumbnail smoke, race/vet/build, bounded-resource checks и отдельный `/backend-review`; затем при необходимости 012.1 | Prometheus Go client |
| `013` | `PLANNED` | Frontend foundation | Production-only: React/Vite/strict TypeScript/pnpm/CSS Modules, feature folders, generated OpenAPI types, typed client/RTK Query, normalized errors и shared filters; без тестов | Установка Node.js LTS и pnpm; Redux Toolkit и openapi-typescript |
| `014` | `PLANNED` | Responsive paginated gallery | Production-only: bbox/contain geometry, responsive cards, `srcset`/`sizes`, lazy images, overlay, bounded cursor gallery и UI states; без тестов | Возможно `@tanstack/react-virtual`, только после обоснования |
| `015` | `PLANNED` | Viewer, UI hardening and full frontend gate | Accessible viewer/polish плюс полный frontend test/type/lint/build suite, bbox/reducer/component/accessibility checks и независимый review; затем при необходимости 015.1 | Vitest и Testing Library устанавливаются здесь |
| `016` | `OPTIONAL` | Greenhouse map | 40×40 m canvas, zoom/pan, markers, near-location selection и shared filters; только уже загруженные/bounded данные, без загрузки всего каталога | `react-konva` и `konva` только здесь |
| `017` | `PLANNED` | Production runtime and documentation | Multi-stage API/web images, Nginx/reverse proxy, healthchecks, CPU/RAM/cache-volume limits; external MinIO profile; clean-clone README, env/seed/test/run steps, architecture/cache/trade-offs | Docker уже установлен на 006 |
| `018` | `PLANNED` | Final acceptance | Clean-clone reproduction, full API/UI matrix, pagination beyond 200 total photos, concurrent thumbnails, 1 CPU + 512 MB–1 GB run, secret/artifact/history hygiene и финальные backend/frontend reviews | Нет |

## Границы объединения

Чтобы сокращение промтов не снижало качество:

1. `008` объединяет Data API операции, потому что они используют один router/DTO/config/repository/storage context; seed остаётся в `009`, так как это отдельный внешний клиент и end-to-end gate.
2. Parsing и generation объединены в `010`, cache/endpoint остаются в `011`; оба этапа пишут production code, а их совместные parsing/concurrency тесты создаются на `012`.
3. Metrics добавляются только после готового endpoint в `012`; этот же этап закрывает backend независимым review до начала frontend.
4. Frontend bootstrap/client/state объединены в `013`; bbox/card/gallery — в `014`, потому что их корректность проверяется вместе на реальном rendered size. Viewer/accessibility остаются отдельным polish gate `015`.
5. Production packaging и README объединены в `017`, поскольку документация должна описывать фактические контейнеры. Финальная приёмка `018` всегда отдельная и не реализует новые функции.

## Quality gates

Implementation-этапы пишут только production code. Тесты создаются и запускаются
пакетно на backend gate 012 и frontend gate 015:

1. Claude Code выполнил один конкретный prompt и показал итоговый diff.
2. Codex просмотрел фактический код, а не только отчёт Claude.
3. На code-only этапе прошли formatter, затронутый build/type compile и `git diff --check`; тесты не создавались и не запускались.
4. Не осталось executable, coverage, temp DB или других build artifacts.
5. Все backend unit/integration/race/smoke проверки создаются и запускаются на `012`; findings закрываются отдельным `012.1` при необходимости.
6. Все frontend bbox/reducer/component/accessibility проверки создаются и запускаются на `015`; findings закрываются отдельным `015.1` при необходимости.
7. Обязательные по ТЗ тесты не удаляются, а откладываются до соответствующего block gate.
8. Только после закрытия замечаний статус меняется на `DONE`.

## Обязательная repository hygiene до публичной отправки

Эти пункты не меняют функциональный порядок промтов, но должны быть закрыты до публикации:

1. Бинарник `backend/api` был добавлен в commit `3a99ee3`, а затем удалён. Он остаётся в Git history. До push/public submission историю нужно переписать или собрать чистую ветку без бинарника.
2. Добавить `/backend/api` и другие локальные build outputs в `.gitignore`, чтобы проблема не повторилась.
3. `dataset/` сейчас игнорируется, но ТЗ требует запуск из чистого клона с seed/ingest. Размер dataset около 41.6 MiB, поэтому его можно закоммитить один раз обычным Git, если лицензия задания это разрешает. Иначе README должен содержать воспроизводимый способ получить dataset; без одного из этих вариантов clean-clone requirement не выполнен.
4. Не изменять и не индексировать `dataset/predictions.db`; приложение открывает его read-only.

## Запуск текущего этапа

```text
/clear
/implement-task @.claude/prompts/009-seed-client.md
```
