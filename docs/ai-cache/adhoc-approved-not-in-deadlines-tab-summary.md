# Ad-hoc approved proposal không hiện ở tab "Cảnh báo thời hạn"

## Câu hỏi

Sau khi tạo + phê duyệt cảnh báo bất thường (ad-hoc proposal), vì sao không thấy trong `/app/deadlines` → tab **Cảnh báo thời hạn**?

## Nguyên nhân hiện tại (2026-05-25)

### 1. Hai luồng tách biệt — không có bridge

| Luồng | Màn FE | Nguồn dữ liệu | Sau approve |
|-------|--------|----------------|-------------|
| Ad-hoc proposal | `/app/ad-hoc-proposals` | `GET /api/v1/company/ad-hoc-proposals` | `status=approved`, `record_id`, `workflow_instance_id` |
| Cảnh báo thời hạn | `/app/deadlines` (tab Deadlines) | **`mockDeadlines`** (mockData.ts) | **Không đọc BE** |

BE `AdminApprove` tạo disclosure record + workflow instance; **không** ghi vào bảng/API "deadline alerts" cho tab deadlines (endpoint list deadline alerts **chưa có** trên BE).

### 2. FE tab "Cảnh báo thời hạn" chưa tích hợp API

`DeadlineList.tsx` filter `mockDeadlines` — không gọi ad-hoc API, không gọi disclosures/deadline API.

Tab **Lịch sử CBTT** cùng màn cũng dùng `mockDisclosures` — record thật sau approve cũng không hiện ở đây.

### 3. Nơi user *nên* thấy kết quả hôm nay

- Danh sách đề xuất: `/app/ad-hoc-proposals` (filter `approved`)
- Chi tiết đề xuất: `/app/ad-hoc-proposals/:proposalId` (có `record_id` → mở hồ sơ CBTT nếu có route)
- Không phải tab Cảnh báo thời hạn cho đến khi có deadline-alerts API + wire FE

## Không phải bug đơn lẻ "approve fail"

Nếu API approve trả OK và proposal có `record_id`, luồng BE ad-hoc đã chạy đúng phạm vi hiện tại; gap là **product/implementation chưa nối** ad-hoc output → màn deadline list.

## Hướng xử lý (khi implement)

1. BE: contract `GET /api/v1/.../deadline-alerts` (hoặc derive từ disclosures + workflow + due dates) gồm cả nguồn ad-hoc đã approved.
2. FE: thay `mockDeadlines` bằng API; map `record_id` / `workflow_instance_id`.
3. Tùy chọn UX: link từ proposal approved → deadline card hoặc disclosure detail.

## Tham chiếu code

- FE: `cobo_web_design/src/pages/portal/DeadlineList.tsx` (mockDeadlines)
- FE ad-hoc: `AdHocProposalListPage.tsx`, `adHocAlertsApi.ts`
- BE approve: `cobo_iam_services/internal/adhoc/app/service.go` → `AdminApprove`
- Audit doc: `adhoc-alert-crud-current-state-business-audit-summary.md` (scope không gồm tab deadlines)
