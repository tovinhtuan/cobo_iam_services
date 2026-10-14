---
name: backend-db-migration-safe
description: Dùng khi thay đổi schema MySQL hoặc semantics dữ liệu trong cobo_iam_services - viết migration NNNN_*.up/down.sql, thêm vào run_dev_migrations.sh, expand-migrate-contract, mixed-version deploy, backfill, lock risk, rollback.
---

# Backend DB Migration Safe

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal Database Architect / Database Administrator (MySQL 8).
- **Vai trò hỗ trợ:** Release Engineer (thứ tự deploy), Backend Engineer (parity in-memory).
- **Cấp độ:** Architect. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Mô hình dữ liệu, index và execution plan (`EXPLAIN`), online DDL MySQL 8, lock và metadata lock.
- Expand → migrate → contract, backfill theo batch, transaction isolation, backup/recovery, down migration.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Không thay đổi schema khi chưa phân tích mixed-version; không sửa migration đã áp dụng.
- File mới phải có trong `run_dev_migrations.sh`; thay đổi dữ liệu có `verify_*.sql`.

**Vấn đề phức tạp phải giải quyết được**
- Thêm NOT NULL/UNIQUE/FK trên bảng có dữ liệu cũ; đổi ngữ nghĩa cột khi API và worker cũ/mới cùng chạy.
- Ước lượng tác động khi dữ liệu tăng trưởng lớn.

**Rủi ro phải chủ động phát hiện**
- `ALTER` khóa bảng lớn, index thiếu gây full scan, deadlock và lock contention trong backfill.
- Mất dữ liệu khi rollback, migration không đảo ngược mà không ghi lý do.

**Tiêu chí đánh giá chất lượng đầu ra**
- Schema delta, phân tích tương thích, bước rollout, điểm rủi ro và truy vấn/test kiểm chứng đầy đủ.

## When to use
- Add/change tables, columns, constraints, indexes.
- Backfill data; change read/write semantics.

## Project facts (verify)
- Files `migrations/NNNN_name.up.sql` + `.down.sql` (some legacy ones have no
  down). Pick the next free number.
- Runner `migrations/run_dev_migrations.sh` applies an explicit ordered
  `MIGRATIONS=` list and records `schema_migrations(file_name)`.
  **A new file not added to that list never runs** (`deploy-dev.sh` reads the
  same list). Known gaps: 0038, 0098, 0099, 0150 were not listed.
- Supporting files: `dryrun_*.sql`, `verify_*.sql`, `seed_*.sql`; some
  migrations have Go tests (`migrations/*_test.go`).
- MySQL 8; outbox uses `SKIP LOCKED`.

## Workflow
1. Current vs desired schema.
2. Compatibility between old and new code (API + worker run old/new mixed).
3. Strategy: expand → migrate → contract.
4. Old/null/default data handling; backfill in batches.
5. Lock/performance risk (`ALTER` on large tables, index builds).
6. Add to `run_dev_migrations.sh`; write `verify_*.sql` if data changes.
7. Rollback considerations (down file or forward-only rationale).
8. Affected APIs/workers and in-memory repo parity.

## Guardrails
- No schema change without mixed-version analysis.
- No risky NOT NULL / UNIQUE / FK without a backfill plan.
- No semantic data change without updated tests.
- Never edit an already-applied migration; add a new one.

## Output format
- Schema delta
- Compatibility analysis
- Rollout steps
- Risk points
- Validation queries/tests
