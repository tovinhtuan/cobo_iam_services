# Deadline alerts tab trống — ad-hoc due date & pending proposal

**Ngày:** 2026-05-25

## Triệu chứng

Tab «Cảnh báo thời hạn» trả `items: []` dù đã có ad-hoc approve hoặc vừa tạo proposal.

## Hai nguyên nhân tách biệt

### 1. Proposal chưa approve (đúng nghiệp vụ)

JSON user paste = `ad_hoc_proposals` với `status: pending_focal_approval`, `record_id: null`.

Tab **không** list proposal — chỉ `disclosure_records` sau approve (`AdminApprove`). Cần focal approve → admin/process controller approve.

### 2. Record đã approve nhưng bị lọc (bug — đã fix)

Record `Published`, `planned_date` NULL, `final_deadline_date` NULL, type `deadline_mode: NONE`.

Service bỏ qua row khi `dueDate == ""`. Ad-hoc chỉ có `proposed_t0_date` + `proposed_deadline_days` (vd. T0 + 2 ngày).

**Fix (0074+ code):**

- SQL join ad-hoc: `final_deadline_date` OR `proposed_t0 + proposed_deadline_days`
- `published` → terminal → status `DONE` (hiển thị trên tab)

## Verify sau deploy BE

`GET /api/v1/company/deadline-alerts` với company `08f59da2-...` → ≥ 3 item (các record approved trước đó).

Proposal `019e5d70-...` chỉ xuất hiện sau khi `status=approved` và có `record_id`.
