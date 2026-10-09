# Release DEV 2026-10-09: checks

## Phase 1: checks
| Repo | Lệnh | Kết quả |
|---|---|---|
| iam | `go build ./...` | PASS |
| iam | `go vet ./...` | FAIL có sẵn (copylocks ở `workflowfulfillment/required_document_gate_test.go:326-327`) |
| iam | `go test ./... -count=1` | Exit 1: 18 test fail có sẵn, không có fail mới so với HEAD (gồm 6 test import thuộc `dea4e59`, 5 test `httpserver`, `companyaccess`, `notification`) |
| iam | `go test -race ./internal/adhoc/... ./internal/workflowconfig/...` | PASS |
| iam | `docker compose -f docker-compose.dev.yml build api` | PASS |
| web | `npm run lint` (`tsc --noEmit`) | **FAIL có sẵn**: 15 lỗi, giống hệt HEAD; các file trong release không có lỗi. ⚠️ `deploy-dev.sh` dừng FE khi lint fail |
| web | `npx vitest run` (toàn bộ) | 63 test fail có sẵn, HEAD fail 64, không có fail mới |
| web | `npm run build` | PASS |
| web | `npm run check:mojibake` | OK |

## Phase 2: risk review của diff
- C2: be-security + admin-role (2 vòng): không có CRITICAL/HIGH/MEDIUM; LOW đã xử lý.
- C3: be-security + admin-role + api-compat: không có CRITICAL/HIGH/MEDIUM; LOW đã xử lý.
- FE 403 UX: fe-security: không có finding bảo mật.
- `dea4e59` (đã nằm ở HEAD, chưa có trên DEV): regression import đã ghi nhận (MEDIUM/LOW, xem `10-risk-report.md` API-05/06, BES-09). **Cần user chấp nhận.**
- Secret scan các file thay đổi: chỉ có false positive (token giả trong test, fixture in-memory có sẵn ở `server.go:245`).

## Phase 3: release checks
- **Compatibility:**
  - BE mới với FE cũ trên DEV: OK. Riêng CMS operator thiếu `cms.template.write/activate` bấm publish/activate sẽ nhận 403 và FE cũ chuyển sang `/app/forbidden`. Deploy FE ngay sau BE sẽ khắc phục.
  - Route bị gỡ không có client nào gọi.
  - Worker không bị ảnh hưởng (chỉ đổi interface adhoc, rebuild cùng lúc).
- **Thứ tự deploy:** BE (api + worker) → verify → FE → verify.
- **Migrations:** không có migration mới. Lưu ý: `deploy-be` xoá rồi SCP lại thư mục `migrations/`, nhưng không chạy migration.
- **Env/flags:** không có biến mới. `WORKFLOW_VERSIONING_ENABLED=true` vẫn giữ: sau deploy các route này có gate nên an toàn. Cờ `VITE_*` không đổi.
- **Cache FE:** `index.html` no-store, asset có hash. `deploy-fe` dừng web rồi `rm -rf dist` trước khi SCP, nên web gián đoạn ngắn (CACHE-06).
- **Rollback point:**
  - `deploy-be` chạy `rm -rf bin/api bin/worker` và không giữ bản cũ. **Đề xuất** copy `bin/api`, `bin/worker`, `web/dist` sang `*.bak-20261009` trên server trước khi deploy.
  - Rollback = khôi phục bản `.bak` rồi restart api/worker/web.
  - Lưu ý: rollback BE sẽ mở lại C2/C3.
