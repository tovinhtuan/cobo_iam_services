# C3: Verify (Gate V), 2026-10-09

Baseline: worktree sạch của HEAD `0609fdf`, chạy song song cùng lệnh. Working tree hiện có cả fix C2 (`internal/workflowconfig`) lẫn fix C3 (`internal/adhoc`).

| Kiểm tra | Kết quả |
|---|---|
| `TestLegacyMigrationRoute_NotExposed` | Trước fix FAIL (200 `{"processed":1}`, finalize `company-B` bằng token `company-001`, xem `02-repro.md`). Sau fix PASS (404) |
| Kiểm tra ngược route | Đăng ký lại route → FAIL (200). Khôi phục → PASS |
| `TestAdminApprove_UsesTokenCompanyIgnoringBodySubject` | PASS. Bỏ `body.Subject = sub` → FAIL: service nhận `company-B` từ body. Xác nhận body JSON ghi đè được `Subject` khi thiếu dòng gán lại |
| `TestRegister_NoPlatformRoutes` | PASS (module adhoc không còn chuỗi `/api/v1/platform/`) |
| `TestAdminApprove_ByNonController_Returns403` (có sẵn) | PASS |
| SQL tenant scope (đọc code) | `FindByID` và `ReserveAdminApproval` có `WHERE proposal_id = ? AND company_id = ?` (`adhoc/infra/mysql/repository.go:120,328`) |
| Grep symbol legacy trong `internal/`, `cmd/` | Chỉ còn path trong test regression `tenant_scope_test.go` |
| `go build ./...` | PASS |
| `go test ./internal/adhoc/... -count=1` | PASS |
| `go test -race -count=1 ./internal/adhoc/...` | PASS (lần chạy này không gặp PERF-10 flaky) |
| `go test ./... -count=1` | Exit 1 ở cả fix lẫn baseline. **Không có test fail mới.** Fix 18 dòng FAIL, baseline 19 (baseline có thêm `TestSchemaParityConditionalPeriodicity`, giống lần C2) |
| `go vet ./...` | Chỉ lỗi copylocks có sẵn ở `workflowfulfillment/required_document_gate_test.go:326-327` |
| `docker compose -f docker-compose.dev.yml build api` | PASS |
| Smoke local | **BLOCKED**: stack Cobo local không chạy (8080 không phản hồi). Hành vi đã được phủ bằng handler test |

Ghi chú:
- `go.mod` (`x/text` chuyển thành direct) vẫn là thay đổi ngoài scope có từ trước. Các lệnh trong C3 không làm đổi thêm.
- Follow-up phát hiện khi verify: `body.Subject = sub` còn ở `handler.go:183` và `:266`. Nên thêm tag `json:"-"` cho các field `Subject` trong request struct của adhoc để phòng thủ nhiều lớp.

## Bổ sung sau review (T5)
| Kiểm tra | Kết quả |
|---|---|
| `TestMutatingRoutes_UseTokenSubjectIgnoringBodySubject` (BES-01: create, patch, approve, focal-approve, reject, cancel) | PASS. Bỏ `body.Subject = sub` trong reject → FAIL (`company-B` lọt qua) |
| `TestAdminApprove_EmptyControllerOrMembership_Returns403` (ROLE-01) | Trước fix FAIL: case "both empty" được duyệt (err = nil). Sau fix PASS |
| `go build ./...`, `go vet ./internal/adhoc/...` | PASS |
| `go test ./internal/adhoc/... -count=1` | PASS |
| `go test -race -count=1 ./internal/adhoc/app/` ×6 | 5 PASS, 1 FAIL `close of closed channel` ở `raceFakeRepo.rendezvous` (`TestConcurrentCancelAndApprove...`, luồng `Approve`). Đây là **PERF-10, flaky có sẵn**, không liên quan diff |
| `go test ./internal/httpserver/...` | 5 test fail, giống hệt baseline |
| `docker compose -f docker-compose.dev.yml build api` | PASS (chạy lại sau fix ROLE-01) |

## Bổ sung (follow-up 6 và 8)
| Kiểm tra | Kết quả |
|---|---|
| `TestRequestStructs_DoNotDecodeSubjectFromJSON` | PASS. Bỏ tag `json:"-"` → 5 struct decode được `Subject` → FAIL |
| PERF-10 `TestConcurrentCancelAndApprove_OneWinsOneGets409Conflict` với `-race` | Trước sửa: 4/15 lần FAIL (`close of closed channel`). Sau sửa: 0/30 |
| `go test -race -count=1 ./internal/adhoc/...` ×3 | PASS |
| `go test ./... -count=1` | Danh sách test fail giống hệt lần chạy trước (không có fail mới) |
| `go vet ./...` | Chỉ lỗi copylocks có sẵn |
| `docker compose -f docker-compose.dev.yml build api` | PASS |
