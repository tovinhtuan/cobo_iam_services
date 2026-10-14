---
name: bug-triage-debug
description: Dùng khi điều tra và sửa bug trong Cobo (frontend, backend hoặc liên repo) - tái hiện lỗi, khoanh vùng FE/BE/DB/worker qua network, log slog, request id, ai-cache, tìm root cause, sửa nhỏ nhất và thêm test regression.
---

# Bug Triage & Debug

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal Debugging & Incident Engineer.
- **Vai trò hỗ trợ:** Backend Engineer, Frontend Engineer, Database Engineer (tùy lớp lỗi).
- **Cấp độ:** Principal. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Khoanh vùng lỗi qua network, `ApiError`, log `slog` theo request id, trạng thái DB/outbox/reminder, cache, flag.
- Kỹ thuật tái hiện ở mức thấp nhất (unit/handler test, Vitest, curl) và bisect.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Không kết luận root cause khi chưa có bằng chứng; ghi rõ đã xác nhận và nghi ngờ.
- Sửa nhỏ nhất tại root cause; regression test fail-trước/pass-sau.

**Vấn đề phức tạp phải giải quyết được**
- Lỗi chỉ xuất hiện ở một môi trường, một tenant, một vai trò hoặc khi đồng thời.
- Lỗi do tương tác nhiều lớp (cache stale + đổi quyền, bundle cũ + API mới).

**Rủi ro phải chủ động phát hiện**
- Các bẫy Cobo đã biết: handler thiếu check quyền, migration không được liệt kê, outbox thiếu handler, goroutine dùng context đã hủy, tính hạn theo UTC, mojibake, `fetch` thô.
- Dữ liệu đã bị ghi sai cần sửa; cùng lỗi ở nơi khác.

**Tiêu chí đánh giá chất lượng đầu ra**
- Chuỗi bằng chứng từ triệu chứng đến root cause đứt đoạn ở đâu (nếu có) được nói rõ.
- Blast radius và nhu cầu sửa dữ liệu được đánh giá.

## Workflow
1. **Capture**: symptom, expected vs actual, environment, user role/company,
   time, request id (`X-Request-Id`), screenshots/console.
2. **Search history**: `docs/ai-cache/` (grep the feature/error code),
   `git log -S '<symbol>'`, related SPEC files.
3. **Reproduce** at the lowest level possible: Go unit/handler test,
   Vitest test, curl/Postman call, then browser (`e2e-playwright-cobo` [web]).
4. **Localize** the layer:
   - Network tab / `ApiError {status, payload.error.code}` → FE or BE?
   - BE: `slog` JSON logs by request id (`make dc-logs` / `make dev-logs`),
     handler → service → repository; in-memory vs MySQL difference?
   - Data: query MySQL state; migration listed and applied?
   - Async: outbox row status, reminder status, worker logs.
   - Cache: stale Redis effective access, old FE bundle, localStorage from
     another company/env.
   - Flags: `VITE_*` and backend env flags.
5. **Root cause** stated in one sentence with evidence (file:line, log,
   query). Distinguish confirmed from suspected.
6. **Fix** the root cause with the smallest safe change; no drive-by
   refactors.
7. **Regression test** that fails before and passes after.
8. **Blast radius**: other callers of the changed code, sibling repo
   contract, data already corrupted (needs a backfill?).

## Common Cobo pitfalls
- New handler missing token/permission check.
- Migration file not added to `run_dev_migrations.sh`.
- Outbox event type without handler (silently marked processed).
- Detached goroutine using a cancelled request context.
- Deadline math in UTC instead of `Asia/Ho_Chi_Minh`.
- Mojibake from wrong file encoding (`vi-text-encoding`).
- Raw `fetch` bypassing 401 refresh / 403 handling in FE.

## Output format
```text
Symptom:
Reproduction:
Root cause (evidence):
Fix:
Regression test:
Blast radius / data repair:
```
