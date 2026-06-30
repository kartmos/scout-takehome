# Goal

Inspect the provided Scout dataset and produce a factual audit that can be used by later implementation tasks.

# Context

Before doing anything, read:

- `CLAUDE.md`
- `README.md`
- `openapi.yaml`
- `.claude/prompts/README.md`
- the schema and contents of `dataset/predictions.db`
- the filenames and basic metadata of images in `dataset/images/`

The repository contains the assignment inputs but no application implementation yet. This task is investigation only.

# Required system tool

This is the first task that requires the SQLite command-line client.

Check whether it is available:

```bash
sqlite3 --version
```

If the command is missing, stop and tell the user to install it in Ubuntu/WSL:

```bash
sudo apt update
sudo apt install -y sqlite3
sqlite3 --version
```

Do not install it yourself and do not use this task to install any other technology.

# Scope

Perform a read-only audit of:

- `dataset/predictions.db`
- `dataset/images/`
- consistency between the dataset, `README.md`, and `openapi.yaml`

Do not create, edit, rename, or delete project files. Do not modify the SQLite database.

# Requirements

1. Inspect the SQLite schema, including tables, columns, indexes, constraints, and foreign keys.
2. Run `PRAGMA integrity_check` and report its result.
3. Report:
   - total photo count;
   - total prediction count;
   - number of photos with and without predictions;
   - prediction count grouped by `class_id`;
   - minimum and maximum confidence;
   - minimum and maximum `x`, `y`, and `h`;
   - distinct photo dimensions;
   - earliest and latest `captured_at` values.
4. Validate data invariants:
   - photo IDs are valid UUIDs;
   - prediction `photo_id` values reference existing photos;
   - confidence is within `[0,1]`;
   - every bbox coordinate is within `[0,1]`;
   - `bbox_xmin < bbox_xmax` and `bbox_ymin < bbox_ymax`;
   - photo coordinates are inside the 40 by 40 metre greenhouse;
   - photo width and height are positive.
5. Inspect `dataset/images/` and report:
   - total image count;
   - filename extensions;
   - distinct image dimensions;
   - files whose basename is not a valid UUID;
   - duplicate photo IDs, if any;
   - image files with no matching row in `photos`;
   - photo rows with no matching image file.
6. Compare observed prediction classes with the six documented classes:
   - `powdery_mildew`
   - `mirid`
   - `whitefly_aphid`
   - `miner_tuta`
   - `thrips`
   - `spider_mites`
7. Identify any material mismatch between the actual dataset, `README.md`, and `openapi.yaml`.
8. When querying SQLite, use read-only operations only. Do not run DDL, DML, `VACUUM`, or write-oriented PRAGMAs.
9. Prefer reproducible shell and SQL commands. Show the important commands in the final report so the audit can be repeated.

# Out of scope

- Creating the Go backend or frontend.
- Initializing Go, Node.js, pnpm, Docker, or MinIO.
- Designing application architecture beyond noting dataset implications.
- Changing `README.md`, `openapi.yaml`, `CLAUDE.md`, prompt files, or dataset files.
- Creating migrations or a replacement database.
- Committing or pushing changes.

# Acceptance criteria

- The task leaves `git status --short` unchanged.
- SQLite integrity and all listed invariants have been checked.
- Image-to-database correspondence has been checked in both directions.
- The report contains exact counts and ranges, not estimates.
- Every anomaly is listed with enough identifying information to investigate it.
- If there are no anomalies, the report explicitly says which checks passed.
- Assumptions and checks that could not be performed are clearly separated from verified facts.

# Verification

At minimum, run and report the results of:

```bash
git status --short
sqlite3 dataset/predictions.db ".schema"
sqlite3 dataset/predictions.db "PRAGMA integrity_check;"
find dataset/images -maxdepth 1 -type f | sort
git diff --exit-code
git status --short
```

Use additional read-only SQL and shell commands as needed to satisfy every requirement above.

# Final report format

Return a concise structured report with these sections:

1. `Dataset summary`
2. `Database schema`
3. `Integrity and invariant checks`
4. `Image/database correspondence`
5. `Contract mismatches`
6. `Implementation implications`
7. `Commands run`

Do not make implementation changes after producing the audit.
