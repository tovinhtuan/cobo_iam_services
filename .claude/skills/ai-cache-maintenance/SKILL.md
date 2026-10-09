---
name: ai-cache-maintenance
description: Dùng khi bắt đầu hoặc kết thúc task trong Cobo để đọc/cập nhật docs/ai-cache - marker [ai-cache] ở đầu câu trả lời, đọc README ai-cache trước, ghi task summary vào reusable-task-updates.md hoặc thư mục task theo ngày, SESSION_HANDOFF, không lưu screenshot/secret vào git.
---

# ai-cache Maintenance

`docs/ai-cache/` is the highest-priority project source of truth after
explicit user instructions (see repo rules).

## Start of task
1. Read `docs/ai-cache/README.md`. It may be stored mojibake-encoded; if text
   looks like `Ã`/`á»`/box-drawing characters, decode it for reading:
   ```bash
   python3 .claude/skills/vi-text-encoding/scripts/fix_mojibake.py --print docs/ai-cache/README.md
   ```
2. Grep task-relevant files instead of reading the huge
   `reusable-task-updates.md` fully:
   ```bash
   grep -n -i "<feature keyword>" docs/ai-cache/reusable-task-updates.md | head -50
   ls docs/ai-cache | grep -i "<topic>"
   ```
3. First line of every substantive answer (repo rule):
   ```text
   [ai-cache] README.md + <other ai-cache files | "chỉ README — task hẹp"> | skill: <.claude/skills/<name> | none> | Mandatory README: đã áp dụng.
   ```
   Skip only for one-line or non-repo replies.

## End of task (implementation tasks; review-only only if asked)
Append a summary to `docs/ai-cache/reusable-task-updates.md`, or create
`docs/ai-cache/<topic>-YYYY-MM-DD/` with numbered `00-summary.md` (+
`results.json` for QA runs):

```markdown
## <YYYY-MM-DD> <task title>
- Task type:
- Objective:
- Findings / decisions:
- Files changed (by repo):
- Verification (commands → result, BLOCKED:...):
- Gaps / follow-ups:
- Skills applied:
```

## Rules
- Facts only; separate confirmed from inferred.
- No secrets, tokens, passwords, personal data.
- Screenshots/traces/videos are gitignored under ai-cache; keep summaries
  and contracts only.
- Write UTF-8 (no BOM); run `npm run check:mojibake` in the web repo when
  touching text.
- `SESSION_HANDOFF.md`: update only when handing off a lane; mark stale
  sections rather than leaving outdated instructions.
