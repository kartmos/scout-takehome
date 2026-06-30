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
- name exact source and test files rather than whole directories;
- reference only relevant README/OpenAPI sections or component names;
- include a scoped initial diff and avoid repeated full-project inspection;
- use focused checks while editing and retain the complete quality gate once at
  the end;
- avoid task restatements and subagents for localized work.

This reduces repeated input/context tokens. It does not remove acceptance
criteria, final race/type/lint/build checks, or independent review gates.

Prompt files are a curated record of important tasks. Full chat transcripts do not need to be committed.
