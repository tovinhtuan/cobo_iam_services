---
description: Review branch/PR hiện tại theo wf-pr-review và kết luận merge
argument-hint: [base-branch]
---

Run the `wf-pr-review` skill on the current branch.
Base: $ARGUMENTS (if empty, merge-base with `main`).

Review only - do not edit code. Finish with the verdict block
(merge / merge after fixes / do not merge).
