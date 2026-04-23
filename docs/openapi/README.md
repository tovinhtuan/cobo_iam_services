# OpenAPI snapshot (B1)

- **File:** `v1-iam-snapshot.yaml` — OpenAPI 3.0.3, sinh từ cùng nguồn với `../api-v1-implemented-contracts.json` bằng `../scripts/build-openapi-snapshot.mjs`.
- **Postman:** Import → **Upload** chọn `v1-iam-snapshot.yaml` (OpenAPI 3.0) → tạo collection.
- Khi cập nhật JSON contract, chạy lại:

```bash
node docs/scripts/build-openapi-snapshot.mjs
```

(từ thư mục `docs` của repo `cobo_iam_services`).

Phần request/response body chi tiết vẫn tham chiếu `../api-contracts-json.md` (một nguồn mô tả bổ sung cho snapshot).
