# Task prompts

The current implementation sequence and completion statuses are tracked in [`ROADMAP.md`](./ROADMAP.md).

Use numbered, descriptive filenames. Decimal suffixes such as `004.1` are corrective tasks belonging to the preceding main stage and do not shift the main roadmap numbering.

Keep one implementation task in each file. Start from `TEMPLATE.md`, then run it in Claude Code:

```text
/clear
/implement-task @.claude/prompts/<prompt-file>.md
```

Use one fresh Claude Code context per implementation prompt. Small fixes for the current prompt may stay in the same context. Run backend or frontend review in a separate clean context after each logical block.

## Context-efficient prompts

Prompts from stage 006 onward use a scoped context manifest:

- carry forward verified facts instead of rereading completed prompts;
- name exact production source files rather than whole directories;
- reference only relevant README/OpenAPI sections or component names;
- include a scoped initial diff and avoid repeated full-project inspection;
- use format/build checks only during code-only stages and defer tests to the
  designated block gate;
- avoid task restatements and subagents for localized work.

This reduces repeated input/context tokens while preserving the two explicit
backend/frontend test and review gates.

## Consolidated stages after 007

Related work sharing the same files and mental model may be combined into one
production-code prompt with explicit internal checkpoints. Code-only checkpoints
use formatting and compilation, not tests. Do not split a task merely because it
has several files, and do not merge unrelated risk domains merely to reduce the
prompt count.

Keep separate production prompts for seed, thumbnail core/cache, frontend
foundation/gallery and viewer. Independent review and test creation happen only
at backend gate `012` and frontend gate `015`. Corrective `.1` prompts are created
only after findings at those gates.

## Deferred testing policy

Implementation stages write production code only. They may format and compile the
affected target, but must not create, modify, or run tests. Do not include test
matrices, fixtures, Docker smoke, race checks, repeated runs, lint gates, or
independent review in stages `009`–`011` and `013`–`014`.

Testing and correction happen at two explicit block gates:

1. **Stage 012 — backend gate.** Add and run the assignment-critical backend
   tests, including thumbnail parsing/math, cache duplicate suppression,
   ingest-to-read smoke, race/vet/build, resource checks, and backend review.
   Create `012.1` only for findings from that gate.
2. **Stage 015 — frontend gate.** Add and run bbox geometry, reducer/component,
   gallery/viewer/accessibility tests, type/lint/build checks, and frontend review.
   Create `015.1` only for findings from that gate.

Stages `017` and `018` validate production packaging and clean-clone acceptance;
they do not move block-gate test suites back into every implementation prompt.

Prompt files are a curated record of important tasks. Full chat transcripts do not need to be committed.
