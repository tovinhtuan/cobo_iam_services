# Platform CMS vs tenant API (B2)

**Bối cảnh:** Web Tuần 1 có route stub `/cms` (ứng dụng nền tảng) tách với shell tenant `/app`. Token hiện tại vẫn là **access token gắn bối cảnh công ty** (`company_id` / `membership_id` trong claims) dùng cho mọi `GET /api/v1/me`, disclosure, v.v.

## Quy ước tối thiểu (khi bổ sung nền tảng thật)

| Kênh | Mục đích | Gợi ý prefix / quy ước |
|------|----------|------------------------|
| Tenant (hiện tại) | Thao tác theo công ty đang chọn | `GET/POST /api/v1/...` với Bearer = access token sau `select-company` / `switch-company` |
| Platform (tương lai) | Thao tác toàn hệ thống (CMS, cấu hình global) | Có thể tách `GET/POST /api/v1/platform/...` **hoặc** host tách; **luôn** từ chối rõ (403 + `error.code` phân biệt) nếu token là tenant-only |

## Ghi chú triển khai

- **W4 (web):** Guard client dùng `platform.cms.view` (và tạm `rbac.manage` trong `routePermissionMatrix.platformCms`) — cần IAM cấp quyền tương ứng trong `effective-access` khi sẵn sàng.
- **Không** coi “một middleware duy nhất” là đủ: từng handler vẫn phải enforce policy; tài liệu `B4` (tenant model) bổ sung chi tiết membership/scope.

*Xem thêm: `api-v1-implemented-contracts.json`, `api-contracts-json.md`.*
