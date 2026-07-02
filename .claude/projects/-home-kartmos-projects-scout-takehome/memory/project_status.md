---
name: project-status
description: Current task completion status and what's awaiting user action
metadata:
  type: project
---

Task 016.1 (greenhouse map redesign) DONE (2026-07-02); 61 focused tests pass (mapAssignment: 9, GreenhouseMap: 30, GalleryPage: 22); typecheck clean, lint zero errors, production build passes, web container healthy at http://localhost:8090. Awaiting commit authorization.

**Why:** 016.1 replaced the rejected SVG/zoom/pan/cluster map with a schematic HTML/CSS drawer (hidden/compact/expanded), deterministic photo assignment via djb2 hash, bounded 200-photo query, point/class/confidence filter intersection, viewer navigation scoped to selected point.

**How to apply:** Do not commit or push until user explicitly authorizes. The complete frontend/backend test suites were intentionally not run per the task's proportionate verification requirement.
