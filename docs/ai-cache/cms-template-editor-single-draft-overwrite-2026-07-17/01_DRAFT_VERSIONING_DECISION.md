# 01 — Draft versioning decision

## Answers

1. **Draft = mutable working copy?** Yes.
2. **Giữ nhiều draft cũ?** No — tối đa 1 open draft per `type_id`.
3. **Save Draft updates row nào?** `MAX(version_no)` trong các version **không** phải `active_version_no`. Nếu chưa có → INSERT `max+1`.
4. **Publish/Activate?** Option A: mark draft version thành active (`active_version_no = draft`); set `is_released=1`. Không tạo snapshot row mới. Draft sau activate = lần save tiếp theo tạo draft mới.
5. **Legacy nhiều draft?** List chỉ trả: `is_released=1` OR active OR open draft (max non-active). Không xóa DB.
6. **Migration?** Yes — additive `is_released TINYINT(1) NOT NULL DEFAULT 0` + backfill active → 1. Không partial unique.
7. **Ảnh hưởng Portal active?** Không — save không đổi `active_version_no`.

## Rules

```
SaveDraft:
  openDraft = MAX(version_no) WHERE version_no != active_version_no
  if openDraft: UPDATE content/blocks; keep version_no; is_released=0
  else: INSERT next; is_released=0 (or is_released=1 if first auto-active create)

Activate:
  active_version_no = version_no
  is_released = 1 for that version
```

## Verdict

**READY_TO_IMPLEMENT**
