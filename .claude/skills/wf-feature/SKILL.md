---
name: wf-feature
description: Workflow end-to-end để xây một feature Cobo (FE, BE hoặc cả hai repo) - brief, thiết kế và chốt contract (gate người dùng duyệt), migration, backend, frontend, test, E2E, review song song bằng subagent, premerge, Postman và ai-cache. Dùng khi được yêu cầu làm feature/chức năng mới, mở rộng API/màn hình, hoặc lệnh /feature.
---

# Workflow: Feature Delivery

Each phase produces an artifact in `docs/ai-cache/<feature-slug>-YYYY-MM-DD/`
(numbered files). Do not skip a gate; if the user explicitly waives one,
record that in the artifact.

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal Software Engineer - Feature Delivery Lead (full-stack).
- **Vai trò hỗ trợ:** Solution Architect (contract, dữ liệu), Test Architect (ma trận kiểm thử), Product/Business Analyst (tiêu chí chấp nhận).
- **Cấp độ:** Principal. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Giao feature theo lát cắt dọc qua migration → repository → service → handler → FE → test → review.
- Contract-first, expand → migrate → contract, feature flag hai phía (`VITE_*` và env backend).
- Tiêu chí chấp nhận, test matrix theo rủi ro, review song song bằng reviewer subagent.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Không viết code triển khai trước GATE A; không báo "xong" khi GATE B còn Critical hoặc check bắt buộc fail.
- Mỗi phase có artifact và bằng chứng; người dùng miễn gate thì ghi lại.

**Vấn đề phức tạp phải giải quyết được**
- Định cỡ đúng (small/normal) để không làm quá hoặc thiếu quy trình.
- Khóa contract khi yêu cầu còn mơ hồ: nêu giả định, câu hỏi mở và hệ quả của từng lựa chọn.
- Phối hợp thứ tự deploy BE/FE/migration để tab cũ và worker cũ vẫn chạy.

**Rủi ro phải chủ động phát hiện**
- Scope creep, contract đổi giữa chừng mà không quay lại GATE A.
- Thiếu state UI (403/409/empty), thiếu check quyền trong handler, migration không có trong `run_dev_migrations.sh`.
- Feature chỉ chạy với MySQL hoặc chỉ chạy in-memory.

**Tiêu chí đánh giá chất lượng đầu ra**
- Tiêu chí chấp nhận được truy vết tới test; gate có người và thời điểm duyệt.
- Rollout (flag, thứ tự deploy, migration) và follow-up rõ ràng, có thể thực thi.

## Phase 0 - Intake  → `00-brief.md`
- Restate the goal, users/roles, success criteria, repos affected.
- Load context per `cobo-task-workflow` Step 2 (ai-cache README, related
  ai-cache files, specs, code).
- Domain check: CBTT rules (`cobo-domain-cbtt`), role boundary
  (`cobo-admin-role-guard`).
- Size it: **small** (one repo, no API/schema change) → go to Phase 3/4
  directly after a short plan; **normal** → all phases.

## Phase 1 - Design  → `01-design.md`
- `system-design-feature` (cross-repo) or the narrow design part of
  `backend-feature-delivery` / `frontend-feature-slice`.
- Write or update the spec with `spec-authoring` when the feature spans
  several PRs.

## Phase 2 - Contract lock  → `02-contract.md`  **GATE A**
Contents: endpoints, request/response, error codes, permissions, company
scope, idempotency, flags, migration list, compatibility matrix
(`api-compatibility-rolling-deploy`), test matrix.
**Stop and ask the user to approve the contract** before writing
implementation code. Proceed without approval only if the user said so.

## Phase 3 - Backend (if in scope)  [iam]
Order: migration (`backend-db-migration-safe`, add to
`run_dev_migrations.sh`) → repository (MySQL + in-memory) → service →
handler (`backend-api-contract`, auth/permission check in handler) →
worker/outbox (`backend-worker-idempotent`) → flags/config/compose.
After each slice: `go test ./<pkg>/...`; at the end `go vet ./...`,
`go test ./...`, `docker compose -f docker-compose.dev.yml build api`.

## Phase 4 - Frontend (if in scope)  [web]
`frontend-feature-slice` → `frontend-route-screen` (routing) →
`frontend-ui-ux-quality` (+ `taste-frontend-cobo` for visual work) →
`frontend-ui-state-guard` → `frontend-test-regression`.
Checks: `npm run lint`, focused `npx vitest run`, `npm run build`,
`npm run check:mojibake`.

## Phase 5 - Integration  → `05-verification.md`
- Contract tests on both sides; Postman updated (`api-postman-sync`).
- Local full stack (`local-dev-stack`) + smoke via `e2e-playwright-cobo`
  for user-visible flows (authorized environments only).

## Phase 6 - Parallel review  → `06-review.md`
Pick reviewers with the path map in `wf-risk-review` Phase 1 and launch them
in one message. Fix every CRITICAL/HIGH (or get explicit user acceptance),
then re-run only affected reviewers.

## Phase 7 - Premerge  **GATE B**
`premerge-system-review` on the full change. Do not report "done" with an
open Critical finding or a failing required check (use `BLOCKED:` honestly).

## Phase 8 - Close  → `99-summary.md`
`ai-cache-maintenance` summary; list follow-ups; no commit/push unless the
user asked for that exact operation.

## Completion report
```text
Feature:
Gates: A (contract) approved by/at:, B (premerge) result:
Files changed by repo:
Checks run and results / BLOCKED:
Review findings fixed / accepted:
Rollout: flags, deploy order, migrations:
Follow-ups:
```
