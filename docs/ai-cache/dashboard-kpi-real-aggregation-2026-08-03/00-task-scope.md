# 00 — Task scope

**Verdict gate:** `PLAN_APPROVED_WITH_GUARDS` (user 2026-08-03)

## In scope

- Portal Operational Dashboard 6 KPI: labels, order, real aggregation, on-time tooltip/a11y
- Additive optional fields on `kpis.on_time_rate` (`completed_on_time`, `completed_total`)
- Remap semantic of existing overview `kpis.*` wire keys **only after** consumer audit PASS
- Targeted tests + evidence; no deploy

## Out of scope

- Alert/workflow state transitions, deadline calculator/materialization, permissions, migration, other dashboard sections, DEV/prod deploy

## Overlap note (product)

- “Cần xử lý ngay”, “Cảnh báo đang xử lý”, “Chưa hoàn thành & quá hạn” **có thể chồng lặp** — không cộng card để suy tổng.
- Chỉ **Hoàn thành & quá hạn** ↔ **Chưa hoàn thành & quá hạn** bắt buộc loại trừ nhau.
- KPI “Cần xử lý ngay” = tổng cùng predicate membership với `immediate_actions`; panel có thể capped — không assert số item render = KPI.