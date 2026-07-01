# Goal

Finish the required frontend with an accessible large-photo viewer and UI hardening, then create deferred frontend tests and run the complete frontend gate. Prove bbox geometry, shared filter/reducer behavior, gallery/viewer states, accessibility basics, strict typing, lint, and build. Do not implement the optional map or production containers.

# Prerequisite

Run only after 013–014 are implemented, reviewed by Codex at production-diff level, and pass generation/typecheck/build. Start with `git status --short`; if Git is unreadable or a prerequisite contract is absent, stop instead of rebuilding earlier stages.

# Context manifest

Start with a scoped diff. Read:

- `CLAUDE.md`;
- README sections Gallery, Tests, Stack, Additional Requirements, Runtime;
- `frontend/package.json` and production files created by 013–014;
- generated API types only through operations/schemas consumed by the app;
- existing frontend tests only when rerunning a partial 015.

Verified facts:

- backend smoke/review is already complete; do not duplicate backend tests;
- frontend tests were intentionally deferred to this task;
- cards use public thumbnails; the viewer may use `originalUrl` but handles failure safely;
- bbox CSS coordinates depend on the contained rendered image rectangle, not source pixels or DPR;
- only one cursor page is rendered, so viewer navigation stays within that page;
- a separate `/frontend-review` runs in a fresh context after this gate.

Do not read previous prompts, backend code/tests, dataset images, Compose, or Git history. Do not use subagents.

# Dependencies and scope

Add only:

- `vitest`, `jsdom`;
- `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`.

Do not add Playwright/Cypress, axe/snapshot libraries, a UI/dialog package, coverage provider, or production dependency.

Modify production files under `frontend/src/` only for viewer and demonstrated hardening. Create focused frontend tests/setup/config and update frontend scripts/config. Do not modify backend, OpenAPI, Compose, root README, or map.

# Phase 1 — viewer and production hardening

1. Open a card in an accessible native `<dialog>` or small standards-based modal with labelled title, close button, Escape, initial focus, focus restoration, modal semantics, and prevented background interaction. No dialog library.
2. Navigate within the active page only, with bounded Previous/Next, position feedback, and keyboard shortcuts that do not steal input keystrokes. Close/reset cleanly if page/filters remove the selected photo.
3. Load `originalUrl` (or a documented high-resolution thumbnail fallback), preserve aspect ratio with contain behavior, reserve stable space, and show loading/error/retry. Never expose the API key or signed URL in UI/logs/errors.
4. Overlay predictions on the exact contained image rectangle including horizontal/vertical letterbox offsets. Reuse task-014 geometry; do not fork bbox math. DPR never changes CSS overlay coordinates.
5. Show class label, confidence, and color/box association in a readable prediction list. Inspection must not depend on hover/color.
6. Harden card semantics, focus, mobile dialog, scroll containment, reduced motion, long errors, and broken images without redesigning correct architecture.
7. Keep selection local or minimally shared based on ownership. Never copy API arrays into Redux or fetch the entire catalog for viewer navigation.

# Phase 2 — deferred focused tests

8. Configure Vitest/jsdom, Testing Library cleanup, jest-dom matchers, and `test`/`test:watch` scripts. Tests are deterministic/offline and mock only the network boundary.
9. Test bbox math—the assignment crux: normalized corners to pixels, same aspect, horizontal/vertical contain offsets, nonzero origin, fractions, defensive invalid geometry, and DPR independence.
10. Test thumbnail candidates: encoded/canonical URLs, actual output-width descriptors, deduplication, endpoint bounds, and stable `sizes`/lazy attributes. Do not test browser selection internals.
11. Test a key reducer/state flow: class/confidence update/reset, cursor-history reset on filter changes, and bounded Previous/Next. Prefer pure helpers over brittle details.
12. Test gallery essentials with concise fixtures: initial loading, populated cards/overlays, empty/reset, typed error/retry, background fetch, and broken thumbnail. Do not duplicate backend validation matrices.
13. Test viewer open/close, role/name, focus restoration, Escape, bounded navigation, prediction details, original failure/retry, and selection invalidation after page/filter change.
14. Assert accessibility through semantic queries, labels, keyboard/focus, alt text, and valid interaction nesting. Do not use giant snapshots or claim jsdom is a visual audit.
15. Test only security-critical config/error behavior: missing config is visible, API key never enters normalized errors/thumbnail URLs, and backend request ID remains available.
16. Use small fixtures/fake boundaries. No dataset, live API/MinIO, sleeps, randomness, network, or browser E2E.

# Phase 3 — full frontend gate

17. Regenerate OpenAPI types deterministically. Generated code may differ from handwritten style; never manually rewrite it for `interface over type`.
18. Run full commands once after focused failures are fixed. Make only narrow production fixes demonstrated by tests/type/lint/build and report them.
19. ESLint covers handwritten TS/React with zero warnings, hooks rules, and no explicit `any`. A narrow documented generated-file ignore is allowed; do not weaken strictness globally.
20. Remove unused dependencies, demo assets, debug logs, stale placeholders, commented experiments, uncleaned listeners/object URLs, and build/test artifacts.
21. Perform a concise source-level review of keyboard order, focus visibility, modal behavior, alt/loading/error announcements, mobile overflow, and reduced motion. Record what still needs a real browser; never claim unperformed visual verification.
22. Do not invoke `/frontend-review` in this context or make broad speculative refactors.

# Out of scope

- Map/Konva, proxy/Docker/Nginx, PWA/SSR, clean-clone docs, deployment, backend tests, live E2E, Git-history cleanup, coverage targets, visual snapshots, or product extras.

# Acceptance criteria

- A user can open, inspect, navigate, and close a large photo with correct contained overlay using mouse or keyboard.
- Bbox math, one key state flow, gallery states, viewer, and security-sensitive config/errors have concise deterministic tests.
- API generation, tests, strict typecheck, zero-warning lint, and production build pass.
- No unbounded catalog work, leaked key/signed URL, artifacts, or map work appears.

# Verification

After implementation stabilizes:

```bash
pnpm --dir frontend install
pnpm --dir frontend generate:api
pnpm --dir frontend test -- --run
pnpm --dir frontend typecheck
pnpm --dir frontend lint -- --max-warnings=0
pnpm --dir frontend build
git diff --check
git status --short
```

If scripts already include fixed flags, use the nonduplicated equivalent and report it. Remove `dist`, coverage, screenshots, caches, and temp artifacts unless ignored.

After success, stop and report:

```text
/clear
/frontend-review
```

If that later review finds actionable issues, create `015.1-frontend-hardening.md` from actual findings. Do not pre-create it.

# Final report

Report viewer/accessibility behavior, production/test files, coverage areas, demonstrated fixes, dependencies, exact gate results, manual-review limitations, and reviewer command. Do not begin 016 or 017.
