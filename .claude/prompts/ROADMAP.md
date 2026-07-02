# Scout implementation roadmap

Этот файл фиксирует фактическое состояние проекта и актуальную последовательность промтов. Он обновляется после проверки каждого выполненного этапа.

## Статусы

- `DONE` — промт выполнен, код и проверки просмотрены Codex.
- `HARDENING` — основная реализация готова, но quality review выявил корректировки.
- `READY` — промт создан и готов к запуску в Claude Code.
- `PLANNED` — этап согласован, но промт ещё не создан.
- `OPTIONAL` — бонусный этап, выполняется после обязательного MVP.

## Фактическое состояние

- Ветка: `main`; текущий принятый commit отправлен в `origin/main`.
- Go: `1.26.4`.
- Backend использует Go standard library и `modernc.org/sqlite v1.53.0`.
- Реализованы HTTP bootstrap, `/healthz`, конфигурация, graceful shutdown, domain models, application errors и SQLite repository.
- HTTP infrastructure этапов 005/005.1 реализована и проверена, включая recovery diagnostics.
- `go test -race ./...` и `go vet ./...` проходят после этапа 005.1.
- Docker Compose и локальный MinIO development environment добавлены; обязательный React/TypeScript frontend реализован и прошёл финальный gate 119/119.

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
| `009-seed-client` | `DONE` | Bounded rerunnable public-API seed client without tests | Production code проверен полным backend gate и integration smoke на 012 |
| `010-thumbnail-core` | `DONE` | Production thumbnail parser/resolver/JPEG generator with bounded concurrency | Parser/generator tests, race и reviewer проходят |
| `011-thumbnail-cache-and-endpoint` | `DONE` | Bounded atomic disk cache, singleflight and public CDN-friendly endpoint | Cache/endpoint/concurrency tests, smoke и reviewer проходят |
| `012-observability-and-backend-gate` | `DONE` | Prometheus metrics, deferred backend tests, race/vet/build, real MinIO smoke and reviewer handoff | Полный gate, smoke и финальный обновлённый backend-review прошли без findings |
| `012.1-backend-hardening` | `DONE` | Bounded unmatched labels, real eviction metric, strict cache keys, correct coalesced accounting, bounded startup index | Повторный review подтвердил исправления; выявлен отдельный leader-context lifecycle gap |
| `012.2-generation-lifecycle-hardening` | `DONE` | Server-bound shared generation context, documented coalesced misses, query/import cleanup | Повторный обновлённый reviewer подтвердил lifecycle/cardinality/cache correctness |
| `012.3-final-backend-cleanup` | `DONE` | Deterministic cancelled-flight test, nil service guards, dead ETag removal | Финальный backend-review подтвердил security, lifecycle, cache atomicity и race cleanliness без findings |

Backend через этап 012.3 полностью реализован, протестирован и проверен независимым reviewer без findings. Все обязательные frontend/runtime этапы через 018.2.1 завершены и проверены. Этап 016.3 (greenhouse map UX hardening) реализован: centred dialog, collapsed marker disclosure, draft/apply workflow, coordinate readout; 67 focused tests pass.

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
| `009` | `DONE` | Seed client | Идемпотентный seed через публичный upload-link API с concurrency default `2`, bounded streaming и exit codes | Нет |
| `010` | `DONE` | Thumbnail core | width/DPR/quality contract, safe bounds, canonical key, no upscale, streaming JPEG resize/encode, cancellation, generation semaphore default `1` max `2` | `golang.org/x/image`; без libvips/ImageMagick |
| `011` | `DONE` | Bounded thumbnail cache and endpoint | Disk cache max bytes + eviction, atomic writes, singleflight, public endpoint, ETag/304 и CDN-friendly Cache-Control | `golang.org/x/sync/singleflight` |
| `012` | `DONE` | Observability and full backend gate | Metrics, полный backend suite, ingest→read→thumbnail smoke, race/vet/build, corrective cycles и финальный независимый review прошли | Prometheus Go client |
| `013` | `DONE` | Frontend foundation | React/Vite/strict TypeScript/pnpm/CSS Modules, generated OpenAPI types, typed RTK Query client, normalized errors и shared filters | Linux Node.js 24 LTS, pnpm 10, Redux Toolkit и openapi-typescript |
| `014` | `DONE` | Responsive paginated gallery | Responsive cursor gallery, filters, bounded current-page rendering, `srcset`/`sizes`, lazy thumbnails, normalized bbox overlay и complete UI states | Новых зависимостей нет |
| `015` | `DONE` | Viewer, UI hardening and full frontend gate | Accessible contained-image viewer, gallery/viewer/accessibility coverage и полный frontend gate | Vitest, jsdom и Testing Library |
| `015.1` | `DONE` | Frontend review hardening | Reviewer findings закрыты; production bbox transform покрыт тестами; malformed-card isolation, valid card semantics, responsive sizes и filter commit hardening завершены | 83/83 tests, strict typecheck, zero-warning lint и production build проходят |
| `016` | `DONE` | Greenhouse map | Konva-canvas 40×40 m floor plan (`react-konva`/`konva`): 3 beds, real x/y markers, zoom 1×–6×, pan, 3 m near-location filter, compact/expanded drawer, bounded ≤200-photo query | Redesigned in 016.1; rewritten to Konva in 016.2; UX hardened in 016.3 |
| `016.3` | `DONE` | Greenhouse map UX hardening | Centred large dialog (fixed inset:0 margin:auto, min(98vw,1440px)×min(94dvh,960px)); collapsed marker disclosure (Markers (N) ▼/▲, aria-expanded, absent from DOM when closed); draft/apply workflow in expanded (draftLocation/draftHighlightedId, no gallery change until Apply); Apply/Cancel action bar with preview count; compact remains quick-apply; coordinate readout beside zoom controls (draft in expanded, applied in compact) | 67 focused GreenhouseMap tests pass; typecheck, zero-warning lint, production build pass |
| `016.4` | `DONE` | Greenhouse map background image | Removed artificial three-bed overlay (Bed 1–3 Rects); added approved 2 MB PNG asset (`frontend/src/assets/greenhouse-map-background.png`) loaded asynchronously via HTMLImageElement effect; rendered as first Konva `Image` node inside transformed layer at 60% opacity, `listening={false}`, filling the 40×40 m plot bounds; load failure degrades gracefully to functional map; all 016.3 behavior preserved | 76 focused GreenhouseMap tests pass (10 new); typecheck, zero-warning lint, production build pass; broader suites and runtime not run (visual-only task) |
| `017` | `DONE` | Production runtime and documentation | Multi-stage API/web images, Nginx/reverse proxy, healthchecks, CPU/RAM/cache-volume limits; internal/public S3 endpoints; external-S3 production topology; clean-clone README, seed/test/run steps, architecture/cache/trade-offs | Закрыт через hardening 017.1/017.2; runtime acceptance остаётся на 018 |
| `017.1` | `DONE` | Production runtime hardening | Seed entrypoint, Vite asset caching/security headers, root-relative URL, versioned image tags, memory headroom, offline presign test и README accuracy | Backend gate, frontend 100/100 и static Compose validation прошли |
| `017.2` | `DONE` | MinIO healthcheck hardening | Custom-port alias для `mc ready` без утечки credentials; default/9099 Compose render | Фактический diff и оба render проверены; seed entrypoint сохранён |
| `018` | `DONE` | Final acceptance | Clean-clone gates, history cleanup, images, ingest/API/proxy, 225-photo pagination и singleflight прошли; runtime findings закрыты в 018.1 | Финальная runtime/browser/resource приёмка пройдена |
| `018.1` | `DONE` | Final acceptance hardening | IPv4 web healthchecks в local/prod, explicit `scout-web:local`, forced eviction (7 evictions, ≤1 MiB), production-limit run (384/64 MiB, 0.75/0.25 CPU confirmed), browser smoke 25/25 PASS | Все runtime-критерии пройдены; final commit разрешён |
| `018.2` | `DONE` | Final product polish | Fresh presigned original URL lifecycle в viewer, percentage confidence input, README/ignore/roadmap cleanup | Gate 129/129 прошёл после hardening 018.2.1 |
| `018.2.1` | `DONE` | Final product polish hardening | Retry независимо от URL rotation, корректный sidebar loading state, нормализация clamped percentage | 129/129 tests, strict typecheck, zero-warning lint, production build, schema без изменений |
| `019` | `PENDING CI` | Multi-platform support | Explicit `--platform=$BUILDPLATFORM` в builder-стадиях обоих Dockerfile, удалены дефолты `TARGETARCH=amd64`/`TARGETOS=linux`; QEMU-enabled Buildx build gate (linux/amd64+linux/arm64, GHA cache, no push) в CI; нативная matrix (ubuntu-latest + ubuntu-24.04-arm) с arch assertion и double-seed в integration; README "Supported platforms" с diagnostic block | Все 6 pinned images подтверждены с amd64+arm64 локально; Buildx builds для обеих платформ локально; ARM64 integration pending GitHub Actions |
| `020` | `READY FOR REVIEW` | Final submission hardening | SQLite `ORDER BY id DESC` (primary-key cursor, no temp B-tree); typed JSON 404/405 via catch-all + per-route handlers; server-side `nearX/nearY/nearRadius` cursor-paginated filter (replaces client-side near filtering limited to 200 markers); thumbnail cache-miss metrics counted before error return; Linux/macOS/Windows Docker prerequisites table in README; `Object.defineProperty` replaces `(globalThis as any)` ResizeObserver stub; named constants for bbox dim opacity, stroke widths, background opacity, axis/legend offsets; task-019 AMD64/ARM64 work preserved | `go test -race ./...`, `go vet`, `go build`, `pnpm test --run` 285/285, `pnpm typecheck`, `pnpm lint`, `pnpm build` all pass |

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

## Repository hygiene

1. История переписана до публичной отправки; исторический бинарник `backend/api` отсутствует в reachable history (завершено).
2. `/backend/api` и другие локальные build outputs закрыты точными правилами `.gitignore` (стареющие комментарии задачи 018 удалены в 018.2).
3. Dataset закоммичен: 50 JPEG и `dataset/predictions.db` воспроизводятся из clean clone.
4. `dataset/predictions.db` не изменяется приложением и открывается read-only; подтверждённая SHA-256: `b84f73a33e99496d1152ef366d914c64a6e60cb72c494c9f2c42bc7b7dcaeb39`.
5. Принятый финальный commit находится в `main` и отправлен в `origin/main`.

## Следующий этап

Все обязательные этапы через 018.2.1 реализованы и приняты. Этап 016 (greenhouse map) полностью переработан через 016.1 и 016.2: Konva-canvas 40×40 m floor plan с реальными x/y координатами из БД, zoom 1×–6×, pan, фильтр ≤3 m, компактный/расширенный drawer, bounded ≤200-photo query. 90 focused tests pass (33 mapGeometry + 36 GreenhouseMap + 21 GalleryPage). Typecheck, lint (zero warnings), production build — все пройдены. Для commit/push требуется разрешение пользователя.
