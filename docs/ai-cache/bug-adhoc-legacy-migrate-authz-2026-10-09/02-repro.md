# C3: Gate R (tái hiện lỗi), 2026-10-09

## Thiết lập
- Test `TestLegacyMigrationRoute_NotExposed` (`internal/adhoc/transport/http/tenant_scope_test.go`) chạy trên HEAD `0609fdf`, chưa có fix.
- Một bản sửa **tạm** của `fakeService` (đã hoàn tác sau khi chạy):
  - `ListPendingLegacyApprovals` trả về 1 proposal của `company-B`.
  - `FinalizeLegacyApproval` ghi lại company đã bị tác động.
- Token của test thuộc `company-001` (`fakeInspector`).
- Lệnh: `go test ./internal/adhoc/transport/http/ -run TestLegacyMigrationRoute_NotExposed -count=1`

## Kết luận
- Route tồn tại và trả **200 `{"processed":1}`**. Handler dùng token `company-001` để gọi finalize cho `company-B/proposal-B`.
- Handler không có chốt chặn tenant nào. Chốt duy nhất là `authorize(action="rbac.manage")` trong service. Chốt này chỉ đánh giá trong company của người gọi, và trên DB thực fallback về `system.settings` (xem `01-root-cause-solution.md` mục 1).
- Service thật còn mạo danh process controller của company đích (`service.go:1031`).

## Output
```
GATE-R: caller=company-001 finalized=company-B/proposal-B
--- FAIL: TestLegacyMigrationRoute_NotExposed (0.00s)
    tenant_scope_test.go:24: expected 404 (route removed), got 200: {"processed":1,"stopped_at_proposal_id":null}
FAIL
FAIL	github.com/cobo/cobo_iam_services/internal/adhoc/transport/http	0.003s
FAIL
```
