---
name: implement-task
description: Implements one Scout task from a prompt file prepared with Codex.
argument-hint: "@path-to-prompt"
disable-model-invocation: true
---

Implement only the task described in `$ARGUMENTS`.

1. Read the prompt, `CLAUDE.md`, `README.md`, `openapi.yaml` and relevant existing code.
2. Before editing, state the goal, scope, acceptance criteria and expected changed files.
3. If the prompt conflicts with the assignment or is materially ambiguous, stop and ask the user.
4. Make the smallest complete change that satisfies the task.
5. Do not modify unrelated files or add unrequested architecture.
6. Add or update relevant tests.
7. Run available formatting, focused tests, lint and type checks.
8. Inspect the final diff.
9. Report changed files, verification results and remaining assumptions.
10. Do not commit or push.
