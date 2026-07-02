---
name: project-status
description: Current task completion status and what's awaiting user action
metadata:
  type: project
---

Task 016.3 (greenhouse map UX hardening) DONE (2026-07-02); 67 GreenhouseMap tests pass; typecheck clean, lint zero warnings, production build passes, web container healthy at http://localhost:8090. Awaiting commit authorization.

**Why:** 016.3 polishes the Konva map UX: centred expanded dialog (fixed/inset:0/margin:auto, min(98vw,1440px)×min(94dvh,960px)), collapsed marker disclosure (Markers (N) ▼/▲), draft/apply workflow in expanded mode (no gallery change until Apply), action bar with preview count, compact stays quick-apply, coordinate readout beside zoom controls.

**How to apply:** Do not commit or push until user explicitly authorizes. Complete frontend/backend suites intentionally not run per proportionate verification.
