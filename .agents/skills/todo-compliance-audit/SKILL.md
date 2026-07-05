---
name: todo-compliance-audit
description: Performs a strict read-only, requirement-by-requirement audit of the Scout repository against every detail in TODO.md, including bonus features, recommended stack, runtime, tests, clean-clone delivery, and public-repo evidence.
disable-model-invocation: true
---

Perform a strict read-only compliance audit of the current Scout repository.
Do not edit files, generate fixes, commit, push, or treat prior prompts/reports as
proof. The audit target is the current working tree; separately evaluate whether
the committed/public submission contains the same implementation.

The user requires maximal compliance: every atomic detail in `TODO.md` must be
implemented or explicitly permitted by the assignment. A green test suite,
`README.md`, roadmap status, previous review, or plausible-looking code is never
sufficient by itself.

## Sources and precedence

Read completely before judging:

1. `TODO.md` — primary assignment and acceptance source;
2. `openapi.yaml` — exact Data API contract;
3. `AGENTS.md` — repository constraints, not a replacement for the assignment;
4. `README.md` — claimed setup/architecture/behavior that must be verified;
5. relevant production code, configuration, tests, Dockerfiles, Compose files,
   dependency manifests, and Git state.

If sources conflict, report the conflict. `TODO.md` wins for product scope;
`openapi.yaml` wins for the Data API wire contract. Do not let later roadmap or
prompt text weaken either source.

## Non-negotiable audit rules

- Decompose every sentence, bullet, parenthetical qualifier, table row, code block,
  and cross-reference in `TODO.md` into atomic requirements. Split compound bullets.
- Include deliverables, required behavior, bonus-map details, tests, stack,
  operational qualities, runtime limits, repository delivery, dataset constraints,
  pagination/volume assumptions, and API semantics.
- Classify each requirement as `required`, `strong bonus`,
  `strong recommendation`, `allowed alternative`, `constraint`, or `deliverable`.
- In this maximal audit, bonus and recommended items also prevent `STRICT PASS`.
  An assignment-authorized substitution may pass only when the replacement is
  implemented and the reason is documented as requested.
- When a bonus feature is claimed as implemented, every subrequirement of that
  feature is mandatory for that feature to pass. Example: the greenhouse map must
  be 40×40 m, use real `x,y`, use a Konva canvas with zoom/pan, show prediction
  locations, filter photos near a clicked spot, and share class-filter state.
- Preserve qualifiers. Examples: same prediction must satisfy both filters;
  responses still contain all predictions; bounding boxes are normalized;
  DPR changes requested pixels, not CSS overlay coordinates; the catalog is much
  larger than 50; MinIO is local but object storage is external in production.
- Never infer implementation from filenames, comments, TODO status, reports, or
  test names. Inspect the executing production path.
- Never infer correctness from code inspection alone when behavior can be tested.
- Never mark an unrun, inaccessible, flaky, externally unverifiable, or ambiguous
  requirement as passing. Use `NOT PROVEN`; it fails the strict verdict.
- Never silently mark a requirement `N/A`. Use it only when the source itself makes
  the requirement inapplicable and explain why.
- Do not dilute missing details into broad summaries. Every atomic requirement
  must retain its own ledger row and final status.

## Status model

Use exactly these statuses:

- `PASS` — production implementation exists, matches the contract, and has adequate
  direct evidence;
- `PARTIAL` — some sub-behavior exists but at least one material detail is missing;
- `FAIL` — implementation or observed behavior contradicts the requirement;
- `NOT PROVEN` — evidence could not be obtained or verification was not run;
- `N/A` — source text explicitly makes it inapplicable, with justification.

Only `PASS` counts toward strict completion. `PARTIAL`, `FAIL`, `NOT PROVEN`, and
unjustified `N/A` all prevent `STRICT PASS`.

## Phase 1 — establish repository truth

Run non-destructive Git inspection first:

```bash
git status --short
git branch --show-current
git log -5 --oneline --decorate
git remote -v
git diff --stat
git diff --check
```

Record:

- branch/HEAD and dirty files;
- whether audited changes are committed;
- whether local HEAD is ahead/behind its upstream;
- public-origin claim and whether anonymous remote access can be proven;
- generated/build/runtime artifacts accidentally tracked or left unignored.

Audit current tracked plus uncommitted source as the local implementation. Do not
claim clean-clone/public-submission compliance for uncommitted or unpushed changes.

## Phase 2 — build the atomic requirement ledger

Before evaluating code, create stable IDs grouped by source section, for example:

```text
DELIVERY-001
DATA-001
THUMB-001
GALLERY-001
MAP-001
TEST-001
STACK-001
ERROR-001
LOG-001
METRIC-001
RUNTIME-001
REPO-001
DATASET-001
API-001
```

For every ledger item record:

- ID;
- normative class;
- exact source location in `TODO.md` or `openapi.yaml`;
- one atomic paraphrase preserving thresholds and qualifiers;
- expected evidence type: code, test, runtime, documentation, external delivery,
  or a combination.

Do not begin the final verdict until every source section has been accounted for.
Use targeted searches after the ledger exists; this audit is explicitly broad, so
the normal narrow-task restriction does not prohibit repository-wide inventory.

## Phase 3 — prove implementation paths

For each ledger item trace evidence through all applicable layers:

1. entry point or configuration;
2. production implementation;
3. data/error/cancellation/resource path;
4. direct test or runtime observation;
5. operator/user documentation when required.

Use file and tight line references. A requirement with only documentation or only
a mock/test implementation is not `PASS`.

At minimum inspect these domains completely:

### Ingest and Data API

- presigned PUT operation and exact OpenAPI response/error behavior;
- rerunnable seed uploads every dataset image through the public API with photo ID
  keys, bounded work, useful failures, and no direct storage shortcut;
- `GET /photos` cursor pagination, page-size validation, stable ordering, no total
  cap, class/confidence same-prediction semantics, and all-predictions response;
- `GET /photos/{id}` and original URLs;
- direct read-only use of `predictions.db`, schema mapping, parameterized queries,
  bounded result work, and no replacement database.

### Thumbnail engine

- on-demand interface covers requested size, DPR, and quality;
- strict parsing, bounds, aspect ratio/no unintended distortion, cancellation, and
  output headers;
- originals come from object storage;
- responsive gallery requests the actual widths/DPRs it needs;
- identical expensive work is deduplicated;
- cache hit avoids generation/decode;
- bounded concurrency, memory, disk cache, atomic writes, eviction, and cleanup;
- CDN-friendly stable keys, `Cache-Control`, ETag/304 behavior, and cross-continent
  delivery rationale.

### Gallery and viewer

- scrolling cursor-paginated responsive grid;
- `srcset`/`sizes`, lazy loading, bounded rendered/requested data;
- normalized bbox math aligned to the actual rendered image at every size/aspect
  ratio and DPR;
- class/confidence filters and same-prediction server semantics;
- displayed counts/boxes respect UI confidence behavior without mutating API data;
- full-size viewer, prediction inspection, loading/empty/error/broken-image/retry
  states, navigation, focus, keyboard, and responsive behavior.

### Greenhouse map

- feature is present because maximal compliance includes the strong bonus;
- 40×40 m floor plan;
- every displayed photo uses its real database `x,y`, not random/fixed assignment;
- `react-konva`/Konva canvas is used, or an assignment-authorized substitution is
  explicitly justified and otherwise fully equivalent;
- bounded zoom and pan;
- prediction locations are visible;
- clicking a spot selects photos within a documented/tested definition of “near”;
- class filter is shared with the gallery; confidence behavior is consistent;
- map does not crawl the unbounded catalog or pretend a bounded page is complete.

### Required tests

- bbox coordinate transform—the assignment's crux;
- thumbnail request parse/validate and coordinate/dimension math;
- a meaningful key component or reducer;
- real ingest-then-read backend smoke;
- tests execute production paths and assert behavior rather than existence/no panic;
- clean-clone commands actually discover and run the tests.

### Stack and code constraints

- frontend is React and TypeScript;
- backend language is allowed, with preferred choices and substitutions recorded;
- recommended React/Vite/pnpm/Redux Toolkit/openapi-typescript/react-konva/
  CSS Modules/Vitest/feature folders/interface-over-type/no-any/no-magic-number
  items are each audited separately;
- every swap is explained as requested;
- dependencies are actually used and lockfiles are reproducible.

### Errors, logs, and metrics

- centralized typed backend error shape and correct 4xx/5xx status mapping;
- no leaked stack traces, credentials, API keys, signed URLs, or internals;
- UI never falls to blank/broken screens and has real loading/empty/error states;
- structured logs, correlation/request ID traceability, sane levels, no secrets;
- `/metrics` provides rate, latency, errors, thumbnail cache hit/miss, and generation
  time with bounded-cardinality labels and truthful semantics.

### Runtime, delivery, and repository

- service topology fits approximately 1 vCPU and 512 MB–1 GB, with external object
  storage treatment understood;
- pools, goroutines, bodies, decoded images, workers, cache, DOM and page requests
  are bounded for a catalog far larger than 50;
- public GitHub repository is anonymously reachable;
- a clean clone has complete source, dataset, lockfiles, configuration examples,
  seed/ingest instructions, and no missing local state;
- documented commands reproduce build, startup, health, seed, gallery, and tests;
- originals are served from object storage rather than `dataset/images`.

## Phase 4 — verification

Run checks in proportion to a **full final audit**, not a localized change. Prefer
documented project commands, then fill gaps. At minimum attempt:

```bash
go test -race ./...
go vet ./...
go build ./...
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend test --run
pnpm --dir frontend typecheck
pnpm --dir frontend lint --max-warnings=0
pnpm --dir frontend build
pnpm --dir frontend generate:api
git diff --exit-code -- frontend/src/entities/api/__generated__/schema.ts
docker compose config
```

Use the repository's actual working directories and commands. Do not hide failures
with retries or weaker flags. Record every command, exit status, and material
warning.

For full runtime/clean-clone proof:

- use an isolated temporary clone/worktree and unique runtime resources when safe;
- do not overwrite `.env`, volumes, ports, containers, or the user's working tree;
- do not stop or delete an existing stack;
- obtain approval before destructive cleanup or collision-prone runtime actions;
- validate health, seed rerun, ingest→read, thumbnail generation/cache behavior,
  API filters/pagination, frontend proxy, and a representative browser flow;
- clean up only ephemeral resources created by this audit.

If Docker, browser, network, credentials, public GitHub, resource measurement, or
clean-clone execution is unavailable, mark the affected rows `NOT PROVEN`. Never
convert environmental inability into `PASS`.

Inspect tests as evidence even when they pass: confirm assertions cover production
behavior and cannot pass vacuously. Check real runtime behavior when mocks cannot
prove integration.

## Phase 5 — adversarial consistency checks

Before reporting, explicitly search for contradictions:

- README/roadmap claims checks or features absent from code;
- optional/bonus wording used to conceal missing strict-scope work;
- map UI that ignores `x,y`, Konva, zoom/pan, near selection, or shared state;
- filters that select photos correctly but display stale counts/boxes;
- list limits misused as total catalog caps;
- unbounded page accumulation, image decoding, cache growth, workers or DOM;
- tests that mock away the production path they claim to prove;
- local-only files, dirty changes, ignored dataset/config, unpushed commits;
- generated types differing from `openapi.yaml`;
- secrets or signed URLs in Git history, logs, metrics, DOM, tests or artifacts;
- runtime/docs using different ports, image tags, endpoints, keys or commands.

## Final report

Read and follow [the report contract](references/report-contract.md). Produce the
full atomic ledger even when it is long; the user explicitly requested every
detail. Do not provide fixes unless the user asks for them after the audit.

The only successful overall verdict is:

```text
STRICT PASS — every atomic requirement is PASS
```

Otherwise return:

```text
STRICT FAIL — <passed>/<total> atomic requirements PASS
```

List all non-PASS requirements and all verification gaps. Never write “fully
compliant”, “everything is done”, or equivalent unless the ledger has zero
`PARTIAL`, `FAIL`, and `NOT PROVEN` rows and clean-clone/public delivery is proven.
