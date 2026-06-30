---
name: frontend-review
description: Reviews Scout React and TypeScript changes for correctness and assignment compliance.
disable-model-invocation: true
---

Perform a read-only review of the current frontend diff. Do not edit files.

Check:

- React and TypeScript correctness, including no unjustified `any`;
- compliance with generated API types and filter behavior;
- responsive pagination and `srcset`/`sizes` usage;
- bounding boxes mapped to the rendered image at every size and DPR;
- full-size viewer behavior;
- loading, empty, error and broken-image states;
- shared filter state if the optional map exists;
- accessibility basics and obvious UX regressions;
- missing component, reducer or coordinate-math tests;
- unnecessary complexity or dependencies.

Return actionable findings ordered by severity. Include file and line references. If no problems are found, state what was checked and any remaining manual verification gaps.
