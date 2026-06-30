# Goal

Describe one concrete result in 1–3 sentences.

# Context manifest

The prompt is self-contained. Carry forward the few verified facts needed from
earlier stages instead of asking Claude to reread completed prompts.

Read before editing:

- `CLAUDE.md`;
- only the exact production source files needed for a code-only task; block-gate
  prompts may additionally name the exact test files they own;
- only named sections or component names from `README.md`/`openapi.yaml` when
  their contract is relevant.

Do not use broad instructions such as `read all files under ...`. Do not read
previous prompt files. Start with `git status --short` and a scoped diff.

List the relevant existing behavior and symbols here so Claude does not need an
open-ended repository exploration. Do not ask Claude to restate the task.

# Context budget

- Use targeted search and read the smallest coherent sections needed.
- Do not inspect unrelated directories or launch subagents for a localized task.
- During a code-only stage, format and compile only. Run tests and the complete
  quality gate only in an explicitly designated block-gate prompt.
- If the scoped context proves insufficient, inspect the directly connected file
  and state why; do not widen to the whole repository automatically.

# Scope

List exact files Claude may create or change. Allow adjacent files only when a
demonstrated blocker requires them.

# Requirements

Copy the relevant task requirements here in compact form. Reference exact
OpenAPI operation/component names instead of requiring a full-file reread.

# Out of scope

List what must not be implemented in this task.

# Acceptance criteria

Describe observable conditions that mean the task is complete.

# Verification

State whether this is a code-only implementation stage or an explicit block gate.

## During implementation

For code-only stages, do not create, edit, inspect, or run tests. Format production
files and compile only the affected target. Defer behavioral verification and
review to the named block gate.

## Final gate

- Code-only implementation stage: formatter, affected build/type compilation,
  `git diff --check`, and scoped production diff inspection only.
- Backend block gate (`012`) or frontend block gate (`015`): create the planned
  tests, run the complete relevant test/race/type/lint/build/smoke suite, perform
  independent review, and record findings for an optional `.1` correction prompt.

Never add opportunistic tests to a code-only stage.

# Final report

Keep the report compact: changed files, key decisions, verification results and
remaining assumptions. Do not repeat the task specification.
