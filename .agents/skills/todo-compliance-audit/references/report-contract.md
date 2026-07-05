# Strict audit report contract

Use this exact report order.

## 1. Verdict

Print one line:

```text
STRICT PASS — every atomic requirement is PASS
```

or:

```text
STRICT FAIL — <passed>/<total> atomic requirements PASS
```

Then show counts for `PASS`, `PARTIAL`, `FAIL`, `NOT PROVEN`, and `N/A`.

## 2. Blocking findings

List concrete non-PASS findings ordered by impact:

1. core required behavior or delivery failure;
2. correctness/security/resource/runtime failure;
3. strong bonus or recommended-stack deviation;
4. missing test/documentation/external proof.

For each finding include:

- requirement ID and status;
- source line in `TODO.md`/`openapi.yaml`;
- tight implementation file/line evidence;
- observed or reachable failure scenario;
- why current tests/docs do not prove compliance.

Do not merge distinct atomic requirements merely to shorten the report.

## 3. Atomic traceability ledger

Include every requirement in a table:

| ID | Class | Atomic requirement | Status | Production evidence | Test/runtime evidence | Gap |
|---|---|---|---|---|---|---|

Rules:

- cite tight `file:line` locations;
- use `—` rather than invented evidence;
- keep each row atomic;
- retain rows that pass;
- `NOT PROVEN` must say which unavailable check would prove it;
- `N/A` must quote the source condition making it inapplicable.

## 4. Verification log

List every attempted command with:

- working directory;
- exit status;
- concise result/test count;
- skipped or unavailable checks and reason;
- whether artifacts or temporary runtime resources were created/cleaned.

Never summarize an unrun group as green.

## 5. Submission-state audit

Report separately:

- current branch/HEAD/dirty state;
- upstream ahead/behind state;
- whether current audited code is committed and pushed;
- public GitHub anonymous accessibility evidence;
- clean-clone reproducibility evidence;
- dataset/config/lockfile completeness;
- tracked or ignored artifacts/secrets concerns.

Local working-tree compliance does not prove submission compliance.

## 6. Residual uncertainty

List remaining environmental or observational limitations. Every item here must
map to at least one `NOT PROVEN` ledger row and therefore prevent `STRICT PASS`.

If there is no uncertainty, state that explicitly.

## 7. Scope statement

End with:

- files/domains inspected;
- whether code was modified (`No` for this skill);
- whether full backend/frontend/runtime/clean-clone checks were actually run;
- a reminder that no fix was applied.
