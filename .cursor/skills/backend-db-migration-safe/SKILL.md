---
name: backend-db-migration-safe
description: Dùng khi thay đổi schema MySQL hoặc semantics dữ liệu, tập trung vào compatibility, rollout an toàn và tránh downtime/lỗi dữ liệu
---

# Backend DB Migration Safe

## When to use
Use this skill when:
- adding/changing tables or columns
- changing constraints or indexes
- backfilling data
- changing read/write semantics

## Workflow
1. Mô tả current schema và desired schema.
2. Phân tích backward compatibility giữa code cũ và code mới.
3. Chọn strategy an toàn: expand -> migrate -> contract khi phù hợp.
4. Xác định dữ liệu cũ/null/default handling.
5. Xác định lock/performance risk.
6. Viết migration checklist và rollback considerations.
7. Xác nhận API/worker nào bị ảnh hưởng.

## Guardrails
- Không merge schema change mà chưa nghĩ đến mixed-version deployment.
- Không thêm NOT NULL / unique / foreign key risk cao mà thiếu backfill plan.
- Không đổi semantics dữ liệu mà không sửa tests tương ứng.

## Output format
- Schema delta
- Compatibility analysis
- Rollout steps
- Risk points
- Validation queries/tests
