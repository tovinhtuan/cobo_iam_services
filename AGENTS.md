# Agent instructions (Cobo — cobo_iam_services)

## Tín hiệu `[ai-cache]` trong Chat

Với câu trả lời có nội dung: **dòng đầu** = **`[ai-cache]`** + README + file `docs/ai-cache/` đã đọc + skill + **Mandatory README: đã áp dụng** (chi tiết đầy đủ: `cobo_web_design/docs/ai-cache/README.md` mục *Tín hiệu tuân thủ*, hoặc mục tương tự trong `docs/ai-cache/README.md` của repo này).

User có thể dán đầu prompt (khớp README):

```text
Bắt buộc: tuân thủ docs/ai-cache/README.md — dòng đầu câu trả lời phải có [ai-cache] theo README.
```

Snippet trên và **lệnh Docker/build sau implement** (hoặc `BLOCKED:`) nằm trong **`.cursor/rules/ai-cache-read-first.mdc`** — Agent phải tuân theo.

## Nguồn bắt buộc

1. **`docs/ai-cache/README.md`**
2. **`.cursor/rules/ai-cache-read-first.mdc`** (`alwaysApply: true`)

Task chạm cả frontend và IAM: skill **`integration-cross-repo`** + đọc `docs/ai-cache/` ở cả hai repo.

Không coi task implement là “xong” nếu chưa qua **`premerge-system-review`** và chưa báo verify (Docker/build/`BLOCKED:`) theo **`.cursor/rules/ai-cache-read-first.mdc`**.
