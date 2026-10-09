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
