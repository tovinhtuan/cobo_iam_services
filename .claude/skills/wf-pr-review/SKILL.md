---
name: wf-pr-review
description: Workflow review một branch/PR/diff trong Cobo - xác định file thay đổi, chọn reviewer subagent theo đường dẫn, chạy song song, chạy test baseline, tổng hợp premerge-system-review và kết luận merge/không merge. Dùng khi được nhờ review code, review PR, review branch, hoặc lệnh /pr-review.
---

# Workflow: PR / Branch Review

Review-only: no code edits unless the user asks.

## Phase 0 - Diff
1. Base: argument, else merge-base with `main`. Record base and head SHA.
2. `git diff --stat <base>...HEAD`, `git diff --name-status <base>...HEAD`,
   uncommitted changes listed separately.
3. Read the PR intent: commit messages, linked spec (`SPEC-*.md`), task files
   in `tasks/`, ai-cache notes.

## Phase 1 - Correctness pass (main agent)
For each changed file read the diff with enough surrounding context:
- logic errors, missing error handling, nil/undefined paths, off-by-one,
  timezone (`Asia/Ho_Chi_Minh`) and day-counting rules,
- tests: do they cover the change and its failure paths?
- repo conventions (layering, `createXApi`, migrations listed in
  `run_dev_migrations.sh`, flags in `.env.example` and compose files).

## Phase 2 - Specialist reviewers (parallel)
Select reviewers with the path map in `wf-risk-review` (Phase 1) and launch
them in one message. Pass base/head SHA and the changed-file list; ask for
the shared finding format (`wf-risk-review/references/finding-format.md`).

## Phase 3 - Checks (when allowed to run commands)
| Changed | Run |
|---|---|
| web | `npm run lint`, focused `npx vitest run <files>`, `npm run build`, `npm run check:mojibake` |
| iam | `go vet ./...`, `go test ./<changed pkgs>/...`, `go test -race` for concurrency changes |
| docker/API | `docker compose -f docker-compose.dev.yml build api` |
Report `BLOCKED: <reason>` for anything that cannot run.

## Phase 4 - Verdict
Apply `premerge-system-review` to the combined evidence and reply:

```text
PR: <branch> (<base>..<head>)
Intent:
Critical findings:
Important findings:
Nice-to-have:
Tests/checks run:
Compatibility (old FE↔new BE etc.):
Verdict: merge / merge after fixes / do not merge
```
Optionally write `docs/ai-cache/pr-review-<branch>-YYYY-MM-DD.md` when the
repo rules require a task summary.
