# Goal

Implement the production-only Scout gallery: shared filter controls, bounded cursor pagination, responsive thumbnail cards, and correct normalized bounding-box overlays at every rendered size and DPR. Include real loading, empty, error, retry, background-fetch, and broken-image states; do not add/run tests or implement the large viewer.

# Prerequisite

Run only after task 013 is complete and its generated types, typecheck, and build pass. Start with `git status --short`; if Git is unreadable or task-013 contracts are missing, stop instead of recreating the foundation.

# Context manifest

Start with a scoped diff. Read only:

```text
CLAUDE.md
README.md: Gallery, Tests, Stack, Additional Requirements, Runtime
frontend/package.json
frontend/src/app/
frontend/src/shared/api/
frontend/src/shared/config/
frontend/src/features/filters/
frontend/src/entities/photo/ (if present)
frontend/src/pages/ (if present)
```

Use generated types; do not reread the full OpenAPI file. Verified contracts:

- bbox corners are normalized `[0,1]` against the original and map to the rendered image area;
- DPR changes requested image pixels, never CSS overlay coordinates;
- public thumbnails accept width `1..2048`, DPR `1..3`, quality `1..100`;
- cursors are opaque, a page is at most 200, and total photos are unlimited;
- backend filters catalog membership on one matching prediction but returns all predictions;
- task 015 owns the accessible viewer and all frontend tests/review.

Do not inspect backend code/tests, previous prompts, dataset, build output, or Git history. Do not use subagents.

# Scope

Modify production files only under `frontend/src/` and the minimum adjacent frontend config required to compile them. Prefer:

```text
frontend/src/pages/gallery/
frontend/src/features/filters/
frontend/src/features/gallery-pagination/
frontend/src/entities/photo/
frontend/src/shared/ui/
```

Add no dependency, especially no virtualization/UI/image/state/request/styling library. Adjust a task-013 contract only for a narrow compile blocker and report it.

# Production contract

1. Replace the placeholder with a semantic gallery page using the existing store, filters, RTK Query service, generated types, error normalizer, and thumbnail builder. Do not duplicate DTOs or fetch server data manually in effects.
2. Render one bounded cursor page at a time with a named page size no greater than 50 (prefer 24). Provide explicit Previous/Next controls and retain only opaque cursor history required to go back; never concatenate/render all visited pages.
3. Reset pagination atomically before a query when class/confidence changes. Disable duplicate navigation during fetch and prevent stale responses from replacing the active filter/page.
4. Build accessible controls for six classes, “all classes”, confidence `0..1`, and reset. Commit confidence changes via select/apply/debounce, not every transient keystroke. Explain active filters compactly.
5. Treat backend filtering as authoritative. Do not client-refilter page membership. You may visually emphasize matching predictions while retaining every returned prediction.
6. Create responsive CSS-grid cards, preserving API width/height aspect ratio and reserving media space before load.
7. Use named thumbnail candidate/quality constants and valid `srcset`/`sizes`. Candidate URLs must describe their real output pixel widths including DPR, be deduplicated, remain within endpoint limits, and let the browser select responsively. Use `loading="lazy"`, `decoding="async"`, useful `alt`, and intrinsic dimensions.
8. Never use `originalUrl` in cards. Thumbnail failure gets a stable fallback and explicit retry without layout collapse or an automatic retry loop.
9. Overlay exactly the rendered image area—not card/caption/original pixels/letterbox. Prefer a same-aspect media wrapper and normalized SVG `viewBox`; keep strokes legible with non-scaling strokes or equivalent.
10. Map every valid `xMin/yMin/xMax/yMax` to rendered coordinates. Clamp only defensive floating drift, never mutate API data, and skip malformed geometry safely. Never multiply CSS coordinates by DPR.
11. Centralize deterministic contrast-aware class colors. Boxes must not hide the image; expose class/confidence in text/accessibility, not color alone.
12. Extract pure bbox/contain geometry and thumbnail-candidate helpers with interfaces/named constants so task 015 can test them. No test-only branches or layout globals.
13. Provide distinct initial skeleton, background-fetch state, empty filtered state/reset, typed API/auth/config error/retry, and nonblank unexpected-error fallback.
14. Define a typed optional card-selection callback/state boundary for task 015. Until a real callback is supplied, keep the card a semantic noninteractive article and render no dead “open” control; task 015 makes the action keyboard reachable when it adds the viewer.
15. Bound rendered photo nodes to the current page. A small RTK Query cache lifetime is fine; never prefetch/follow every cursor.
16. Support narrow mobile through desktop, visible focus, reduced-motion preference, stable pagination, and no horizontal page overflow.

# Out of scope

- Full viewer/dialog, original inspection, map/Konva, infinite auto-pagination, all-catalog prefetch, service worker, packaging, or backend/OpenAPI changes.
- Tests, mocks, fixtures, Vitest/Testing Library, screenshots, automation, accessibility audit, or review.

# Acceptance criteria

- One cursor page renders as a responsive thumbnail grid with correct normalized overlays and responsive image candidates.
- Filters and Previous/Next preserve server semantics and bounded rendering.
- Loading, empty, error/retry, background-fetch, and image-error states are stable.
- Production typecheck/build pass without tests.

# Verification

Code-only stage: do not create/inspect/run tests or the full lint/review gate.

```bash
pnpm --dir frontend generate:api
pnpm --dir frontend typecheck
pnpm --dir frontend build
git diff --check
git status --short
```

Inspect the scoped diff. Do not start recursive pagination or leave `dist`, screenshots, caches, or other artifacts.

# Final report

Report files, pagination/filter transitions, responsive-image policy, bbox geometry, UI states, checks, and assumptions for task 015. Confirm tests/viewer were deferred. Do not begin task 015.
