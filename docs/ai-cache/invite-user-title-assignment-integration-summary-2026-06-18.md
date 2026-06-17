# Invite User Title Assignment Integration Summary

- Created: 2026-06-18
- Updated: 2026-06-18
- Skill: integration-cross-repo
- Scope: `cobo_iam_services` + `cobo_web_design`

## Summary

Luồng `POST /api/v1/admin/users/invite` nay hỗ trợ field tùy chọn `title_id` để membership của nhân sự mới được gán chức danh ngay khi tạo. Giải pháp tái sử dụng logic membership-title hiện có thay vì thêm một bước cập nhật thứ hai từ frontend.

## Key Decisions

- Giữ contract cũ tương thích ngược: `title_id` là optional.
- Áp dụng title trong cả hai nhánh:
  - user đã active nhưng chưa là member của công ty
  - user mới được mời và tạo membership mới
- Không sửa schema DB; dùng bảng `membership_titles` hiện có.

## Changed Areas

- `internal/companyaccess/app/admin.go`
- `internal/companyaccess/app/admin_service.go`
- `internal/companyaccess/transport/http/admin_handler.go`
- `internal/companyaccess/infra/inmemory/admin_repository.go`
- `internal/companyaccess/app/admin_service_test.go`

## Verification Notes

- Có test service xác nhận invite với `title_id` sẽ thấy title trong membership list.
- `go test ./internal/companyaccess/...` vẫn bị chặn bởi một test HTTP pre-existing ngoài scope.
- `go build ./...` cần pass và Docker build API nên được thử lại nếu môi trường cho phép.
