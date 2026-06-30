# Goal

Describe one concrete result in 1–3 sentences.

# Context manifest

The prompt is self-contained. Carry forward the few verified facts needed from
earlier stages instead of asking Claude to reread completed prompts.

Read before editing:

- `CLAUDE.md`;
- only the exact source/test files needed for this task;
- only named sections or component names from `README.md`/`openapi.yaml` when
  their contract is relevant.

Do not use broad instructions such as `read all files under ...`. Do not read
previous prompt files. Start with `git status --short` and a scoped diff.

List the relevant existing behavior and symbols here so Claude does not need an
open-ended repository exploration. Do not ask Claude to restate the task.

# Context budget

- Use targeted search and read the smallest coherent sections needed.
- Do not inspect unrelated directories or launch subagents for a localized task.
- During editing, run focused checks. Run the complete final quality gate once
  after the implementation is stable.
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

Separate focused iteration checks from the full final gate.

## During implementation

Run formatter and tests only for the affected package/component as needed.

## Final gate

Run the complete required formatter, tests, race/type checks, lint, build and
`git diff --check` once. Inspect the final scoped diff and report any check that
could not run.

# Final report

Keep the report compact: changed files, key decisions, verification results and
remaining assumptions. Do not repeat the task specification.
