# C2: Gate R (tái hiện lỗi), 2026-10-09

## Cách chạy
- Test: `internal/workflowconfig/transport/http/authz_test.go`. Code chạy ở HEAD `0609fdf`, chưa có fix.
- Dùng constructor cũ (4 tham số) qua một bản sửa tạm của helper `newServer`; bản sửa tạm này đã được hoàn tác sau khi chạy.
- Lệnh: `go test ./internal/workflowconfig/transport/http/ -run TestAuthz -count=1`

## Kết luận
- Tenant admin và tenant member (không có `platform.cms.view`) **activate thành công (200)** một version workflow toàn cục. Response có `activated_by` là user của tenant.
- Publish đi qua tầng phân quyền và chỉ dừng ở bước validate của service (422 "workflow has no steps", do fake manifest rỗng). Không có lớp authz nào chặn.
- CMS maker activate được, tức là không có tách maker/checker.
- Khi authorizer nil thì không fail-closed.

## Output
```
--- FAIL: TestAuthz_PublishActivate (0.00s)
    --- FAIL: TestAuthz_PublishActivate/tenant_admin_cannot_publish (0.00s)
        authz_test.go:170: expected 403, got 422: {"error":{"code":"INVALID_REQUEST","message":"workflow has no steps"}}
    --- FAIL: TestAuthz_PublishActivate/tenant_member_cannot_publish (0.00s)
        authz_test.go:170: expected 403, got 422: {"error":{"code":"INVALID_REQUEST","message":"workflow has no steps"}}
    --- FAIL: TestAuthz_PublishActivate/cms_reader_cannot_publish (0.00s)
        authz_test.go:170: expected 403, got 422: {"error":{"code":"INVALID_REQUEST","message":"workflow has no steps"}}
    --- FAIL: TestAuthz_PublishActivate/cms_checker_cannot_publish (0.00s)
        authz_test.go:170: expected 403, got 422: {"error":{"code":"INVALID_REQUEST","message":"workflow has no steps"}}
    --- FAIL: TestAuthz_PublishActivate/tenant_admin_cannot_activate (0.00s)
        authz_test.go:170: expected 403, got 200: {"data":{"type_id":"t1","version_no":1,"state":"active","published_at":"1970-01-01T08:00:00+08:00","activated_by":"u-m-tenant-admin"}}
    --- FAIL: TestAuthz_PublishActivate/tenant_member_cannot_activate (0.00s)
        authz_test.go:170: expected 403, got 200: {"data":{"type_id":"t1","version_no":1,"state":"active","published_at":"1970-01-01T08:00:00+08:00","activated_by":"u-m-tenant-member"}}
    --- FAIL: TestAuthz_PublishActivate/cms_maker_cannot_activate_(checker_action) (0.00s)
        authz_test.go:170: expected 403, got 200: {"data":{"type_id":"t1","version_no":1,"state":"active","published_at":"1970-01-01T08:00:00+08:00","activated_by":"u-m-cms-maker"}}
--- FAIL: TestAuthz_NilAuthorizerFailsClosed (0.00s)
    authz_test.go:184: /api/v1/platform/cms/templates/t1/workflow/publish: expected 503 with nil authorizer, got 422
FAIL
FAIL	github.com/cobo/cobo_iam_services/internal/workflowconfig/transport/http	0.002s
FAIL
```
