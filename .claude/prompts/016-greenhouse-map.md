# Goal

Implement the optional greenhouse map as a second view of the current paginated
photo result. Use a lightweight accessible React/SVG implementation with shared
class/confidence filters, a fixed 40×40 metre plane, bounded current-page data,
zoom/pan, overlap clustering, photo details, and entry into the existing
`PhotoViewer`. Do not add a map library or change the backend/API contract.

# Verified baseline

- Work on branch `codex/016-greenhouse-map`. Start with `git status --short`, record
  `HEAD`, and inspect a scoped diff. Preserve any user changes.
- The app has one `GalleryPage`; it owns cursor pagination, current-page API data,
  shared class/minimum-confidence filters, selected photo ID, and `PhotoViewer`.
- `GET /photos` is cursor-paginated and the catalog is unbounded. The map must not
  crawl every page. It displays only `currentData.items` from the current page and
  clearly says so.
- Each `Photo` contains `x` and `y` in `[0,40]` metres and non-negative camera
  height `h`. The greenhouse is one fixed plane, not an API resource.
- The frontend already has shared class labels/colors and a pure
  confidence-aware visible-predictions helper. Reuse them rather than inventing a
  second filter interpretation.
- The existing high-contrast class palette, grouped detection summaries, bbox
  behavior, fresh-photo viewer lifecycle, and filter controls are accepted. Do not
  regress them.
- No router or canvas dependency is installed. This feature does not require one.
- Do not commit, push, rewrite history, edit the dataset, or change Docker/runtime
  topology.

# Context manifest

Read before editing:

```text
CLAUDE.md
openapi.yaml: introductory 40×40/x/y/h description and Photo schema only
frontend/src/app/App.tsx
frontend/src/app/App.module.css
frontend/src/pages/gallery/GalleryPage.tsx
frontend/src/pages/gallery/GalleryPage.module.css
frontend/src/features/filters/FilterControls.tsx
frontend/src/features/filters/filtersSlice.ts
frontend/src/features/viewer/PhotoViewer.tsx
frontend/src/entities/photo/classColors.ts
frontend/src/shared/lib/predictions.ts
frontend/src/shared/lib/thumbnailCandidates.ts
frontend/src/test/GalleryPage.test.tsx
frontend/src/test/PhotoViewer.test.tsx                     # viewer opening/focus helpers only
frontend/package.json
README.md: product overview, frontend behavior, and known limitations only
.claude/prompts/ROADMAP.md: task 016 and next-stage section only
compose.yaml: service `web` only
```

Inspect one directly connected file if a demonstrated compile/test blocker
requires it. Do not reread prior prompts, recursively inspect unrelated folders,
or use subagents.

# Scope

Create a cohesive map feature under:

```text
frontend/src/features/map/GreenhouseMap.tsx
frontend/src/features/map/GreenhouseMap.module.css
frontend/src/features/map/mapGeometry.ts                 # only pure geometry/clustering helpers
frontend/src/test/GreenhouseMap.test.tsx
frontend/src/test/mapGeometry.test.ts                    # only if pure helper coverage is useful
```

Modify only when needed:

```text
frontend/src/pages/gallery/GalleryPage.tsx
frontend/src/pages/gallery/GalleryPage.module.css
frontend/src/features/viewer/PhotoViewer.tsx             # only trigger typing/focus compatibility if required
frontend/src/test/GalleryPage.test.tsx
frontend/src/test/PhotoViewer.test.tsx                   # only if viewer trigger contract changes
README.md
.claude/prompts/ROADMAP.md
```

Do not change backend, OpenAPI, generated API types, RTK Query transport, Redux
filter shape, dataset, thumbnail endpoint/cache, Compose/Docker definitions, or
existing gallery/card/filter visual behavior. Do not add `konva`, `react-konva`, a
router, a GIS package, or any dependency.

# Product structure

## View switch

1. Add an accessible compact segmented switch above the result area and below the
   shared filters:

   ```text
   [ Gallery ] [ Greenhouse map ]
   ```

2. Keep the selected view in local `GalleryPage` UI state. Do not add it to Redux
   and do not introduce routes.
3. Use native buttons with `aria-pressed` or equivalent tab semantics. Keyboard
   and visible focus states are required.
4. Switching views must reuse the same fulfilled `currentData`; it must not issue a
   second list request, reset pagination, clear filters, or lose the current page.
5. Existing initial loading, background fetching, API error, empty state, and
   pagination remain shared. Pagination changes the page shown by either view.

## Bounded-data disclosure

1. The map receives `currentData.items` through props and performs no API request.
2. Show a concise status above/beside the map, for example:

   ```text
   Showing 24 photos from page 1
   ```

   Use the actual item count and page number, with correct singular wording.
3. Do not imply that the visible markers represent the entire catalog or entire
   greenhouse history.
4. Do not preload the next page or iterate cursor tokens in the background.

# Greenhouse plane and geometry

1. Render a responsive SVG map of a fixed `40 m × 40 m` plane. Use a neutral dark
   background consistent with the application, a visible greenhouse boundary, and
   grid lines/tick labels every `5 m`.
2. Treat `x` as increasing left-to-right and `y` as increasing bottom-to-top. SVG
   screen Y increases downward, so implement and test the explicit conversion:

   ```text
   mapX = x
   mapY = 40 - y
   ```

3. Clamp/ignore malformed non-finite or out-of-range coordinates safely. One bad
   photo must not break the entire map. If it is omitted, expose an unobtrusive
   accessible count such as `1 photo has invalid coordinates`.
4. Do not invent greenhouse beds, walls, camera orientation, or floor-plan details
   absent from the data.
5. Show `h` only in photo details as camera height; do not create a 3D map.

# Zoom and pan

1. Initial/reset view fits the complete `0–40` plane.
2. Support:
   - mouse-wheel zoom centred near the pointer;
   - pointer drag to pan while zoomed;
   - touch/pointer input without scrolling the page accidentally during an active
     map gesture;
   - visible `+`, `−`, and `Reset view` buttons for mouse, touch, and keyboard users.
3. Use bounded zoom, for example `1×` through `6×`. Clamp pan so the viewport cannot
   lose the greenhouse completely.
4. Use pointer capture during drag and always release/reset drag state on pointer
   up/cancel. Do not install global listeners that leak after unmount.
5. Keep marker hit areas and labels usable at every zoom. Marker visual/hit size
   should remain approximately constant in screen pixels using inverse zoom sizing
   or an equivalent simple approach.
6. Respect `prefers-reduced-motion`; no animated zoom is required.

# Markers, colors, and visible detections

1. One photo contributes one marker before overlap clustering.
2. Determine each photo's marker from its confidence-filtered visible predictions:
   - if a class filter is selected, use that class's exact shared color;
   - otherwise use the class color of the highest-confidence visible prediction;
   - if no visible prediction exists, use a neutral accessible marker color.
3. Reuse the exact current palette from `classColors.ts`; do not copy a second map
   of HEX values into the map feature.
4. Give markers a contrasting outline/halo and a minimum comfortable hit target.
   Selection must not rely on color alone: use an outline, shape, or selected ring.
5. An accessible marker name includes at least the captured date/time, coordinates,
   and number of visible detections.
6. Hover/focus may show a compact tooltip with:
   - capture date/time;
   - `x`, `y`, and `h` in metres;
   - count of confidence-eligible detections;
   - strongest eligible confidence when present.
7. Avoid putting presigned `originalUrl` values in labels, logs, test output, or
   DOM diagnostics.

# Nearby points and clusters

1. Photos at the same or visually overlapping/nearby screen positions must not
   silently cover each other.
2. Implement a small deterministic pure clustering helper suitable for at most the
   bounded page size. O(n²) is acceptable at 24–200 items; a GIS dependency is not.
3. Cluster using a screen-space-equivalent radius that shrinks in metres as zoom
   increases, so nearby points can separate after zooming in. The result must be
   stable for the same inputs regardless of incidental render timing.
4. A cluster marker shows its photo count and has an accessible name such as
   `7 photos near x 12, y 8`.
5. Clicking/activating a multi-photo cluster selects it and shows its member list in
   the details panel. Optionally zoom one step toward the cluster, but never make
   zooming the only way to access its members.
6. Clicking near an isolated marker should select the nearest marker within the
   marker hit radius. Clicking empty map space clears selection without opening the
   viewer.
7. A cluster list uses deterministic newest-first photo ordering, with a stable ID
   tie-breaker.

# Photo details and existing viewer integration

1. On desktop, show a compact details panel beside the map. On narrow screens,
   place it below the map as a bounded card/drawer-like region; do not cover the
   entire map permanently.
2. For one selected photo show:
   - thumbnail using the existing thumbnail candidate utilities;
   - capture date/time;
   - coordinates `x`, `y` and camera height `h` with metre units;
   - grouped confidence-eligible detections using shared labels/colors;
   - `Open photo` button.
3. For a selected cluster, show a scrollable member list with date/time, coordinates,
   and eligible detection count. Selecting a member reveals its details; every
   member remains reachable by keyboard.
4. `Open photo` launches the existing `PhotoViewer` for that photo while preserving
   the current page array for previous/next navigation.
5. Preserve viewer fresh-URL/refetch behavior. The map must never use the stale list
   `originalUrl` as its full-resolution image source.
6. Preserve focus restoration: closing the viewer returns focus to the map control
   that opened it. If the existing trigger type accepts only `HTMLElement`, make the
   narrowest safe type adjustment needed for an SVG/native map trigger; do not
   weaken unrelated types to `any`.
7. Clear map selection when its photo disappears after a page/filter change.

# Accessibility and responsive behavior

1. SVG markers/clusters must be keyboard reachable and activatable with Enter and
   Space. Use clear focus rings and selected state.
2. Provide a concise map region label and instructions without announcing every
   pointer movement through a live region.
3. Native zoom/reset/view buttons must have explicit accessible names.
4. Do not create a keyboard trap. Tab order should move through toolbar, markers or
   cluster items, details controls, pagination, and back out naturally.
5. Maintain a practical map height across desktop, tablet, and mobile; avoid a tiny
   letterbox or a map taller than the viewport with inaccessible controls.
6. Tooltip-only content is supplemental. All actionable information must also be
   available through focus/selection and the details panel.

# State and performance constraints

1. Keep view, transform, hover, and selection state local to the gallery/map feature.
   Shared class and confidence filters remain in Redux exactly as they are.
2. Memoize derived valid markers/clusters where it improves clarity, but avoid
   premature caching or mirrored state.
3. Do not request thumbnails for every marker. Load a thumbnail only for the
   selected photo/details item.
4. Do not render `PhotoCard` components inside the SVG or cluster markers.
5. No polling, background prefetch, full-catalog accumulation, or hidden page cache.

# Documentation bookkeeping

After implementation and successful verification:

1. Update README briefly:
   - mention Gallery/Greenhouse map views;
   - state that the map is a 40×40 m current-page visualization using `x/y`;
   - document shared filters, clustering, zoom/pan, and viewer opening;
   - remove the obsolete `No optional greenhouse map` limitation;
   - explicitly document that the map does not fetch all cursor pages.
2. Update only task 016 and the next-stage paragraph in `ROADMAP.md`:
   - change 016 from `OPTIONAL` to `DONE` only after the checks below pass;
   - record SVG/no-new-dependency implementation rather than the old planned
     `react-konva` technology;
   - do not rewrite completed task history or claim a full suite that was not run.

# Focused tests only

Add proportionate tests for this feature; do not rerun or rewrite the complete
frontend suite.

## Pure geometry

Cover the highest-risk math:

- `(0,0)`, `(40,40)`, and one interior coordinate map to the expected SVG plane;
- invalid/out-of-range coordinates are rejected or clamped according to the chosen
  explicit policy;
- zoom stays within bounds and reset restores the complete plane;
- deterministic nearby points cluster at base zoom and separate when sufficiently
  zoomed;
- cluster member ordering is stable/newest-first.

## GreenhouseMap component

Cover at minimum:

- 40×40 grid/region and current-page disclosure render;
- valid photos produce accessible markers and invalid coordinates do not crash;
- marker selection exposes x/y/h and eligible detection information;
- cluster activation exposes all member photos;
- keyboard activation works;
- zoom buttons and reset update/restore the view;
- `Open photo` calls the supplied selection callback with the correct photo and
  focus trigger.

## Gallery integration

Add only the minimum integration assertions proving:

- Gallery/Map switching reuses current query data and does not trigger another list
  request;
- active class/confidence filters and page number reach the map;
- pagination changes the bounded map page;
- opening from the map launches the existing viewer for the correct current-page
  index and closing restores focus.

Do not add screenshot/pixel tests, browser automation, canvas mocks, or broad
viewer retry duplication.

# Acceptance criteria

- Users can switch between a gallery and an understandable 40×40 m greenhouse map
  without refetching or losing filters/page state.
- The map accurately places current-page photos using `x/y`, discloses its bounded
  scope, and never crawls the full catalog.
- Nearby/overlapping photos remain discoverable through clusters and a keyboard-
  accessible member list.
- Zoom, pan, reset, marker selection, details, and opening/closing the existing
  viewer work on mouse, touch/pointer, and keyboard paths.
- Shared confidence/class semantics and high-contrast colors are reused exactly.
- Invalid coordinates and photos without eligible detections fail locally, not as a
  broken map.
- No new runtime dependency or backend/API/runtime change is introduced.

# Proportionate verification

Run only the feature and directly affected integration test files:

```bash
pnpm --dir frontend test --run \
  src/test/mapGeometry.test.ts \
  src/test/GreenhouseMap.test.tsx \
  src/test/GalleryPage.test.tsx
```

Omit `mapGeometry.test.ts` only if all pure helper cases are intentionally located
in `GreenhouseMap.test.tsx`. Add `PhotoViewer.test.tsx` only if the trigger/focus
contract actually changes. Do not run Vitest without explicit file arguments.

Then run:

```bash
pnpm --dir frontend typecheck
pnpm --dir frontend exec eslint <only changed frontend source and test files>
pnpm --dir frontend build
git diff --check
git status --short
```

Inspect the scoped diff for bounded-data behavior, no duplicate HEX palette, no
presigned URL leakage, and no accidental dependency/package-lock change. Do not run
backend tests, the complete frontend suite, API generation, or full Docker/runtime
acceptance.

# Final operational step — rebuild and restart only changed containers

After focused tests, typecheck, scoped lint, build, and diff check pass, rebuild and
restart only the changed frontend service:

```bash
docker compose up -d --build --no-deps web
docker compose ps web
```

Do not deliberately rebuild/restart `api`, `minio`, `minio-init`, or `seed`.
Confirm that `web` becomes healthy and the Gallery/Greenhouse map switch is
available at `http://localhost:8090` or the configured `SCOUT_WEB_PORT`. If Docker
is unavailable, do not alter configuration; report the blocker and exact command
the user can run.

# Final report

Report changed files, bounded data scope, SVG geometry/orientation, clustering,
zoom/pan controls, marker/details accessibility, viewer integration, focused test
results, typecheck/scoped lint/build results, documentation bookkeeping, exact Git
status, and final `web` container health. Explicitly state that complete frontend
and backend suites were intentionally not run. Do not commit or push. Finish with
`READY FOR REVIEW`, `BLOCKED`, or `CHANGES REQUIRED`.
