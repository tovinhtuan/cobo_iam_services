---
description: Kiểm tra sẵn sàng release (wf-release Phase 0-4), không deploy
argument-hint: [be | fe | all] [previous-deployed-sha]
---

Run the `wf-release` skill, Phases 0-4 only (candidate, checks, risk review of
the release diff, release-specific checks, go/no-go).

Arguments: $ARGUMENTS

Do **not** deploy, push, tag or restart anything. End with the GO / NO-GO
table and wait for my decision.
