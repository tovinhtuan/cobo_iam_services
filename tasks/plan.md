# Plan: Fix C2 — phân quyền cho route `workflowconfig` (phương án B: bậc quyền template CMS)

## Context
Risk review ngày 2026-10-09 (`docs/ai-cache/risk-review-2026-10-09/10-risk-report.md`, mục C2) xác nhận lỗi sau:
- Mọi route `/api/v1/platform/cms/.../workflow/*` trong `internal/workflowconfig/transport/http` chỉ kiểm tra access token (`actor()` ở `version_handler.go:121`).
- Do đó bất kỳ user nào của bất kỳ tenant nào cũng publish/activate được workflow toàn cục và tạo được assignee role toàn cục.
- `WORKFLOW_VERSIONING_ENABLED` đang là `"true"` trên server dev.
- `GET/POST assignee-roles` được đăng ký mỗi khi có MySQL, không phụ thuộc flag.

Giải pháp đã chốt nằm trong `docs/ai-cache/bug-workflowconfig-authz-2026-10-09/01-root-cause-solution.md`, theo phương án B: dùng lại bậc quyền template CMS của `internal/disclosure/app/cms_template_permissions.go`.

Mục tiêu:
- Tenant không chạm được các route ghi và route đọc CMS.
- CMS operator hiện tại vẫn làm việc bình thường. Migration 0071 đã cấp `cms.template.write/activate/archive` cho mọi role có `platform.cms.view`.
- Giữ tách maker/checker giữa publish và activate.

Quy trình: `wf-bugfix`, viết test fail trước rồi mới sửa (Gate R → fix → Gate V).

## Ma trận quyền (đích)
| Route | Yêu cầu |
|---|---|
| `GET /platform/cms/workflow/assignee-roles` | chỉ cần token, **giữ nguyên** (tenant portal dùng) |
| `POST /platform/cms/workflow/assignee-roles` | `platform.cms.view` + (`cms.template.write` \| `disclosure_type.manage`) |
| `POST .../workflow/publish` | `platform.cms.view` + (`cms.template.write` \| `disclosure_type.manage`) |
| `POST .../versions/{n}/activate` | `platform.cms.view` + (`cms.template.activate` \| `disclosure_type.publish`) |
| `GET configuration`, `readiness`, `lifecycle`, `versions`, `versions/{n}`; `POST validate` | `platform.cms.view` + bất kỳ quyền nào trong {`cms.template.read`, `.write`, `.activate`, `.archive`, `.config.write`, `disclosure_type.manage`, `rbac.manage`} |

Các mã trả về:
- Thiếu quyền: 403 `PERMISSION_DENIED`, kèm `details.required_permissions` (cùng shape với `newCMSPermissionDenied`).
- Không có token: 401 (như hiện tại).
- Authorizer nil: 503 (fail-closed).

## Dependency graph
```
T0 (ghi tasks/plan.md, cập nhật tasks/todo.md)
 └─ T1 authz helper + constructor/wiring + publish/activate   [path chính, critical]
     ├─ T2 route đọc CMS (configuration/readiness/lifecycle/versions/validate)
     ├─ T3 POST assignee-roles (GET giữ nguyên)
     └─ T4 test quét toàn bộ route (chống quên gate)   ← cần T2 + T3 xong mới pass
         └─ T5 verify baseline + docker build
             └─ T6 review subagent + premerge + ai-cache
                 └─ T7 (ops, cần user duyệt) mitigation server + audit dữ liệu read-only
```

## Tasks (cắt dọc: mỗi task gồm test fail → fix → test pass cho một nhóm route)

### T0 — Ghi artefact kế hoạch
- Tạo `tasks/plan.md` với nội dung plan này.
- **Append** section "C2 workflowconfig authz" vào `tasks/todo.md`. File này đang chứa task list của feature adhoc notifications, không được ghi đè.
- AC: hai file tồn tại; nội dung cũ của `todo.md` còn nguyên.

### T1 — Gate publish/activate (critical path)
1. **Test trước** — tạo `internal/workflowconfig/transport/http/authz_test.go` (package `http_test`). Fakes:
   - `fakeInspector` (token → `iamapp.AccessTokenClaims{Sub, MembershipID, CompanyID}`). Interface: `iamapp.TokenInspector`, `internal/iam/app/contracts.go:131`.
   - `fakeAuthorizer` (membership → `[]string` permissions). Trả `*authapp.EffectiveAccessSummary` (`internal/authorization/app/contracts.go:86`).
   - `fakeVersionRepo`, implement đủ 6 method của `wfcapp.VersionRepository` (`version_service.go:93`). Tham khảo fake có sẵn ở `internal/workflowconfig/app/version_service_test.go:15-70` nhưng chép sang, vì fake đó ở package khác.
   - Service: `wfcapp.NewVersionService(fakeRepo, nil)` → `NewReadinessService(versionSvc, wfcapp.DefaultRoleRegistry())` → `NewConfigService(...)`.
   - Personas:
     - `tenantAdmin`: `rbac.manage`, `admin.membership.invite`.
     - `tenantMember`: không có quyền nào.
     - `cmsReader`: `platform.cms.view` + `cms.template.read`.
     - `cmsMaker`: `platform.cms.view` + `cms.template.write`.
     - `cmsChecker`: `platform.cms.view` + `cms.template.activate`.
   - Case:
     - `tenantAdmin`, `tenantMember` gọi publish/activate → 403.
     - `cmsReader` gọi publish → 403.
     - `cmsMaker`: publish → 201; activate → 403.
     - `cmsChecker`: activate → 200 (hoặc khác 401/403 nếu fake repo trả conflict).
     - Không token → 401.
     - Authorizer nil → 503.
   - Chạy `go test ./internal/workflowconfig/transport/http/ -run Authz`. Phải **FAIL** ở các case kỳ vọng 403/503 (Gate R). Ghi output vào `docs/ai-cache/bug-workflowconfig-authz-2026-10-09/02-repro.md`.
2. **Fix** — tạo `internal/workflowconfig/transport/http/authz.go`:
   - `type subject struct{ UserID, MembershipID, CompanyID string }`.
   - `func (h *Handler) subject(r) (subject, error)`: dùng lại `bearer()` và `h.inspector.InspectAccessToken`; thay cho `actor()`.
   - Interface hẹp `accessResolver interface{ GetEffectiveAccess(ctx, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) }`.
   - Các helper `requireAnyPermission`, `hasAnyPermission`: theo mẫu `internal/workflowdoctemplate/transport/http/handler.go:63-88` (nil → 503, so khớp EqualFold).
   - Các helper bậc quyền: `requireTemplateRead`, `requireTemplateWrite`, `requireTemplateActivate`. Mỗi helper kiểm `platform.cms.view` trước, rồi tới tập quyền tương ứng.
   - Lỗi 403 có `Details{"required_permissions": ..., "accepted_legacy_permissions": ...}`, giống `newCMSPermissionDenied` (`disclosure/app/cms_template_permissions.go:31`).
   - Hằng permission khai báo cục bộ. **Không import `disclosure/app`**, vì `workflowconfig` cố ý độc lập (xem comment ở `version_service.go:28`).
3. Cập nhật `version_handler.go`:
   - Thêm field `authorizer accessResolver`.
   - `NewHandler(svc, cfg, catalog, inspector, authorizer)`.
   - `publish` gọi `requireTemplateWrite`; `activate` gọi `requireTemplateActivate`. Truyền `sub.UserID` làm actor.
4. Cập nhật `internal/httpserver/server.go:595`: truyền `authSvc`, cùng instance với `wdthttp.NewHandler(..., authSvc)` ở dòng 363.
5. Cập nhật `version_handler_register_test.go`: thêm đối số `nil`.
- AC:
  - Test T1 pass.
  - Revert `authz` check trong publish/activate → test fail lại.
  - `go build ./...` pass.
- **Checkpoint CP-1**: `go test ./internal/workflowconfig/...` + `go build ./...` xanh.

### T2 — Gate route đọc CMS
- **Test trước**:
  - `tenantAdmin`, `tenantMember` gọi GET configuration/readiness/lifecycle/versions/versions/1 và POST validate → 403.
  - `cmsReader` → khác 401/403.
- **Fix**: mỗi handler `configuration`, `readiness`, `validate`, `lifecycle`, `listVersions`, `versionDetail` gọi `requireTemplateRead` ngay đầu. Xoá `actor()` khi không còn ai dùng.
- AC: test T2 pass, test T1 vẫn pass.

### T3 — POST assignee-roles (GET giữ nguyên)
- **Test trước**:
  - `tenantMember` gọi GET → 200 (khoá hành vi hiện tại).
  - `tenantAdmin` gọi POST → 403.
  - `cmsMaker` gọi POST → 201. Catalog dùng `wfcapp.NewAssigneeRoleCatalogService(wfcapp.NewInMemoryAssigneeRoleCatalog())`.
- **Fix**:
  - `catalog_handler.go`: `assigneeRoles` lấy `subject` cho mọi method; nhánh POST gọi thêm `requireTemplateWrite`.
  - `RegisterAssigneeRoleCatalog(mux, catalog, inspector, authorizer)`.
  - Cập nhật `server.go:580`.
- AC: test T3 pass.
- **Checkpoint CP-2**: `go test ./internal/workflowconfig/... ./internal/httpserver/...` xanh.

### T4 — Test chống hồi quy cho mọi route
- Trong `authz_test.go`, tạo mux, gọi cả `Register` lẫn `RegisterAssigneeRoleCatalog`.
- Duyệt bảng `{method, path}` khớp 1-1 với các `HandleFunc` trong `version_handler.go:32-44`.
- Gửi request bằng token `tenantAdmin`. Mọi route trừ `GET assignee-roles` phải trả 403.
- Thêm assert số route = 10, để ai thêm route mà không cập nhật bảng thì test fail.
- AC: test pass; xoá gate ở một route bất kỳ → test fail.

### T5 — Verify (Gate V)
- `go test ./...`: so với baseline đã biết (8 test fail ở `companyaccess/app`, `companyaccess/transport/http`, `httpserver`, `notification/app`). Không được phát sinh fail mới.
- `go vet ./...`: chỉ còn lỗi copylocks có sẵn ở `workflowfulfillment/required_document_gate_test.go:326-327`.
- `go test -race ./internal/workflowconfig/...`.
- `docker compose -f docker-compose.dev.yml build api`.
- Smoke local, tuỳ chọn, chỉ trên stack local (`make dc-up`), **không chạy trên server dev**:
  - Login `admin.dn` (tenant) gọi `POST .../workflow/publish` → 403.
  - Login platform CMS admin seed → 201.
  - Mở portal tenant `DisclosureTypeDetail`: label role vẫn hiển thị (GET catalog còn 200).
  - Nếu không chạy được thì ghi `BLOCKED: <lý do>`.
- AC: tất cả mục ở trên có kết quả ghi lại trong `03-verify.md`.

### T6 — Review và đóng task
- Chạy song song `be-security-reviewer` + `admin-role-reviewer` trên diff (path map của `wf-risk-review`).
- Sau đó chạy `premerge-system-review` (bản ngắn).
- Grep lại `HandleFunc(".* /api/v1/platform/cms` để xác nhận không còn route platform nào chỉ kiểm token. C3 adhoc là ngoài scope, giữ ticket riêng.
- Cập nhật ai-cache:
  - `docs/ai-cache/bug-workflowconfig-authz-2026-10-09/` (02-repro, 03-verify, completion report).
  - Đánh dấu C2 "fixed in branch" trong `risk-review-2026-10-09/10-risk-report.md`.
- Không commit/push nếu user chưa yêu cầu.

### T7 — Ops (chỉ thực hiện khi user duyệt từng bước)
- Mitigation trước khi deploy fix:
  - Tắt `WORKFLOW_VERSIONING_ENABLED` trên server.
  - Cân nhắc chặn POST assignee-roles qua nginx (chỉ hiệu quả nếu 8080 không còn mở thẳng).
- Audit read-only trên DB server, tìm:
  - Version publish/activate có `actor` không có `platform.cms.view`.
  - Assignee role do user ngoài CMS tạo.
  - Nếu thấy bất thường thì lập kế hoạch rollback bằng API activate, dùng CMS admin.

## File chính sẽ thay đổi
- **Mới**: `internal/workflowconfig/transport/http/authz.go`, `internal/workflowconfig/transport/http/authz_test.go`.
- **Sửa**: `internal/workflowconfig/transport/http/version_handler.go`, `catalog_handler.go`, `version_handler_register_test.go`, `internal/httpserver/server.go` (2 dòng wiring).
- **Docs**: `tasks/plan.md` (mới), `tasks/todo.md` (append), `docs/ai-cache/bug-workflowconfig-authz-2026-10-09/*`.
- **Không đổi**: service layer, schema/migration, FE, cờ feature.

## Code dùng lại
- `workflowdoctemplate/transport/http/handler.go:63-94`: mẫu `requireAnyPermission`, `hasAnyPermission`, `requireCMSEditor`, fail-closed khi authorizer nil.
- `disclosure/app/cms_template_permissions.go`: tập quyền và shape lỗi 403 (chép hằng, không import).
- `authapp.Service.GetEffectiveAccess`: `authorization/app/contracts.go:20`.
- `wfcapp.NewInMemoryAssigneeRoleCatalog`, `wfcapp.DefaultRoleRegistry`: dùng cho test.

## Rủi ro và lưu ý
- **CMS operator mất quyền:** thấp, nhờ migration 0071. T5 smoke với seed CMS admin sẽ xác nhận.
- **Tenant portal:** GET catalog giữ nguyên. Nếu sau này siết thêm, `authApi.ts:72-74` đã fail-soft khi gặp 403.
- **FE CMS:** publish/activate có thể nhận 403 khi operator thiếu quyền. Lỗi load đã được phân loại trong `workflowConfigurationLoadError.ts`. Thông báo cho publish/activate chưa có mapping riêng, ghi follow-up FE (không làm trong PR này).
- **Effective access dùng cache Redis TTL 5':** quyền vừa bị thu hồi vẫn có hiệu lực tới 5 phút. Đây là finding H17, không xử lý ở đây.

## Verification end-to-end (tóm tắt)
1. `go test ./internal/workflowconfig/... -run Authz -v`: fail trước fix, pass sau fix.
2. `go test ./...`, `go vet ./...`, `go test -race ./internal/workflowconfig/...`.
3. `docker compose -f docker-compose.dev.yml build api`.
4. Smoke local: tenant → 403, CMS → OK, portal label vẫn hiển thị.
5. Reviewer subagent không báo CRITICAL/HIGH mới trên diff.


---

# Plan (C3): Fix C3 — gỡ endpoint `adhoc-migrate-legacy-approvals` (phương án A′, xử lý trong phạm vi tenant)

## Context
Risk review 2026-10-09 (C3) và bản phân tích `docs/ai-cache/bug-adhoc-legacy-migrate-authz-2026-10-09/01-root-cause-solution.md` cho thấy vấn đề ở `POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals` (`internal/adhoc/transport/http/handler.go:48,302-333`):
- Endpoint quét proposal `pending_admin_approval` của **mọi tenant** (`repository.go:640`, query không lọc company).
- Rồi tự duyệt bằng cách **mạo danh process controller** của từng tenant (`service.go:1007-1041`).
- Gate chỉ là `authorize(action="rbac.manage")`. Action này fallback về `system.settings`, chỉ đánh giá trong company của người gọi, và không kiểm `platform.cms.view`.

Yêu cầu user: **không được tác động sang tenant khác nếu không có permission trong chính tenant đó.**

User đã chốt:
- **A′**: gỡ hẳn endpoint, không thay bằng CLI duyệt hộ.
- Proposal legacy chỉ được xử lý trong tenant:
  - người tạo **Rút** (UI đã có, `AdHocProposalDetailPage.tsx:309-313`);
  - hoặc process controller gọi `POST /api/v1/company/ad-hoc-proposals/{id}/admin-approve` (company lấy từ token, có check `ProcessControllerID`, `service.go:618`).
- **Chỉ làm backend.** Nút duyệt legacy trên FE là follow-up.
- **Proposal mồ côi** để nguyên, ghi follow-up.

Quy trình: `wf-bugfix`, viết test fail trước rồi mới sửa.

Kết quả mong muốn: không còn route hay code path nào trong adhoc thao tác lên company khác company của token; API cho tenant giữ nguyên.

## Dependency graph
```
T0 artefact (append tasks/plan.md + tasks/todo.md)
 └─ T1 Gate R: test tái hiện ở mức route (FAIL trên code hiện tại)
     └─ T2 Gỡ endpoint + service + repo + fakes (test T1 PASS)
         └─ T3 Test khoá hành vi tenant-scope (admin-approve, không còn route /platform trong adhoc)
             └─ T4 Verify (Gate V) + quét tàn dư
                 └─ T5 Review subagent + premerge + ai-cache
                     └─ T6 Ops/follow-up (chỉ khi user duyệt từng bước)
```

## Tasks

### T0 — Artefact kế hoạch
- **Append** section "C3 adhoc legacy migrate" vào `tasks/plan.md` và `tasks/todo.md`.
- Không ghi đè plan C2 và task list adhoc notifications đang có trong hai file.
- AC: nội dung cũ còn nguyên; section C3 có đủ T0–T6.

### T1 — Gate R: tái hiện lỗi (test trước)
- Thêm vào `internal/adhoc/transport/http/handler_test.go` test `TestLegacyMigrationRoute_NotExposed`:
  - Dùng `NewHandler(discardLogger(), svc, fakeInspector{}, nil)` + `Register(mux)`, giống `TestAdminApprove_UsesFallbackIdempotencyKeyAndCompletesReservation` (`:165`).
  - `POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals` với token tenant → kỳ vọng **404**.
- Để bằng chứng rõ hơn, khi chạy Gate R thì cho fake `ListPendingLegacyApprovals` trả 1 dòng `{proposal-B, company-B}`, và `FinalizeLegacyApproval` ghi lại `companyID`.
- Kỳ vọng trên code hiện tại: **FAIL**, nhận 200 `{"processed":1}`, fake ghi `company-B` từ token của tenant khác.
- Ghi output vào `docs/ai-cache/bug-adhoc-legacy-migrate-authz-2026-10-09/02-repro.md`.
- AC: test fail đúng lý do (200 thay vì 404) và có bằng chứng `company-B`.

### T2 — Gỡ endpoint và toàn bộ code path cross-tenant
Các thay đổi:
- `internal/adhoc/transport/http/handler.go`:
  - Xoá route ở `:46-48` (cả comment "platform admin only").
  - Xoá handler `migrateLegacyApprovals` (`:296-333`).
- `internal/adhoc/app/contracts.go`:
  - Xoá `FinalizeLegacyApproval`, `ListPendingLegacyApprovals` khỏi interface `Service` (`:37-43`).
  - Xoá type `PendingApprovalRow` (`:50-55`).
  - Xoá `ListPendingAdminApproval` khỏi interface repository (`:86-88`).
  - Sửa doc `AdminApprove` (`:26-30`): chỉ còn phục vụ legacy client.
- `internal/adhoc/app/service.go`:
  - Xoá `FinalizeLegacyApproval` và `ListPendingLegacyApprovals` (`:1007-1049`).
  - Sửa comment `@deprecated` của `AdminApprove` (`:606-610`).
- `internal/adhoc/infra/mysql/repository.go`: xoá `ListPendingAdminApproval` (`:638-660`). Đây là query không scope tenant duy nhất trong adhoc.
- Fakes:
  - `handler_test.go:67-73` (xoá 2 method và phần tạm của T1).
  - `service_test.go:179`, `:1399-1423` (`concurrencyFakeRepo`), `:1811` (`raceFakeRepo`).
  - Xoá `TestFinalizeLegacyApproval_DelegatesToAdminApprove` (`service_test.go:741-757`). Hành vi `AdminApprove` đã có test riêng (`:759`, `:1230`, `:1611`…).
- AC:
  - `go build ./...` pass.
  - Test T1 pass (404).
  - Revert route → T1 fail lại.
- **Checkpoint CP-1**: `go build ./...` + `go test ./internal/adhoc/... -count=1` xanh. `adhoc/app` có test race flaky sẵn (PERF-10); nếu gặp thì chạy lại và ghi chú.

### T3 — Test khoá hành vi tenant-scope
1. `TestAdminApprove_UsesTokenCompanyIgnoringBodySubject` (handler):
   - Body chứa `{"Subject":{"CompanyID":"company-B","MembershipID":"m-B"}}`. `adhocapp.Subject` không có json tag nên JSON decode được vào đó.
   - Assert service nhận `Subject` lấy từ token (`fakeInspector`), không phải từ body. Khoá dòng `body.Subject = sub` (`handler.go:212`).
2. `TestRegister_NoPlatformRoutes` (handler):
   - Đọc các file `*.go` không phải test trong `internal/adhoc/transport/http`.
   - Assert không có chuỗi `/api/v1/platform/`, để module tenant không thể đăng ký lại route platform.
3. Đã có sẵn, chỉ chạy lại: `TestAdminApprove_ByNonController_Returns403` (`service_test.go:1230`).
4. Repo: xác nhận bằng đọc code rằng `FindByID` / `ReserveAdminApproval` có `WHERE proposal_id = ? AND company_id = ?` (`repository.go:120,328`). Package này không có test MySQL; ghi nhận trong verify.
- AC: test mới pass; xoá `body.Subject = sub` → test 1 fail.
- **Checkpoint CP-2**: `go test ./internal/adhoc/... -count=1` xanh.

### T4 — Verify (Gate V)
- `grep -rn "adhoc-migrate-legacy\|FinalizeLegacyApproval\|ListPendingLegacyApprovals\|ListPendingAdminApproval\|PendingApprovalRow" internal cmd`: không còn kết quả. Chỉ được còn trong docs.
- `go test ./... -count=1` so với baseline HEAD (dùng worktree tạm trong scratchpad như ở C2). Không được có test fail mới.
- `go vet ./...`: chỉ còn lỗi copylocks có sẵn ở `workflowfulfillment/required_document_gate_test.go:326-327`.
- `go test -race -count=1 ./internal/adhoc/...`: có thể gặp PERF-10 flaky có sẵn, ghi rõ nếu gặp.
- `docker compose -f docker-compose.dev.yml build api`.
- Smoke local nếu stack chạy được: POST route → 404; luồng admin-approve của tenant vẫn chạy. Không chạy được thì ghi `BLOCKED:`.
- Ghi kết quả vào `03-verify.md`.

### T5 — Review và đóng task
- Chạy song song trên diff:
  - `be-security-reviewer`: không còn đường cross-tenant trong adhoc; không đụng tới authz của các route tenant.
  - `admin-role-reviewer`: phân định platform vs tenant.
  - `api-compat-reviewer`: gỡ route không phá client nào (FE/Postman không dùng); comment deprecated hợp lý.
- Sau đó chạy `premerge-system-review` bản ngắn.
- Cập nhật ai-cache:
  - `03-verify.md` + completion report trong thư mục bug.
  - Sửa C3 trong `risk-review-2026-10-09/10-risk-report.md`: severity hiệu chỉnh HIGH (có điều kiện), cơ chế gate thật (`system.settings` qua fallback), trạng thái "fixed in branch".
- Không commit/push khi user chưa yêu cầu. `go.mod` (`x/text`) để commit riêng, không gộp.

### T6 — Ops và follow-up (mỗi bước cần user duyệt)
- **Mitigation trước khi deploy:**
  - Chặn route ở nginx (chỉ hiệu quả nếu cổng 8080 không mở thẳng, xem H23).
  - Thu hồi `system.settings` khỏi role demo `role_org_admin_001`.
- **Query read-only trên DB server dev:**
  - Đếm `pending_admin_approval` theo company.
  - Tìm dấu vết `adjustment_note='Auto-approved by migration 0098 (D9)'`.
  - Liệt kê role có `system.settings`.
  - Kiểm tra bảng `action_policy_matrix` có tồn tại không.
- **Nếu còn dòng `pending_admin_approval`:** báo tenant tương ứng để người tạo rút hoặc controller duyệt.
- **Follow-up ticket:**
  - (a) Nút "Duyệt (legacy)" cho process controller trên FE.
  - (b) Hướng xử lý proposal mồ côi.
  - (c) `companyaccess/app/admin_service.go:983,1008,1026` dùng `authorize("rbac.manage")` nên thực tế cần `system.settings`.
  - (d) Authorizer bỏ qua `Resource`; đây là gốc của C4, C5, H2.

## File chính sẽ thay đổi
- `internal/adhoc/transport/http/handler.go`, `handler_test.go`
- `internal/adhoc/app/contracts.go`, `service.go`, `service_test.go`
- `internal/adhoc/infra/mysql/repository.go`
- Docs: `tasks/plan.md`, `tasks/todo.md` (append); `docs/ai-cache/bug-adhoc-legacy-migrate-authz-2026-10-09/{02-repro,03-verify}.md`; `docs/ai-cache/risk-review-2026-10-09/10-risk-report.md`
- **Không đổi**: schema/migration, FE, config/flag, các route `/api/v1/company/ad-hoc-proposals/*`.

## Code dùng lại
- Scaffolding test handler: `fakeService`, `fakeInspector`, `fakeIdemStore`, `discardLogger` (`handler_test.go`).
- Mẫu test quét source chống hồi quy: `TestAuthz_EveryRouteGatedForTenant` (`internal/workflowconfig/transport/http/authz_test.go`, C2).
- Kiểm danh tính process controller có sẵn: `AdminApprove` (`service.go:618`).

## Tương thích và rollback
- Gỡ route là tương thích ngược: FE (`cobo_web_design/src`) và Postman không gọi route này. Đây là route chạy tay, có trong các doc ai-cache.
- Không có migration. Thứ tự deploy API/worker không ảnh hưởng.
- Rollback bằng redeploy bản cũ sẽ mở lại lỗ hổng, nên giữ rule nginx của T6 cho tới khi bản fix ổn định.

## Verification end-to-end (tóm tắt)
1. T1 FAIL trước fix (200, `company-B`), PASS sau fix (404).
2. `go test ./internal/adhoc/...`, `go test ./...` (không có fail mới so với baseline), `go vet ./...`, `-race` cho adhoc.
3. `docker compose -f docker-compose.dev.yml build api`.
4. Grep không còn symbol legacy trong `internal/` và `cmd/`.
5. Ba reviewer không báo CRITICAL/HIGH mới trên diff.


---

# Plan (C4 + H2): giới hạn thao tác cross-company trong companyaccess cho platform operator

## Context
- Nguồn: risk review 2026-10-09 (C4, H2); tài liệu `docs/ai-cache/bug-cross-tenant-membership-create-2026-10-09/{00-report,01-root-cause-solution}.md`.
- Vấn đề: một số route và service trong `internal/companyaccess` cho phép thao tác trên company khác company của token. Lý do là service dùng `rbac.manage` (quyền tenant) làm tín hiệu "platform admin", và route tenant nhận `company_id` từ client.
- Mục tiêu: chỉ **platform operator** mới thao tác cross-company hoặc no-company. Route tenant `/api/v1/admin/*` luôn dùng company của token.
- Quyết định đã chốt:
  1. Platform operator = `platform.cms.view` **và** (`rbac.manage` | `system.settings`).
  2. Route tenant nhận company khác token → 403 `COMPANY_SCOPE_MISMATCH`.
  3. Giữ `POST /api/v1/admin/memberships`, ép company theo token.
  4. Gộp sửa H2.
- Phạm vi: chỉ backend `cobo_iam_services`. FE không đổi: caller tenant luôn gửi company của token, CMS dùng `/api/v1/platform/cms/*`.
- Quy trình: `wf-bugfix`, viết test fail trước rồi mới sửa.

## Dependency graph
```
T0 artefact
 └─ T1 helper isPlatformCompanyOperator + resolveTargetCompany
     ├─ T2 CreateUser
     ├─ T3 CreateMembership + AssignUserToCompany
     ├─ T4 H2: ListCompanyMemberships + authorizeMembershipInvite
     └─ T5 InviteUser / ListInviteRoles / ResendUserInvitation
         └─ T6 handler tenant: kiểm company ở biên + test quét route
             └─ T7 cập nhật test cũ, giữ luồng CMS xanh
                 └─ T8 verify (Gate V)
                     └─ T9 review + ai-cache (+ deploy DEV khi được duyệt)
```

## Personas cho test
Dùng `fakeAuthService{decision, permissions}` có sẵn ở `internal/companyaccess/app/admin_service_test.go:19-46`. Subject mặc định thuộc `c_001`.

| Persona | Permissions |
|---|---|
| `tenantAdmin` | `rbac.manage`, `admin.membership.invite` |
| `tenantAdminSys` | `system.settings`, `admin.membership.invite` |
| `cmsViewOnly` | `platform.cms.view`, `admin.membership.invite` (không đủ điều kiện operator) |
| `platformOp` | `platform.cms.view`, `rbac.manage`, `admin.membership.invite` |

Kỳ vọng chung:
- `platformOp` được dùng company khác và no-company.
- Các persona còn lại: company khác → 403 `COMPANY_SCOPE_MISMATCH`; rỗng → ép về `c_001` (hoặc 403 nếu hàm không có nghĩa "ép về").

## Tasks

### T0: Artefact
- Append plan này vào `tasks/plan.md` và task list vào `tasks/todo.md`. Không ghi đè nội dung cũ (C2, C3, adhoc notifications).

### T1: Helper (file mới `internal/companyaccess/app/admin_service_company_scope.go`)
- `isPlatformCompanyOperator(ctx, sub) (bool, error)`:
  - Đọc `GetEffectiveAccess` một lần.
  - Trả `platform.cms.view && (rbac.manage || system.settings)`.
  - Quyền nào thiếu trong eff thì xét qua overlay break-glass, giống `hasPermission` (`admin_service.go:765-776`, dùng lại `hasBreakGlassPermissionOverlay`).
- `resolveTargetCompany(ctx, sub, requested string, allowNoCompany bool) (string, error)`:
  - Operator: giữ `requested`. Rỗng chỉ hợp lệ khi `allowNoCompany`, ngược lại dùng `sub.CompanyID`.
  - Không phải operator: `""` hoặc `== sub.CompanyID` → `sub.CompanyID`; khác → `perr.NewHTTPError(403, perr.CodeCompanyScopeMismatch, ...)`.
- Test `admin_service_company_scope_test.go`: bảng persona × {own, other, empty} × allowNoCompany.
- AC: test helper pass.

### T2: CreateUser (`admin_service.go:74-104`)
- **Test trước** (phải FAIL trên code hiện tại):
  - `tenantAdmin` và `tenantAdminSys` với company khác → 403, repo không thêm user.
  - Company rỗng → membership thuộc `c_001`.
- **Test giữ hành vi:** `platformOp` tạo được ở company khác và tạo user no-company.
- Ghi output FAIL vào `docs/ai-cache/bug-cross-tenant-membership-create-2026-10-09/02-repro.md`.
- **Fix:** thay khối `isWebAdmin` bằng `req.CompanyID, err = s.resolveTargetCompany(ctx, req.Subject, req.CompanyID, true)`. Phần validate role phía sau giữ nguyên.
- AC: test pass; revert fix → test FAIL lại.

### T3: CreateMembership (`:873-889`) và AssignUserToCompany (`:781-871`)
- **Test trước:** `tenantAdmin` với company khác → 403, không có membership mới. Cùng company → tạo được.
- **Test giữ hành vi:** `platformOp` với company khác → tạo được (luồng CMS `assign-company`, `companies/{id}/members`).
- **Fix:** gọi `resolveTargetCompany(..., allowNoCompany=false)` ngay đầu hàm, trước `authorize`, rồi dùng company đã resolve cho mọi bước ghi.
- AC: như T2.

### T4: H2, ListCompanyMemberships (`:1048-1090`) và authorizeMembershipInvite (`admin_service_invite_scope.go:25-34`)
- **Test trước:**
  - `tenantAdmin` liệt kê company khác → 403.
  - `ListWithoutCompany` với tenant → 403.
  - Liệt kê company của mình → OK.
- **Test giữ hành vi:** `platformOp` liệt kê company khác và no-company → OK.
- **Fix:**
  - `authorizeMembershipInvite`: operator → nil. Còn lại: `companyID` phải == `sub.CompanyID` (403 nếu khác), rồi `authorize`.
  - `ListWithoutCompany`: thay check `rbac.manage` bằng `isPlatformCompanyOperator`.
- AC: như T2.

### T5: InviteUser (`:325-380`), ListInviteRoles (`:632-660`), ResendUserInvitation (`:664-700`)
- **Test trước:** `tenantAdmin` với company khác hoặc no-company → 403 (hoặc ép về company mình, theo semantics từng hàm). Không gửi email hoặc invite mới khi bị từ chối.
- **Test giữ hành vi:** `platformOp` → như cũ (luồng CMS invite, roles, resend).
- **Fix:** thay `isWebAdmin = hasPermission("rbac.manage")` bằng `isPlatformCompanyOperator` hoặc `resolveTargetCompany`. Sửa comment nói "web admin (rbac.manage)".
- AC: như T2.

### T6: Handler tenant (`internal/companyaccess/transport/http/admin_handler.go`)
- `createUser` (`:160-217`), `createMembership` (`:235-254`): body `company_id` khác rỗng và khác `sub.CompanyID` → 403 `COMPANY_SCOPE_MISMATCH`; rỗng → dùng `sub.CompanyID`.
- `listMemberships` (`:345`): path `company_id` khác `sub.CompanyID` → 403.
- Handler CMS (`internal/platformcms`) **không đổi**.
- Test handler (theo mẫu `admin_handler_memberships_test.go`): bảng các route tenant nhận company → company khác token trả 403, company của token trả 2xx.
- Thêm assert đếm route để route mới nhận `company_id` phải được thêm vào bảng.
- AC: test pass; bỏ check ở một handler → test FAIL.

### T7: Cập nhật test cũ và giữ luồng CMS
- `TestAdminService_CreateUser_WebAdminCanCreateOtherCompany` (`admin_service_test.go:338`): đổi tên thành `..._PlatformOperatorCanCreateOtherCompany`, permissions thêm `platform.cms.view`.
- `TestAdminService_CreateUser_WebAdmin_NoMembershipWhenCompanyOmitted` và `TestAdminService_ListCompanyMemberships_ListWithoutCompany`: đổi sang persona `platformOp`, thêm case tenant → 403.
- Rà `rbac_phase_e_assign_invite_test.go` và `admin_service_invite_focal_test.go`: test nào dựa vào `rbac.manage` để thao tác cross-company hoặc no-company thì đổi persona; test cùng company giữ nguyên.
- Giữ xanh `TestIntegration_platformCMSPrefix_adminUsersCreateAndList` (`internal/httpserver/server_test.go:984`).
- AC: `go test ./internal/companyaccess/... ./internal/platformcms/...` xanh. Danh sách FAIL của `httpserver` giống baseline (5 test có sẵn).

### T8: Verify (Gate V)
- `go build ./...`; `go test ./... -count=1` so với baseline HEAD (worktree tạm), không có fail mới.
- `go vet ./...` (chỉ còn lỗi copylocks có sẵn); `go test -race -count=1 ./internal/companyaccess/...`.
- `docker compose -f docker-compose.dev.yml build api`.
- Grep: không còn `hasPermission(ctx, .*"rbac.manage")` dùng làm tín hiệu cross-company trong `internal/companyaccess/app`. Ghi lại các chỗ còn lại là quyền tenant hợp lệ.
- Ghi `03-verify.md`.

### T9: Review và đóng
- Chạy song song `be-security-reviewer`, `admin-role-reviewer`, `api-compat-reviewer` (vì có mã lỗi 403 mới) trên diff `internal/companyaccess`. Sau đó `premerge-system-review` bản ngắn.
- ai-cache: `04-completion.md`; đánh dấu C4 và H2 "fixed in branch" trong `risk-review-2026-10-09/10-risk-report.md`.
- Không commit hay push khi chưa được yêu cầu.
- Deploy DEV và smoke chỉ khi user yêu cầu, theo `wf-release`:
  - Tenant gửi company khác → 403.
  - Tenant tạo user trong company mình → 201.
  - CMS tạo, mời, liệt kê cho company khác → OK.
- Follow-up (ngoài PR): C5 (IDOR theo membership id, gán global role); authorizer bỏ qua `Resource` company; xử lý 9 user mồ côi trên DEV (thao tác ghi, cần duyệt riêng).

## File chính
- **Mới:** `internal/companyaccess/app/admin_service_company_scope.go`, `admin_service_company_scope_test.go`.
- **Sửa:** `internal/companyaccess/app/admin_service.go`, `admin_service_invite_scope.go`, `internal/companyaccess/transport/http/admin_handler.go`, các test liên quan trong `internal/companyaccess/{app,transport/http}`.
- **Không đổi:** schema/migration, FE, `internal/platformcms` handler, cờ cấu hình.

## Code dùng lại
- `hasPermission`, `hasBreakGlassPermissionOverlay` (`admin_service.go:765+`).
- `isPlatformCMSOperator` (`admin_service.go:628`): dùng cho validate role, giữ nguyên.
- `perr.CodeCompanyScopeMismatch` (`internal/platform/errors`).
- `fakeAuthService` (`admin_service_test.go:19-46`).
- Mẫu test quét route: `internal/workflowconfig/transport/http/authz_test.go` (C2).

## Tương thích và rollback
- FE hiện tại không bị ảnh hưởng.
- Các role có `rbac.manage` nhưng thiếu `platform.cms.view` (22 role trên DEV) mất khả năng thao tác cross-company hoặc no-company qua route tenant. Đây là hành vi mong muốn.
- Không có migration. Rollback bằng binary backup sẽ mở lại lỗ hổng.


---

# Plan (C5): ràng buộc company cho thao tác theo `membership_id` và khuyết điểm team/department/title

## Context
- Nguồn: risk review 2026-10-09 (C5, ROLE-04, BES-03, ROLE-05); tài liệu `docs/ai-cache/bug-membership-id-company-scope-2026-10-09/{00-report,01-root-cause-solution,02-data-audit}.md`. Tiếp nối C4.
- Vấn đề: nhiều service trong `internal/companyaccess/app` nhận `membership_id` (hoặc id team/department/title) mà không kiểm tra đối tượng thuộc company của token. Authorizer bỏ qua resource, và `authorizeScopedMembershipMutation` trả `nil` ngay khi scope là "company". Phía SQL, nhiều câu `UPDATE/DELETE/SELECT` theo `membership_id` không có `company_id`.
- Phạm vi tiếp cận (đã grep): chỉ `AdminHandler` tenant gọi các hàm này, nên **một quy tắc duy nhất**: đối tượng phải thuộc `Subject.CompanyID`, không cần ngoại lệ cho platform operator.
- Quyết định đã chốt:
  1. Đối tượng ngoài company → **404 `MEMBERSHIP_NOT_FOUND`** (không lộ id có tồn tại).
  2. PR gồm **lớp 1** (service guard) và **lớp 2** (sửa SQL/repo cho thao tác phá huỷ và các lỗi riêng lẻ).
  3. Gộp khuyết điểm team, department, title vào cùng PR.
  4. Đã audit DEV read-only: toàn vẹn quan hệ sạch (0 dòng lệch), không có dấu vết thao tác chéo company.
- **Đính chính so với tài liệu:** mục "bổ sung audit cho thao tác phá huỷ" trong `01-root-cause-solution.md` là không cần. Đã đọc code: mọi handler phá huỷ (`deleteMembership`, `removeRole`, `removeTitle`, `removeDepartment`, thành viên team/title…) **đã có `auditLog`**. DEV không có event vì chưa ai dùng. Sửa tài liệu ở T0.
- Phạm vi code: chỉ `cobo_iam_services`; FE không đổi (mọi caller tenant gửi id lấy từ danh sách của company đang chọn). Không có migration.
- Quy trình: `wf-bugfix`, viết test fail trước rồi mới sửa (Gate R, Gate V).

## Dependency graph
```
T0 artefact + đính chính doc
 └─ T1 fixture 2 company + test bảng cross-company (Gate R)  [nâng cấp in-memory repo: team/dept/title có state]
     └─ T2 guard tập trung (requireTargetMembership + authorizeScopedMembershipMutation)
         ├─ T3 guard tường minh cho các hàm còn lại + config approval
         ├─ T4 team/department/title: validate, count, 404, DeleteTeamRow (lớp 2)
         └─ T5 UpdateMembershipStatus / DeleteMembership ràng buộc company_id ở SQL (lớp 2)
             └─ T6 validate theo sub.CompanyID (primary-role, org-assignments) + test quét route handler
                 └─ T7 cập nhật test cũ dùng membership giả
                     └─ T8 verify (Gate V)
                         └─ T9 review + ai-cache (+ deploy DEV và smoke khi được yêu cầu)
```

## Quy ước test
- Package `app_test`, dựng trên `fakeAuthService` và `newScopeSvc`/`seedInviteScopedSubject` của C4 (`admin_service_company_scope_test.go`).
- **Fixture chung** `newTwoCompanyFixture(t, persona)`: người gọi ở `c_001`; một membership "đích" thật ở `c_002` (có role, phòng ban, chức danh, direct permission, thành viên team); một membership đối chứng ở `c_001`.
- **Persona:** `tenantAdmin` (`rbac.manage`), `tenantAdminSys`, `delegated` (không có `rbac.manage`, có quyền theo delegation phòng ban, để đi qua nhánh scope phòng ban), `platformOp`.
- **Khẳng định chung cho mỗi hàm bị ngoài company:** lỗi là 404 `MEMBERSHIP_NOT_FOUND` (hoặc 404 tương ứng với team/department/title) **và** dữ liệu của đối tượng đích giữ nguyên (đọc lại qua repo): trạng thái membership, `ListMembershipRoles`, department và title (`ListActiveMembership*IDs`), `ListActiveDirectPermissions`, thành viên team.

## Tasks

### T0: Artefact và đính chính tài liệu
- Append plan này vào `tasks/plan.md` và task list vào `tasks/todo.md`. Không ghi đè C2/C3/C4/adhoc.
- Sửa `01-root-cause-solution.md`: bỏ đề xuất bổ sung audit, ghi rõ handler đã có `auditLog`.
- AC: nội dung cũ của hai file tasks còn nguyên.

### T1: Fixture, nâng cấp in-memory repo, test bảng cross-company (Gate R)
- Nâng cấp `internal/companyaccess/infra/inmemory/admin_repository.go`: các hàm team/department/title đang là stub (no-op, ví dụ `DeleteTeamRow`, `AddTeamMember`, `RemoveTeamMember`, `CountTeamsInDepartment`, `CountDepartmentMembers`, `CountTitleMembers`, `SoftDeleteDepartment`, `SoftDeleteTitle`). Thêm state tối thiểu để test khẳng định được dữ liệu đích không đổi: `teams` (id → company, department), `teamMembers`, và đếm thành viên theo department/title. Repo MySQL không có công cụ test SQL (không `sqlmock`), nên phần SQL được kiểm bằng đọc code và smoke DEV (T9).
- Test mới `admin_service_membership_scope_test.go`, bảng theo hàm (mỗi dòng: tên hàm, hàm gọi, kỳ vọng):
  - `UpdateMembership`, `DeleteMembership`, `AssignRole`, `RemoveRole`, `ReplaceMembershipPrimaryRole`, `AssignDepartment`, `RemoveDepartment`, `AssignTitle`, `RemoveTitle`, `UpdateMembershipOrgAssignments`, `AddDirectPermission`, `RemoveDirectPermission`, `ListDirectPermissions`, `RevokeCompanyAdmin`, `AddTeamMember`, `RemoveTeamMember`, `RemoveTitleMember`, `AddDeptMember`/`RemoveDeptMember` (nhánh `delegated`).
  - Mỗi hàm: membership của `c_002` → 404 và dữ liệu không đổi; membership của `c_001` → chạy như cũ (một case đại diện mỗi hàm).
- Chạy trên code hiện tại: nhóm "ngoài company" phải **FAIL**. Ghi output vào `docs/ai-cache/bug-membership-id-company-scope-2026-10-09/03-repro.md`.
- AC: test biên dịch, FAIL đúng lý do; nhóm "cùng company" PASS.

### T2: Guard tập trung
- Thêm vào `admin_service_company_scope.go`: `requireTargetMembership(ctx, sub, membershipID)` = fail-closed khi `sub.CompanyID` rỗng + `requireMembershipInCompany(membershipID, sub.CompanyID)` (`admin_service_departments.go:129`, trả 404 `MEMBERSHIP_NOT_FOUND`).
- Gọi helper ở **đầu** `authorizeScopedMembershipMutation` (`admin_delegation_scope.go:125`), trước nhánh scope. Một thay đổi phủ: `UpdateMembership`, `DeleteMembership`, `ReplaceMembershipPrimaryRole`, `AssignDepartment`, `RemoveDepartment`, `UpdateMembershipOrgAssignments`, và nhánh delegation của `AddDeptMember`/`RemoveDeptMember` (`authorizeDeptMemberMutation`).
- AC: các dòng tương ứng trong test T1 chuyển PASS; bỏ lời gọi → FAIL lại.
- **Checkpoint CP-1:** `go test ./internal/companyaccess/...` chỉ còn 2 fail có sẵn và các dòng T3 đến T5 chưa sửa.

### T3: Guard tường minh cho các hàm còn lại và config approval
- Thêm `requireTargetMembership` sau bước `authorize`/`requireRbacManage`, trước mọi lookup hoặc ghi, ở: `AssignRole`, `RemoveRole`, `AssignTitle`, `RemoveTitle`, `AddDirectPermission`, `RemoveDirectPermission`, `ListDirectPermissions`, `RevokeCompanyAdmin`, `AddTeamMember`, `RemoveTeamMember`, `RemoveTitleMember`.
- `SubmitConfigApproval` loại `RBAC_DIRECT_PERM_REMOVE` (`config_approval.go:254`): validate `membership_id` trong body bằng `requireTargetMembership` ngay khi submit.
- Kiểm tra `RevokeCompanyAdmin`, `TransferOwnership`: đích đã được kiểm tra; thêm test cho đủ.
- AC: các dòng T3 trong test PASS; kiểm tra ngược (gỡ guard ở 2 hàm đại diện → FAIL).

### T4: Team, department, title (lớp 2)
- **Test trước** (FAIL trước):
  - `DeleteTeam` với team của company khác: trả 404 **và** thành viên team đó còn nguyên.
  - `CreateTeam` với department của company khác → 404, không tạo; `CountTeamsInDepartment` chỉ đếm trong company.
  - `AddTeamMember`/`RemoveTeamMember` với team ngoài company → 404.
  - `DeleteDepartment`/`DeleteTitle` với id ngoài company → 404 (không còn 409 "có thành viên").
- **Fix:**
  - `DeleteTeamRow` (`infra/mysql/admin_repository_teams.go:90`): trong một transaction, kiểm `org_units` thuộc company **trước**, rồi mới xoá `org_unit_memberships`.
  - `RemoveTeamMember`: dùng tham số company đang bị bỏ qua (`_`) qua join với `org_units`.
  - Repo thêm `TeamBelongsToCompany(ctx, companyID, teamID) (bool, error)` (MySQL: `org_units` với `unit_type='team'`; in-memory theo state mới). Service `AddTeamMember`/`RemoveTeamMember` gọi hàm này.
  - `CreateTeam`: validate department bằng `departmentInCompany` (`config_delegation.go:238`, dùng lại).
  - `CountDepartmentMembers`/`CountTitleMembers`/`CountTeamsInDepartment`: thêm tham số company (đổi chữ ký ở `app/admin.go:247,254,261`, hai implementer). Service kiểm tra tồn tại theo company **trước** khi đếm, để id ngoài company trả 404.
- AC: test T4 PASS; `go build ./...` xanh với chữ ký mới.

### T5: `UpdateMembershipStatus` và `DeleteMembership` ràng buộc `company_id` ở SQL (lớp 2)
- Đổi chữ ký ở `app/admin.go:182-183` thành nhận `companyID`; cập nhật implementer MySQL (`admin_repository.go:123,137`) và in-memory (`:346,355`).
  - MySQL: `UPDATE memberships SET … WHERE membership_id=? AND company_id=?`; `DeleteMembership` xoá bảng con qua membership thuộc company, trong cùng transaction.
  - Không khớp company → trả lỗi không tìm thấy (map sang 404 ở service).
- **Test trước:** gọi repo in-memory với company sai không thay đổi dữ liệu (test mức repo trong `infra/inmemory`).
- AC: test PASS; `go build ./...` xanh.

### T6: Validate theo company của token và test quét route handler
- `ReplaceMembershipPrimaryRole` (`admin_service_primary_role.go:41`) và `UpdateMembershipOrgAssignments` (`admin_service_membership_org.go:154`): đổi `member.CompanyID` thành `sub.CompanyID` khi validate role, phòng ban, chức danh.
- Test handler `admin_handler_membership_scope_test.go` (mẫu `admin_handler_company_scope_test.go` của C4): bảng mọi route tenant có `{membership_id}` hoặc `membership_id` trong body, gọi bằng token `c_001` với id của `c_002` → 404 `MEMBERSHIP_NOT_FOUND`; thêm assert đếm route để route mới phải khai báo vào bảng.
- AC: PASS; bỏ guard ở một handler/service → FAIL.

### T7: Cập nhật test cũ
- Test dùng membership giả không tồn tại trong repo sẽ nhận 404 (guard đòi membership có thật trong company): chạy `go test ./internal/companyaccess/...`, sửa fixture seed membership thật. Ví dụ `TestAssignCompanyAdmin_MembershipNotInCompany` đã dùng "m-ghost" hợp lệ vì đích không tồn tại.
- Cập nhật `docs/api-contracts-json.md`: các route theo `membership_id` trả 404 `MEMBERSHIP_NOT_FOUND` khi đối tượng ngoài company.
- AC: chỉ còn 2 fail có sẵn (`TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`, `TestCreateSelfServiceCompany_FeatureFlagOff`).

### T8: Verify (Gate V)
- `go build ./...`; `go test ./... -count=1` so với baseline HEAD (worktree tạm), không có fail mới; `go vet ./...` (chỉ lỗi copylocks có sẵn); `go test -race -count=1 ./internal/companyaccess/... ./internal/platformcms/...`; `docker compose -f docker-compose.dev.yml build api`.
- Grep: mọi hàm trong `internal/companyaccess/app` nhận `MembershipID` có guard hoặc có chú thích ngoại lệ.
- Ghi `04-verify.md`.

### T9: Review và đóng
- Chạy song song `be-security-reviewer`, `admin-role-reviewer`, `api-compat-reviewer` trên diff `internal/companyaccess` (có đổi chữ ký repo và mã lỗi 404 mới), rồi `premerge-system-review` bản ngắn.
- ai-cache: `05-completion.md`; đánh dấu C5 "fixed in branch" trong `risk-review-2026-10-09/10-risk-report.md`.
- Không commit hoặc push khi chưa được yêu cầu.
- Deploy DEV và smoke chỉ khi được yêu cầu (theo `wf-release`): backup binary; lấy id membership thật của `c_002` qua route platform bằng tài khoản operator (read-only); dùng token tenant `c_001`: probe đọc `GET /memberships/{id c_002}/permissions` → 404 (chứng minh binary mới), sau đó các request ghi kỳ vọng bị từ chối → 404; luồng trong company mình vẫn chạy; Playwright chỉ đọc màn `/app/admin/users`. Với SQL MySQL mới (`DeleteTeamRow`, `RemoveTeamMember`, `TeamBelongsToCompany`, các hàm đếm) cần thao tác trên dữ liệu của chính company mình để xác nhận không lỗi cú pháp: dùng team/department tạm do smoke tự tạo và tự xoá trong `c_001`, chỉ khi user đồng ý ghi dữ liệu DEV.
- Follow-up (ngoài PR): sửa authorizer để so company của resource; `CreateMembership` gắn user bất kỳ (BES-02); hợp nhất hai định nghĩa "platform operator" (ROLE-02); FE xử lý `COMPANY_SCOPE_MISMATCH`/404 do tab cũ; 9 user không có membership trên DEV (cần duyệt riêng).

## File chính
- **Mới:** `internal/companyaccess/app/admin_service_membership_scope_test.go`, `internal/companyaccess/transport/http/admin_handler_membership_scope_test.go`.
- **Sửa (service):** `app/admin_service_company_scope.go`, `admin_delegation_scope.go`, `admin_service.go`, `admin_service_departments.go`, `admin_service_titles.go`, `admin_service_primary_role.go`, `admin_service_membership_org.go`, `config_approval.go`, `admin.go` (interface repo).
- **Sửa (repo):** `infra/mysql/admin_repository.go`, `admin_repository_teams.go`, `admin_repository_departments.go`, `admin_repository_titles.go`; `infra/inmemory/admin_repository.go`.
- **Docs:** `tasks/plan.md`, `tasks/todo.md`, `docs/api-contracts-json.md`, `docs/ai-cache/bug-membership-id-company-scope-2026-10-09/*`.
- **Không đổi:** FE, migration, `internal/platformcms`, cờ cấu hình.

## Code dùng lại
- `requireMembershipInCompany` (`admin_service_departments.go:129`), `repo.MembershipBelongsToCompany` (MySQL `admin_repository_departments.go:14`, in-memory `:1489`).
- `departmentInCompany` (`config_delegation.go:238`), `perr.CodeMembershipNotFound`.
- Mẫu test: `admin_service_company_scope_test.go` và `admin_handler_company_scope_test.go` (C4), `fakeAuthService`, `seedInviteScopedSubject`.

## Tương thích và rollback
- FE hiện tại không bị ảnh hưởng; rủi ro duy nhất là tab cũ sau khi đổi company nhận 404.
- Không có migration; đổi chữ ký repo chỉ trong tiến trình. Rollback bằng binary cũ sẽ mở lại khoảng trống.
- Rủi ro kỹ thuật chính: hàm đếm đổi chữ ký và SQL MySQL mới không có test tự động (không có `sqlmock`); giảm rủi ro bằng review kỹ SQL và smoke DEV.

---

# Plan (ROLE-01, CRITICAL): rollback / apply ma trận RBAC chỉ thay đổi role `tenant_custom` của company

## Context
- Nguồn: risk review lần 2 (`docs/ai-cache/risk-review-2026-10-09-2/10-risk-report.md`); phân tích `docs/ai-cache/bug-rbac-rollback-global-roles-2026-10-09/{00-report,01-root-cause-solution,02-data-audit}.md`.
- Vấn đề:
  - `RestoreRBACMatrixFromSnapshot` và `restoreRBACMatrixInTx` reconcile mọi role do `ListRoles` trả về, kể cả role global và role protected, và xoá quyền `cms`/`platform` vì `want` đã lọc còn `current` thì chưa.
  - Dữ liệu DEV: rollback ở c_001 sẽ gỡ `platform.cms.view` của cả 3 operator CMS; một tenant tự đăng ký rollback sẽ đổi quyền role chủ sở hữu dùng chung của 5 company.
- Quyết định đã chốt:
  1. Chỉ `tenant_custom` của company.
  2. Rollback có thay đổi đụng quyền critical thì đi qua approval.
  3. Đã audit DEV.
  4. CRITICAL.
- Quy trình: `wf-bugfix`, test fail trước (Gate R) rồi mới sửa; Gate V; reviewer.
- Phạm vi: chỉ `cobo_iam_services`. Không migration. FE không gọi route rollback.

## Dependency graph
```
T0 artefact
 └─ T1 in-memory: role có company + fixture (chưa đổi logic restore)
     └─ T2 test Gate R (service + hàm thuần): FAIL trên code hiện tại
         └─ T3 hàm thuần ComputeRBACMatrixRestorePlan
             ├─ T4 nối plan vào restore (MySQL + in-memory) + SQL phòng thủ
             ├─ T5 SubmitConfigApproval kiểm role
             └─ T6 rollback có quyền critical → approval (change_type rbac.matrix.rollback)
                 └─ T7 hợp đồng + test cũ
                     └─ T8 verify (Gate V, revert-check)
                         └─ T9 review + ai-cache (+ deploy/smoke khi được yêu cầu)
```

## Quy ước test
- Package `app_test`, repo `cainmem`, `fakeAuthService` persona (mẫu C4/C5).
- Fixture `newRollbackFixture(t)`:
  - Company `c_001`.
  - Persona: `ownerAdmin` có `rbac.manage` (người rollback hoặc submit); `approver` có `system.settings` (membership khác).
  - Catalog: `perm_ent_a` (enterprise, thường), `perm_ent_b` (thường), `perm_crit` = `rbac.manage` (critical), `perm_cms_view` = `platform.cms.view` (module `platform`), `perm_cms_write` = `cms.template.write` (module `cms`).
  - Role:
    - `g_owner`: `system_global`, protected, company rỗng.
    - `cms_op`: `tenant_default`, protected, c_001, giữ `perm_cms_view` + `perm_cms_write`.
    - `admin_dn`: `tenant_default`, protected, c_001.
    - `custom_a`: `tenant_custom`, c_001.
    - `custom_other`: `tenant_custom`, c_002.
- Khẳng định chung: đọc lại `rolePermissions` qua repo (không qua service đã lọc) sau rollback hoặc approval.

## Tasks

### T0: Artefact
- Append plan vào `tasks/plan.md`, việc vào `tasks/todo.md` (không ghi đè C2–C5).
- AC: tài liệu `00`/`01`/`02` có trong `docs/ai-cache/bug-rbac-rollback-global-roles-2026-10-09/`.

### T1: In-memory repo có company cho role (chưa đổi logic restore)
- `infra/inmemory/admin_repository.go`:
  - Thêm `roleCompany map[string]string` và `SeedRoleForCompany(item, companyID)`. `SeedRole` cũ giữ nguyên, tương đương role global.
  - `ListRoles`/`RoleAccessibleByCompany` lọc `company == "" || company == companyID`.
- AC: `go test ./internal/companyaccess/...` không có fail mới (chỉ 2 fail có sẵn).

### T2: Test Gate R (viết trước, phải FAIL trên code hiện tại)
File mới `app/rbac_rollback_scope_test.go`. Mỗi test tạo snapshot bằng một mutation hợp lệ trên `custom_a`, rồi đổi trạng thái DB trực tiếp qua repo cho role ngoài phạm vi.
- `TestRBACRollback_KeepsGlobalRolePermissions`: thêm `perm_ent_b` vào `g_owner` sau snapshot, rollback → `g_owner` vẫn có `perm_ent_b`. **FAIL hiện tại.**
- `TestRBACRollback_KeepsPlatformPermsOnTenantDefault`: rollback → `cms_op` vẫn có `perm_cms_view` + `perm_cms_write`. **FAIL hiện tại.**
- `TestRBACRollback_DoesNotModifyProtectedTenantDefault`: thêm `perm_ent_b` vào `admin_dn` sau snapshot → còn nguyên. **FAIL hiện tại.**
- `TestRBACRollback_RestoresTenantCustom` (giữ hành vi): `custom_a` về đúng snapshot. PASS hiện tại.
- `TestRBACRollback_DoesNotTouchOtherCompanyRoles`: `custom_other` và direct permission của c_002 không đổi. In-memory hiện thu hồi direct permission mọi company. **FAIL hiện tại.**
- `TestRBACApprovalApply_KeepsOutOfScopeRoles`: `RemoveRolePermission` critical trên `custom_a` → approval → `approver` duyệt → `cms_op`, `g_owner`, `admin_dn` không đổi; `custom_a` mất đúng quyền đó. **FAIL hiện tại.**
- `TestSubmitConfigApproval_RBACPermRemove_RejectsProtectedAndGlobal`: `role_id` = `cms_op` → 403 `PROTECTED_ROLE_READ_ONLY`; `g_owner` → 403; `custom_other` → 404. **FAIL hiện tại** (đang xếp hàng được).
- `TestRBACRollback_CriticalChange_RoutesToApproval`: snapshot có `perm_crit` trên `custom_a`, sau đó gỡ (qua approval đã duyệt), rollback → 202 `APPROVAL_ROUTED`, không đổi dữ liệu, pending `rbac.matrix.rollback`. Người rollback tự duyệt → 403 `SELF_APPROVAL_NOT_ALLOWED`. `approver` duyệt → `custom_a` có lại `perm_crit`, role ngoài phạm vi không đổi. **FAIL hiện tại** (đang áp dụng ngay).
- `TestRBACRollback_NonCritical_AppliesDirectly`: chỉ đụng `perm_ent_a` → áp dụng ngay, tạo version `source=rollback`. PASS hiện tại (giữ hành vi).
- `TestRBACRollback_CriticalApproval_StaleAfterMutation`: sau khi xếp hàng, một mutation khác tạo version mới → duyệt trả 409 `STALE_PROPOSAL`.
- Ghi output vào `docs/ai-cache/bug-rbac-rollback-global-roles-2026-10-09/03-repro.md`.
- AC: test biên dịch được (dùng hằng hoặc hàm mới qua stub tối thiểu nếu cần); các test đánh dấu FAIL thì fail đúng lý do; test giữ hành vi thì PASS.

### T3: Hàm thuần `ComputeRBACMatrixRestorePlan` (file mới `app/rbac_restore_plan.go`)
- Input: `companyID`, `roles []RoleListItem`, `current map[roleID][]PermissionListItem`, `snap configversion.RBACMatrixSnapshot`, `catalog map[permissionID]PermissionListItem`.
- Luật: chỉ role `tenant_custom` && `!IsProtected`; chỉ quyền `IsEnterprisePermission`; bỏ qua mục snapshot của role ngoài phạm vi hoặc quyền không có trong catalog.
- Output: `[]RBACRestoreOp{RoleID, PermissionID, PermissionCode, Op}` + `TouchesCritical`.
- **Test bảng** `app/rbac_restore_plan_test.go` (Gate R viết cùng lúc, FAIL vì hàm chưa có):
  - global, `tenant_default` và `is_protected` bị bỏ qua;
  - quyền `cms`/`platform` trong `current` không bao giờ bị gỡ, trong snapshot không bao giờ được thêm;
  - quyền lạ bị bỏ qua;
  - thêm/gỡ đúng cho custom;
  - `TouchesCritical` đúng khi thêm hoặc gỡ quyền critical;
  - snapshot cũ có mục role global bị bỏ qua.
- AC: test bảng PASS.

### T4: Nối plan vào restore + SQL phòng thủ
- **MySQL:**
  - `RestoreRBACMatrixFromSnapshot` và `restoreRBACMatrixInTx` dựng input (`ListRoles`, `ListRolePermissions` của role custom, `ListPermissions`), gọi plan, rồi thực thi op.
  - Câu gỡ và thêm có điều kiện `roles.company_id = ? AND role_type = 'tenant_custom' AND is_protected = 0` (xem `01-root-cause-solution.md` §2).
  - Direct permission giữ nguyên (đã theo company).
- **In-memory:** dùng cùng plan; chỉ thu hồi direct permission của `companyID`.
- AC: T2 (trừ T5/T6) PASS; `go build ./...` xanh.
- **Checkpoint CP-1:** `go test ./internal/companyaccess/...` chỉ còn 2 fail có sẵn và các test T5/T6.

### T5: `SubmitConfigApproval` kiểm role
- `config_approval.go`, nhánh `ChangeTypeRBACPermissionRemove`: `roleForRBACMutation` → `IsRoleProtectedForMutation` → quyền phải `IsEnterprisePermission`, rồi mới `submitRBACRolePermRemoveApproval`.
- AC: `TestSubmitConfigApproval_RBACPermRemove_RejectsProtectedAndGlobal` PASS.

### T6: Rollback có quyền critical → approval
- `configversion/types.go`: thêm `ChangeTypeRBACMatrixRollback = "rbac.matrix.rollback"`.
- `RollbackRBACMatrixVersion`:
  - Sau khi lọc snapshot, tính plan (không ghi). Nếu `TouchesCritical`, gọi `queueConfigApproval` (`AggregateRBACMatrix`, `proposed` = snapshot đã lọc, `BaseLiveVersionNo` = live) và trả `routeApprovalRouted`. Ngược lại giữ luồng cũ.
  - Audit: `appendVersionAudit(..., "admin.version.rbac.rollback_routed", ...)`, hoặc dùng audit approval có sẵn; chọn khi implement và ghi lại.
- Kiểm `checkStaleProposal`/`currentLiveVersionNo` hoạt động với change type mới (cùng aggregate).
- AC: `TestRBACRollback_CriticalChange_RoutesToApproval`, `..._NonCritical_AppliesDirectly`, `..._StaleAfterMutation` PASS.

### T7: Hợp đồng và test cũ
- Chạy `go test ./internal/companyaccess/...`. Sửa fixture test cũ dựa trên role không company hoặc rollback đụng role protected (ví dụ `TestRBACRollback_DoesNotReintroduceCMSPermission`, `config_approval_test.go`), không nới khẳng định.
- `docs/api-contracts-json.md`:
  - rollback có thể trả 202 `APPROVAL_ROUTED`;
  - chỉ áp dụng role tùy chỉnh;
  - `rbac.permission.remove` trên role protected → 403;
  - change type mới.
- `docs/qa-test-matrix.csv`: thêm dòng cho các case T2.
- AC: chỉ còn 2 fail có sẵn.

### T8: Verify (Gate V)
- `go build ./...`; `go test ./... -count=1` so với baseline HEAD (worktree tạm), không có fail mới; `go vet ./...`; `go test -race -count=1 ./internal/companyaccess/...`; `docker compose -f docker-compose.dev.yml build api`.
- **Revert-check:**
  - bỏ điều kiện `tenant_custom` trong plan → các test global/`tenant_default` FAIL;
  - bỏ guard ở T5 → test submit FAIL;
  - bỏ nhánh approval ở T6 → test critical FAIL.
- Grep: mọi chỗ ghi `role_permissions` trong `infra/mysql` đi qua đường có guard hoặc có lý do ghi rõ (Assign/Remove trực tiếp đã có guard ở service).
- Ghi `04-verify.md`.

### T9: Review và đóng
- Song song `be-security-reviewer`, `admin-role-reviewer`, `api-compat-reviewer` trên diff; `premerge-system-review` bản ngắn.
- ai-cache: `05-completion.md`; đánh dấu ROLE-01 "fixed in branch" trong `risk-review-2026-10-09-2/10-risk-report.md`.
- Không commit/push khi chưa được yêu cầu.
- Deploy/smoke **chỉ khi được yêu cầu**. Rollback là thao tác ghi, nên đề xuất:
  1. backup binary;
  2. query read-only trước (đếm quyền của `cms_operator`, `admin_web`, `self_reg_company_owner`);
  3. rollback có quyền critical ở c_001 → kỳ vọng 202 và không đổi dữ liệu;
  4. rollback không critical **chỉ khi user duyệt ghi DEV**;
  5. query lại: role ngoài `tenant_custom` không đổi.

## File chính
- **Mới:** `app/rbac_restore_plan.go`, `app/rbac_restore_plan_test.go`, `app/rbac_rollback_scope_test.go`.
- **Sửa:** `app/config_versioning.go`, `app/config_approval.go`, `configversion/types.go`, `infra/mysql/admin_repository_versioning.go`, `infra/mysql/admin_repository_approval.go`, `infra/inmemory/admin_repository.go`, `infra/inmemory/admin_repository_versioning.go`.
- **Docs:** `tasks/*`, `docs/api-contracts-json.md`, `docs/qa-test-matrix.csv`, `docs/ai-cache/bug-rbac-rollback-global-roles-2026-10-09/*`.
- **Không đổi:** FE (nhãn change type mới là follow-up), migration, interface `AdminRepository`.

## Tương thích và rollback
- Old FE ↔ new BE: FE không gọi route rollback. Màn Approvals hiển thị mã `rbac.matrix.rollback` thô (follow-up nhãn).
- Snapshot cũ có mục role global vẫn đọc được; plan bỏ qua các mục đó.
- Rollback binary sẽ mở lại lỗi. Không có thay đổi schema.
- Lưu ý: approver cần `system.settings`. Company không ai có quyền này sẽ không duyệt được rollback critical (liên quan ROLE-04).

---

# Plan (ROLE-01 follow-up): người duyệt, transaction, test tích hợp, approval lỗi thời, FE

Chi tiết đầy đủ: `docs/ai-cache/bug-rbac-rollback-global-roles-2026-10-09/06-followup-plan.md`.

## Context
- ROLE-01 đã sửa và deploy DEV (`09-role01-post-deploy.md`); còn 6 việc trong `05-completion.md`.
- Phát hiện khi lập plan: 0/20 công ty trên DEV có người giữ `system.settings` → mọi approval cấu hình không ai duyệt được (cùng gốc với ROLE-04).

## Dependency graph
```
C (test tích hợp MySQL) ──> B (transaction: queryer, khoá, recheck critical, snapshot thực, version khi tạo role) ──> D (approval lỗi thời, compare thật)
A (người duyệt + ROLE-04)  [chờ quyết định]  ──> C-T4 smoke DEV nhánh duyệt
E (FE nhãn + guard nút duyệt)  [PR web, độc lập; E-T2 theo A]
```

## Tasks (tóm tắt; AC chi tiết trong 06-followup-plan.md)
- A-T1..T5: test → `authorizeConfigApprovalDecide` nhận `rbac.manage` (người khác người yêu cầu) → ROLE-04 `requireRbacManage` cho company admin/transfer → guard FE → hợp đồng.
- B-T1 `queryer` (đọc qua tx, cũng xử lý PERF-15); B-T2 khoá `FOR UPDATE` + plan trong tx; B-T3 `RBACRestoreOptions{AllowCritical}` + chuyển approval khi mismatch; B-T4 lưu snapshot thực sau apply; B-T5 chụp version khi tạo/nhân bản/vô hiệu role.
- C-T1..T3 test tích hợp `MYSQL_TEST_DSN` cho restore + `ApplyPendingApprovalInTx`; C-T4 smoke DEV nhánh duyệt sau A.
- D-T1 409 `APPROVAL_NOTHING_TO_APPLY`; D-T2 compare trả danh sách thay đổi thật; D-T3 dọn approval pending lỗi thời (cần duyệt ghi); D-T4 hợp đồng.
- E-T1 nhãn `rbac.matrix.rollback`; E-T2 guard nút duyệt; E-T3 hiển thị thay đổi.

## Checkpoints
- CP-1 sau C: test tích hợp chạy xanh trên MySQL local với code hiện tại (ghi lại hành vi hiện tại, kể cả snapshot sau apply = bản đề xuất).
- CP-2 sau B: test tích hợp + in-memory xanh; revert-check guard B-T2/B-T3/B-T4.
- CP-3 sau A (+C-T4): approval duyệt được bởi admin thứ hai trên DEV.
- CP-4 sau D/E: premerge review (be-security, admin-role, api-compat, fe-security cho E).

## Quyết định đã chốt (user, 2026-10-09)
1. Người duyệt: **A1** (người khác người yêu cầu, có `rbac.manage` hoặc `system.settings`).
2. Công ty 1 admin: **giữ nguyên** (mời thêm admin, không tự duyệt).
3. **Gộp ROLE-04** vào WS-A.
4. B-T3 mismatch: **tự chuyển sang approval 202**.
5. DEV là dữ liệu mẫu: được phép ghi/dọn.
