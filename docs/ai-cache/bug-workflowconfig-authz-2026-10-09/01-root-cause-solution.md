# C2: Root cause và giải pháp

## Root cause (confirmed)
Handler `workflowconfig` chỉ gọi `InspectAccessToken` (`internal/workflowconfig/transport/http/version_handler.go:121-128`, `actor()` trả về `claims.Sub`), không gọi tới authorizer. Service (`VersionService.PublishVersion/ActivateVersion`, `AssigneeRoleCatalogService.Create`) cũng chỉ nhận chuỗi actor, không kiểm tra quyền.

- Layer: API transport.
- Handler không nhận `authapp.Service`: wiring `wfchttp.NewHandler(versionSvc, configSvc, catalogSvc, tokenManager)` (`server.go:595`) và `RegisterAssigneeRoleCatalog(mux, svc, tokenManager)` (`server.go:580`).

## Lựa chọn mô hình quyền

| Phương án | Mô tả | Đánh giá |
|---|---|---|
| A. `requireCMSEditor` | `platform.cms.view` + `rbac.manage\|system.settings`, theo đề xuất trong risk report | Không khớp mô hình template: route PUT workflow toàn cục hiện có (`disclosure/app/cms_service.go:42`) dùng `requireCMSTemplateWrite`. Không phân biệt maker/checker giữa publish và activate. Có thể chặn nhầm CMS operator không có `rbac.manage`. |
| **B. Bậc quyền template CMS (khuyến nghị)** | Tái dùng đúng bậc trong `disclosure/app/cms_template_permissions.go` | Nhất quán với PUT/DELETE global workflow và activate template. Migration 0071 đã cấp `cms.template.write/activate/archive` cho mọi role có `platform.cms.view`, nên CMS operator hiện tại không mất quyền. Giữ được tách maker/checker (DOC-001 Q2). |

### Ma trận quyền (phương án B)
| Route | Yêu cầu | Lý do |
|---|---|---|
| GET assignee-roles | Chỉ cần token hợp lệ (giữ nguyên) | Tenant portal dùng để lấy label. Dữ liệu là metadata toàn cục, không theo tenant. |
| POST assignee-roles | `platform.cms.view` + (`cms.template.write` \| `disclosure_type.manage`) | Tạo metadata toàn cục, tương đương sửa template |
| GET configuration / readiness / lifecycle / versions / versions/{n}, POST validate | `platform.cms.view` + một trong {`cms.template.read`, `.write`, `.activate`, `.archive`, `.config.write`, `disclosure_type.manage`, `rbac.manage`} | Giống `requireCMSTemplateRead`. Tenant không gọi các route này. |
| POST publish | `platform.cms.view` + (`cms.template.write` \| `disclosure_type.manage`) | Maker: giống `CmsUpsertGlobalWorkflow` |
| POST versions/{n}/activate | `platform.cms.view` + (`cms.template.activate` \| `disclosure_type.publish`) | Checker: giống `requireCMSTemplateActivate`. Maker (`disclosure_type.manage`) không đủ. |

Thiếu `platform.cms.view` hoặc thiếu quyền → **403 `PERMISSION_DENIED`**, kèm `details.required_permissions` giống `newCMSPermissionDenied`. Không có token hoặc token sai → 401 như hiện tại.

## Thiết kế thay đổi (nhỏ nhất, chỉ trong `cobo_iam_services`)

1. **`internal/workflowconfig/transport/http/authz.go` (file mới)**
   - Định nghĩa `type subject struct{ UserID, MembershipID, CompanyID string }`.
   - `subject(r)`: đọc claims đầy đủ từ token. Thay `actor()`, vẫn trả `UserID` cho tham số actor của service.
   - Interface hẹp `accessResolver{ GetEffectiveAccess(ctx, membershipID, companyID) (…, error) }`, khớp với `authapp.Service`, giống `workflowdoctemplate/transport/http/handler.go:22,63-77`.
   - Các helper `requireTemplateRead / requireTemplateWrite / requireTemplateActivate`, mỗi helper kiểm tra `platform.cms.view` trước rồi tới bộ quyền tương ứng.
   - **Fail-closed:** authorizer nil → 503 `authorizer unavailable`, không cho đi tiếp.
   - Các hằng permission khai báo cục bộ, không import `disclosure/app`, để tránh phụ thuộc vòng.
2. **`version_handler.go` / `catalog_handler.go`**
   - Thêm field `authorizer` vào `Handler`. `NewHandler(..., inspector, authorizer)` và `RegisterAssigneeRoleCatalog(mux, catalog, inspector, authorizer)`.
   - Mỗi handler gọi helper đúng bậc trước khi gọi service.
   - `assigneeRoles`: GET chỉ cần `subject`; POST cần thêm `requireTemplateWrite`.
3. **`internal/httpserver/server.go:580,595`**: truyền `authSvc`, cùng instance đang dùng cho `wdthttp.NewHandler`, dòng 363.
4. Sửa lời gọi trong `version_handler_register_test.go`: thêm `nil`. Test này chỉ khớp pattern nên không bị ảnh hưởng.
5. **Không đổi:** service layer, schema, response shape của các trường hợp thành công, cờ feature.

Có thể làm thêm (defense in depth): đưa check vào `VersionService` qua tham số `Subject`. **Không làm trong PR này**, để diff nhỏ. Handler là điểm vào duy nhất: grep cho thấy `PublishVersion`, `ActivateVersion` và `catalog.Create` chỉ được gọi từ `version_handler.go:155,174` và `catalog_handler.go:86`.

## Kế hoạch tái hiện (Gate R) — làm trước khi sửa code
Viết `internal/workflowconfig/transport/http/authz_test.go` với fake inspector (map token → claims) và fake authorizer (map membership → permissions):

| Case | Kỳ vọng sau fix | Hiện tại |
|---|---|---|
| Tenant admin (`rbac.manage`, không có `platform.cms.view`): POST publish | 403 | 200/422: **fail** |
| Tenant admin: POST activate | 403 | **fail** |
| Tenant admin: POST assignee-roles | 403 | **fail** |
| Tenant admin: GET configuration / versions / readiness / lifecycle, POST validate | 403 | **fail** |
| Tenant member: GET assignee-roles | 200 | pass (giữ nguyên) |
| CMS chỉ có read: GET versions → 200; POST publish → 403 | | |
| CMS maker (`cms.template.write`): publish → tới service; activate → 403 | | |
| CMS checker (`cms.template.activate`): activate → tới service | | |
| Không có token: mọi route → 401 | | |
| Authorizer nil: route có gate → 503 | | |

Thêm một **test chống hồi quy cho route mới**: duyệt mọi pattern mà `Register` và `RegisterAssigneeRoleCatalog` đăng ký, gửi request bằng token tenant admin. Mọi route trừ `GET assignee-roles` phải trả 403. Nhờ vậy, route mới thêm mà quên gate sẽ làm test fail.

Service fake: dùng `VersionService` với repo in-memory nếu có. Nếu không, chỉ cần assert status khác 403 cho case được phép (không cần service thành công).

## Verify (Gate V)
- `go test ./internal/workflowconfig/... ./internal/httpserver/...`; `go vet ./...` (lỗi copylocks có sẵn trong test `workflowfulfillment` không liên quan).
- `go test ./...` toàn bộ: so với baseline 8 test fail đã biết, không được phát sinh fail mới.
- `docker compose -f docker-compose.dev.yml build api`.
- Revert fix → test Gate R phải fail lại.
- Smoke local (sau `make dc-up`, dùng seed local, **không chạy trên server dev**):
  - `admin.dn` (tenant) gọi POST publish → 403.
  - Platform CMS admin gọi các route trên → hoạt động như cũ.
  - Portal tenant: màn DisclosureTypeDetail vẫn hiển thị label role.

## Blast radius
- **Cùng pattern ở chỗ khác:**
  - `adhoc` `/platform/cms/admin/ops/adhoc-migrate-legacy-approvals` (C3, ticket riêng).
  - Các route `/platform/cms` của `platformcms`, `disclosure` (gồm `workflow/template-departments`) và `workflowdoctemplate` đã có gate (đã xác nhận trong risk review).
  - Lúc implement, chạy lại lệnh grep `HandleFunc(".* /api/v1/platform/cms` và đối chiếu từng route.
- **Cross-repo:**
  - Không đổi contract của trường hợp thành công.
  - FE CMS: thêm khả năng nhận 403 ở publish/activate cho operator thiếu quyền. Kiểm tra `workflowConfigApi.ts` có hiển thị lỗi 403 hợp lý không (`workflowConfigurationLoadError.ts` đã phân loại lỗi load).
  - FE tenant: GET catalog giữ nguyên, nên không ảnh hưởng.
- **Dữ liệu có thể đã bị sửa trái phép** (lúc implement cần chạy query read-only để kiểm, chưa chạy):
  - `global_workflow_versions` / bảng active version: tìm bản ghi có `published_by` / `activated_by` mà user đó không có `platform.cms.view`.
  - Bảng assignee role catalog: tìm role tạo bởi user không phải CMS.
  - Nếu có bản ghi bất thường: lập kế hoạch rollback sang version trước bằng chính API activate (dùng CMS admin). Không tự chạy khi chưa được duyệt.

## Mitigation tạm thời trên server (trước khi deploy fix)
- Đặt `WORKFLOW_VERSIONING_ENABLED=false`: chặn các route publish/activate/versions.
- **Không chặn được POST assignee-roles** (đăng ký độc lập với flag). Nếu cần chặn ngay, thêm rule nginx từ chối `POST /api/v1/platform/cms/workflow/assignee-roles`. Rule này chỉ có tác dụng nếu 8080 không còn publish thẳng (xem H23).

## Review sau khi implement
Fix chạm authz, nên chạy `be-security-reviewer` + `admin-role-reviewer` trên diff (theo `wf-risk-review` path map), sau đó `premerge-system-review`.

## Ước lượng
Khoảng 150–250 dòng: authz helper + wiring + test. Một PR: `sec/workflowconfig-authz`.
