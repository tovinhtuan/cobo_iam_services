# C2: Verify (Gate V), 2026-10-09

Baseline so sánh: worktree sạch của HEAD `0609fdf`, chạy song song cùng lệnh.

| Lệnh | Kết quả |
|---|---|
| `go test ./internal/workflowconfig/... -count=1` | PASS (app, infra/mysql, transport/http) |
| `TestAuthz_*` (PublishActivate, NilAuthorizerFailsClosed, ReadRoutes, AssigneeRoles, EveryRouteGatedForTenant) | PASS. Trước khi fix thì FAIL (xem `02-repro.md`, kèm repro của T2/T3 trong cùng phiên) |
| Kiểm tra ngược | Gỡ check ở activate → 4 case FAIL. Gỡ check ở lifecycle → `EveryRouteGatedForTenant` FAIL. Khôi phục → PASS |
| `go build ./...` | PASS |
| `go test ./... -count=1` | Exit 1 ở **cả bản fix lẫn baseline**. Không có test fail mới: bản fix có 18 dòng FAIL, baseline 19 (baseline thêm `TestSchemaParityConditionalPeriodicity`, nằm ở `disclosure/app`, không liên quan) |
| `go test ./internal/httpserver/...` | 5 test fail, giống hệt baseline (template_category, CMS review state, SESSION_EXPIRED) |
| `go vet ./...` | Exit 1, chỉ do lỗi copylocks có sẵn ở `internal/workflowfulfillment/required_document_gate_test.go:326-327` |
| `go test -race -count=1 ./internal/workflowconfig/...` | PASS |
| `docker compose -f docker-compose.dev.yml build api` | PASS (image `cobo_iam_services-api` built) |
| Smoke local (tenant → 403, CMS → OK, portal vẫn hiện label) | **BLOCKED**: stack Cobo local không chạy (cổng 8080 không phản hồi; các container đang chạy thuộc project `iam-services` khác). Hành vi đã được phủ bằng handler test với wiring giống `server.go` |

Các test fail sẵn có ở baseline (không phải do fix này):
- `companyaccess/app`: `TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`
- `companyaccess/transport/http`: `TestCreateSelfServiceCompany_FeatureFlagOff`
- `notification/app`: `TestContract_VariableParity`
- `httpserver`: 5 integration test
- `disclosure/app`, liên quan template import / example:
  - `TestCanonicalTemplateImportExample_SchemaAndValidateCompatibility`
  - `TestDownloadedExampleValidates`
  - `TestServerOwnedIDsGeneratedAtConfirm`
  - `TestTemplateBuilderValidation_ReusesCMSRulesWithoutTokenOrHistory`
  - `TestTemplateImportService_TokenIssuanceLifecycle`
  - `TestTemplateImportService_ZeroDBWriteAssertion`
  - Các test này fail ngay ở HEAD và nhiều hơn baseline 8 test ghi nhận sáng nay. Nhiều khả năng chúng liên quan commit `dea4e59 fix bug import`; cần ticket riêng.

Ghi chú: `go.mod` có thay đổi ngoài scope (`golang.org/x/text` từ indirect thành direct). File bị sửa lúc 10:20, trong lúc chạy risk review, trước khi bắt đầu implement C2. Thay đổi này không thuộc fix C2 và chưa bị revert.
