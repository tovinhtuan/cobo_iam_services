# 05: Kiểm chứng, deploy DEV và smoke (MR-4)

## Commit
| Repo | Commit | Nội dung |
|---|---|---|
| BE | `253c65f` | Tính năng, migration 0156, test, contract và docs API |
| FE | `95363723` | Tính năng Portal và test |

## Kiểm tra cục bộ (trên đúng nội dung đã commit)
| Lệnh | Kết quả |
|---|---|
| `go test ./...` | 19 dòng fail, **trùng hệt baseline HEAD `5e4fff0`**: 14 test (`template_import*` reminder `not_positive`, `TestIntegration_*` …) và 5 package. Không có fail mới |
| `MYSQL_TEST_DSN=… go test ./internal/disclosure/infra/mysql/ -run OverrideReview` (MySQL 8 tạm, đã chạy 0156) | 3/3 pass |
| Mutation (M1–M7 trước review, M8–M17 sau review) | 17/17 bị test bắt |
| `go vet ./internal/disclosure/...` | Sạch |
| `go vet ./...` | Lỗi `copylocks` ở `internal/workflowfulfillment/required_document_gate_test.go:326-327`. File không đổi từ 14/09, không thuộc MR-4 |
| `docker compose -f docker-compose.dev.yml build api` | Built |
| FE `npx vitest run` | 48 fail / 3290 pass. 48 tên fail **trùng hệt baseline HEAD `54fb5c87`** |
| FE `npm run lint` | 12 lỗi TS, giống hệt HEAD (`dashboardOverviewAdapter.ts`, `LegacyUserProfile.tsx`) |
| FE `npm run build`, `npm run check:mojibake` | OK |

## Deploy DEV (16/10/2026)
| Bước | Lệnh | Kết quả |
|---|---|---|
| Migration | `sh deploy-artifacts/push-migration.sh 0156_workflow_override_version_review.up.sql` | "Migration applied and tracked". Ledger có 0156. `information_schema` có đủ 6 cột. **0155 không bị đẩy** (chưa được duyệt) |
| BE | `bash deploy-dev.sh be --skip-tests` (HEAD `253c65f`) | `/healthz` ok, `/readyz` ready, nginx `/api` 200. Log api/worker 3 phút: 0 panic/fatal/unknown column |
| FE | `VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED=true VITE_STEP_COMMENT_MENTIONS_V11=true sh deploy-dev.sh fe --skip-tests` (HEAD `95363723`) | Bundle `index-D8LVrMPp.js` đang phục vụ |

Rollback:
- BE: deploy lại `5e4fff0` sau khi chạy runbook trong 02-contract. Cột 0156 để nguyên (binary cũ không đọc chúng).
- FE: deploy lại `54fb5c87` (bundle `index-Cj9fssGR.js`), cùng bộ cờ.

## Smoke API: `scripts/devqa/smoke_mr4_override_maker_checker.py`, **20/20**
Dữ liệu ghi gồm hai phần, đều trong phạm vi đã duyệt ở GATE A:
- MEMBER được cấp tạm `template.workflow.override.read/.write` qua `membership_direct_permissions`, và đã bị thu hồi khi dọn;
- các version override trên loại QA `dt-co-1791964269635927893` ("QA smoke F3 bất thường 2026-10-14", loại riêng của c_001, chưa có hồ sơ).

| Nhóm | Kết quả |
|---|---|
| Schema | 0156 có trong ledger, đủ 6 cột |
| Người chỉ có quyền ghi (MEMBER) | `publish:true` trả 403 và không tạo override; lưu nháp được; submit chuyển sang `pending_approval` kèm `submit_note`; approve, reject, danh sách chờ duyệt và rebase-apply đều 403; body không phải JSON trả 400 |
| Người duyệt (ENT) | Danh sách chờ duyệt có tên loại và tên người gửi; reject không có lý do trả 422 `field=reason`; reject giữ `submit_note`; duyệt bản `rejected` trả 409 `STATE_CONFLICT` |
| Bốn mắt / tự duyệt | ENT duyệt bản MEMBER gửi: `self_approved=false`, `from_state=pending_approval`; duyệt lần hai trả 409; ENT `publish:true` kích hoạt ngay, `self_approved=true` |
| Bản nháp bị xoá | Dòng giữ `discarded`; bản nháp tiếp theo là v+1 (không dùng lại số) |
| Audit | `submitted` ×2, `rejected`, `approved` (`self_approved=false`, `from_state`), `applied` (`self_approved=true`) |
| FE | Bundle đang chạy có chuỗi của MR-4 |
| Dọn | Đặt lại override bằng API (HTTP 200); còn 0 grant tạm |

## Smoke trình duyệt: `cobo_web_design/docs/ai-cache/mr4-override-maker-checker-2026-10-16/smoke-mr4-ui.mjs`, **9/9**
Script chặn mọi lệnh ghi API (trừ auth). Fixture dùng `--ui-fixture up`: MEMBER gửi duyệt một bản trên loại QA. Dọn bằng `--ui-fixture down`: ENT từ chối bản đó, MEMBER tạo rồi xoá một bản nháp để không còn bản mở, sau đó thu hồi grant.

Lần chạy đầu sai 3 mục và lần hai sai 1 mục, đều do script: lần đầu mở nhầm tab mặc định, lần hai tìm nút sai tên (nút thật là "Cấu hình workflow"). Không phải lỗi sản phẩm.

| Kiểm tra | Kết quả |
|---|---|
| ENT, màn danh sách | Thẻ "Tùy chỉnh workflow chờ duyệt (n)" có link tới loại QA |
| ENT, tab Workflow | Chip "Chờ duyệt vN", người gửi, "Ghi chú gửi duyệt", có nút Duyệt và Từ chối |
| ENT | "Xem nội dung bản vN" mở danh sách bước chỉ đọc (`aria-expanded=true`); hiện ghi chú "chưa có bản đang áp dụng" |
| ENT | Hộp xác nhận Duyệt mở ra và đóng bằng "Quay lại" |
| ENT, 375px | Panel hiện, không có thanh cuộn ngang |
| MEMBER | Chip "Chờ duyệt", không có Duyệt/Từ chối; trình chỉnh sửa có "Gửi duyệt", không có "Kích hoạt"; màn danh sách không có thẻ chờ duyệt |
| Guard | Không có lệnh ghi nào bị gửi |

Lỗi 403 `/api/v1/admin/departments` của MEMBER có từ trước MR-4: trang chi tiết vẫn tải danh mục phòng ban như trước, MR-4 chỉ dùng lại map đó.

## Dữ liệu còn lại trên DEV (đã biết)
- Loại QA `dt-co-1791964269635927893` ở c_001:
  - header override có `active_version_no=0`, nên workflow hiệu lực vẫn là mặc định;
  - còn các version v1–v11: `approved` v2, v3 (lịch sử), `rejected`, `discarded`;
  - GET không còn bản mở.
  - User QA chỉ được ghi hai bảng quyền; xoá hẳn các dòng version cần quyền root DB, nên chưa làm.
- Các dòng grant tạm còn lại với `revoked_at` (đã thu hồi).
- Dòng `audit_logs` của smoke (append-only).
