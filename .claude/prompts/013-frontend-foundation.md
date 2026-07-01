# Goal

Create the production-only frontend foundation for Scout: React 19, Vite, strict TypeScript, pnpm, CSS Modules, generated OpenAPI types, Redux Toolkit/RTK Query, typed runtime configuration, shared filters, and a minimal application shell. Do not implement the gallery/viewer or add/run tests.

# Prerequisites — check before editing

1. Run `git status --short`. If Git cannot read `HEAD` or reports a corrupt/empty object, stop and report it. Do not initialize a new repository, rewrite history, delete `.git`, or edit project files while the baseline is unreadable.
2. Check `command -v node && node --version` and `command -v pnpm && pnpm --version` inside Ubuntu WSL. Windows executables under `/mnt/c/Program Files/nodejs` do not satisfy this prerequisite.
3. Require Linux Node.js 24 LTS (`>=24 <25`) and pnpm 10. Pin them with a root `.nvmrc`, `engines`, and exact `packageManager`. If missing, stop before editing and report these user-run steps; do not use `sudo`, `apt`, `curl | sh`, or change the machine automatically:

```bash
# Install nvm from its official repository first if needed, reopen WSL, then:
nvm install 24
nvm use 24
npm install --global pnpm@10
node --version
pnpm --version
```

Do not silently use npm/yarn/Bun or Windows Node.

# Context manifest

Start with the prerequisite checks and a scoped diff. Read only:

```text
CLAUDE.md
README.md: Gallery, Stack, Additional Requirements, Runtime, Data API
openapi.yaml: GET /photos, GET /photos/{photoId}, PhotoPage, Photo, Prediction, BoundingBox, error schemas
backend/internal/httpapi/thumbnails.go
.env.example
.gitignore
```

Verified facts:

- backend through `012.3` is complete and independently reviewed;
- Data API base URL is configurable, requires `X-API-Key`, and returns cursor field `next_token`;
- public thumbnails use `GET /photos/{photoId}/thumbnail?width=&dpr=&quality=` without an API key;
- filters are `classId` and `minConfidence`; the backend matches both on one prediction but returns all predictions;
- the catalog is unbounded; one backend page is at most 200 items;
- task 014 owns gallery/cards/bbox; task 015 owns viewer, tests, accessibility hardening, and the full frontend gate.

Do not inspect backend tests, previous prompts, dataset, Git history, or unrelated backend internals. Do not use subagents.

# Dependencies and scope

Create `frontend/` using:

- runtime: React 19, React DOM 19, `@reduxjs/toolkit`, `react-redux`;
- development: Vite React plugin, TypeScript and React types, ESLint with official TypeScript/hooks/refresh configuration, and `openapi-typescript`;
- CSS Modules and ordinary CSS only.

Resolve compatible versions with pnpm and commit `pnpm-lock.yaml`; never hand-edit it. Do not add React Router, a component kit, Tailwind/CSS-in-JS, Axios, another request/cache library, tests, Konva, or virtualization.

Create or modify only:

```text
.nvmrc
.gitignore
.env.example
frontend/package.json
frontend/pnpm-lock.yaml
frontend/index.html
frontend/tsconfig*.json
frontend/vite.config.ts
frontend/eslint.config.js
frontend/src/
```

Keep generated OpenAPI output in one marked file and never manually edit it. Do not modify `openapi.yaml`, backend, Compose, or README.

# Production contract

1. Scaffold a small Vite React app without demo assets. Add scripts for `dev`, `build`, `typecheck`, `lint`, and `generate:api`; task 015 adds test scripts.
2. Enable strict TypeScript, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, and unused checks where compatible with generated output. No handwritten `any`, unsafe double casts, `@ts-ignore`, or unchecked environment access.
3. Use feature folders with explicit direction, such as `app`, `pages`, `features`, `entities`, `shared`. Avoid a generic dumping-ground `components/`.
4. Generate definitions from root `openapi.yaml` with `openapi-typescript`. The deterministic script must work from `frontend/`; handwritten code consumes generated operations/schemas rather than duplicating DTOs.
5. Configure one Redux store and typed hooks. Keep shared client state in slices, but never mirror RTK Query server data into Redux slices.
6. Add a shared filter slice with optional known `classId` and `minConfidence`, update/reset actions, and all six class IDs plus readable labels. Prefer `interface` for handwritten object shapes; use `type` only for unions/mapped aliases.
7. Add one RTK Query service using `fetchBaseQuery` with typed `listPhotos` and `getPhoto`. Encode only present parameters, preserve opaque cursors, and consume transport field `next_token`.
8. Add one public thumbnail URL builder. Validate/canonicalize typed inputs, encode ID/parameters, and never attach the API key.
9. Read `VITE_SCOUT_API_BASE_URL` and `VITE_SCOUT_API_KEY` through one validated config module. Normalize the base URL and show a visible config error instead of a blank app. Document development examples in root `.env.example` and configure Vite's `envDir` so the existing root `.env` workflow actually supplies them.
10. Treat the browser API key as an exposed local-demo credential: never call it secret storage, log/render it, put it in URLs/Redux/errors, or commit frontend `.env`. Do not invent an auth proxy here.
11. Normalize fetch/OpenAPI failures into a UI-safe shape retaining status, machine code, request ID, and safe message when available. Network/parse failures need useful fallbacks without headers or credentials.
12. Create a semantic `App` shell/provider setup proving config/store/API wiring. It may show a Scout heading and “gallery arrives in task 014”; it must not fetch the catalog.
13. Establish global tokens/reset and CSS Modules for a responsive, keyboard-visible shell. No runtime CDN fonts/assets.
14. Keep the API origin configurable for existing backend CORS. Do not hard-code `localhost` throughout components.

# Out of scope

- Gallery, cards, pagination UI, bbox, images, filter UI, viewer, map, service worker, SSR, Docker, production proxy, or README.
- Tests, fixtures, mocks, Vitest, Testing Library, jsdom, coverage, browser automation, or review.

# Acceptance criteria

- Clean frontend install generates API types and produces a strict build.
- Typed API/state/config/errors/thumbnail foundations are ready for task 014.
- No catalog request is issued by the placeholder and no tests are created or run.

# Verification

Code-only stage: do not create/run tests or run the full lint/review gate.

```bash
pnpm --dir frontend install
pnpm --dir frontend generate:api
pnpm --dir frontend typecheck
pnpm --dir frontend build
git diff --check
git status --short
```

Inspect the scoped diff. Remove/ignore `dist`, caches, and `node_modules`.

# Final report

Report environment versions, files, direct dependencies/reasons, folder/API/state/config decisions, generation/typecheck/build results, and assumptions. Confirm tests/gallery were deferred. Do not begin task 014.
