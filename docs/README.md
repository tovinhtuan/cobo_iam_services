# Docs Index

## Postman

Các file Postman đã generate:

- `CMS.postman_collection.json`
- `WebPortal.postman_collection.json`
- `environment.dev.postman_environment.json`

Phạm vi của 2 collection:

- Chỉ gồm endpoint đã được cross-check giữa FE `cobo_web_design` và BE `cobo_iam_services`
- Không gồm endpoint `mismatch`
- Không gồm endpoint `unmapped / need confirmation`
- Không gồm các screen hiện vẫn đang dùng mock/local state ở FE

## How To Import In Postman

1. Import file `environment.dev.postman_environment.json`
2. Import file `CMS.postman_collection.json`
3. Import file `WebPortal.postman_collection.json`
4. Chọn environment `environment.dev`
5. Cập nhật `{{base_url}}`
6. Điền `{{access_token}}` hoặc chạy login flow để lấy token
7. Với flow multi-company, điền thêm:
   - `{{pre_company_token}}`
   - `{{company_id}}`
8. Với request detail/action, điền các biến tương ứng như:
   - `{{record_id}}`
   - `{{type_id}}`
   - `{{workflow_instance_id}}`
   - `{{task_id}}`
   - `{{proposal_id}}`

## Notes

- `{{base_url}}` mặc định đang để `http://localhost:8080`
- Không có secret/token thật được commit trong repo
- Một số request dùng example body tối thiểu để import ổn định; khi test thật có thể cần payload đầy đủ hơn theo business case
- Tài liệu phân tích chi tiết nằm ở repo workspace root:
  - `docs/postman-collections.md`
  - `docs/ai-cache/postman-collection-cross-repo-summary.md`
