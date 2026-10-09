---
description: Bắt đầu một task Cobo bất kỳ - phân loại và chạy đúng workflow/skill
argument-hint: <mô tả task>
---

Use the `cobo-task-workflow` skill for this task. First check Step 0: if a
`wf-*` workflow fits (feature, bugfix, risk-review, pr-review, release), switch
to it and follow its phases and gates. Otherwise select the minimum skill
chain.

Task: $ARGUMENTS

Start your answer with the `[ai-cache]` line required by the repo rules.
