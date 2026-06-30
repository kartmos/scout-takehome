# Task prompts

The current implementation sequence and completion statuses are tracked in [`ROADMAP.md`](./ROADMAP.md).

Use numbered, descriptive filenames. Decimal suffixes such as `004.1` are corrective tasks belonging to the preceding main stage and do not shift the main roadmap numbering.

Keep one implementation task in each file. Start from `TEMPLATE.md`, then run it in Claude Code:

```text
/clear
/implement-task @.claude/prompts/<prompt-file>.md
```

Use one fresh Claude Code context per implementation prompt. Small fixes for the current prompt may stay in the same context. Run backend or frontend review in a separate clean context after each logical block.

Prompt files are a curated record of important tasks. Full chat transcripts do not need to be committed.
