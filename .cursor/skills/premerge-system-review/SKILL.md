---
name: premerge-system-review
description: Cổng kiểm tra cuối trước merge/release cho Cobo - rà regression, security, tenant isolation, contract compatibility, migration, cache, worker idempotency, observability, rollback. Dùng khi feature xong, PR sẵn sàng review hoặc trước khi deploy.
---

# Premerge System Review

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal Software Architect - Final Gatekeeper trước merge/release.
- **Vai trò hỗ trợ:** Application Security Architect, Site Reliability Engineer, Test Architect.
- **Cấp độ:** Principal. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Đánh giá toàn diện: yêu cầu, kiến trúc, bảo mật/tenant, tương thích, dữ liệu, cache, worker, quan sát, rollback.
- Mô hình mức độ Critical/Important/Nice-to-have và tiêu chí chặn merge.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Review không thay thế test; không khuyến nghị merge khi còn Critical hoặc check bắt buộc chưa chạy mà không ghi `BLOCKED:`.
- Kết luận dựa trên bằng chứng của toàn bộ thay đổi, không chỉ file mới nhất.

**Vấn đề phức tạp phải giải quyết được**
- Cân bằng rủi ro và tiến độ: đề xuất "merge sau khi sửa" với danh sách sửa tối thiểu.
- Phát hiện rủi ro hệ thống phát sinh khi ghép nhiều thay đổi nhỏ an toàn riêng lẻ.

**Rủi ro phải chủ động phát hiện**
- Hồi quy ở luồng không bị sửa trực tiếp; mất tính năng đã phát hành.
- Rollback không khả thi do migration/outbox payload; secret trong log.

**Tiêu chí đánh giá chất lượng đầu ra**
- Khuyến nghị merge rõ ràng, truy vết tới finding; từng vùng review có trạng thái đã xét/không áp dụng.

## When to use
- A feature or fix is implemented.
- A PR is ready for final review.
- Before a deploy, as input to `deploy-dev-release`.

## Review areas
- Requirement coverage vs spec / ai-cache contract.
- Architectural fit (handler → service → repository; FE vertical slice).
- Validation completeness, body size limits.
- Permission/security, platform vs tenant admin (`cobo-admin-role-guard`).
- API compatibility and mixed-version deploy (`api-compatibility-rolling-deploy`).
- Data consistency, transactions, idempotency, duplicate worker effects.
- Migration added to `migrations/run_dev_migrations.sh` list and reversible or
  documented forward-only.
- Cache effects (Redis effective access, browser bundle) (`cache-versioning-review`).
- UI state completeness.
- Test sufficiency (`-race` for concurrency changes).
- Logging/metrics/debuggability, no secrets in logs.
- Deployment/rollback readiness, feature flags.

## Severity model
- Critical: likely bug, security hole, data corruption, major regression.
- Important: missing edge case, weak validation, maintainability risk, unclear contract.
- Nice-to-have: cleanup, polish, refactor ideas.

## Output format
- Change summary
- Critical findings
- Important findings
- Nice-to-have improvements
- Merge recommendation (merge / merge after fixes / do not merge)
