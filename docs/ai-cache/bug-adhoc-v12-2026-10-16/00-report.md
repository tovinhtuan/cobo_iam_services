# 00: Intake, MR-2 (ad-hoc theo v1.2 §3.2)

Nguồn:
- `docs/ai-cache/po-decisions-template-workflow-2026-10-16/02-decisions.md`, MR-2 (GATE A đã duyệt 16/10/2026);
- phát hiện của đợt rà soát nghiệp vụ ngày 16/10 (`01-design.md`, phụ lục A).

Bốn phát hiện này đều đã có quy định trong v1.2 §3.2 (`business-contract-workflow-deadline-final.md`), nên không cần PO quyết thêm.

| ID | Mức | Quy định v1.2 §3.2 | Thực tế (theo code) |
|---|---|---|---|
| H1 (XL-01, WF-10) | High | Ngày chốt (`final_t0_date`, `final_deadline_date`, ghi chú): giá trị được nhập sau cùng thắng; ô để trống không xoá giá trị người trước đã nhập | Khi finalize, ngày chốt chỉ lấy theo phiếu cuối cùng. Người duyệt cuối để trống thì giá trị của người duyệt trước bị mất, và T0 quay về T0 đề xuất |
| H2 (XL-05, WF-04) | High | Khi đã đủ phiếu thì không cho Reject/Cancel (409) | Reject/Cancel chỉ kiểm trạng thái `pending_focal_approval`, kể cả khi mọi reviewer đã duyệt |
| H3 (XL-04, WF-05) | High | Finalize lỗi giữa chừng phải bù trừ | (a) Finalize lỗi sau khi phiếu cuối đã ghi: mọi reviewer đều "đã duyệt" nên FE không còn nút nào, đề xuất kẹt ở `pending_focal_approval`. (b) Thử lại sau khi lần trước đã tạo record nhưng chưa tạo workflow: proposal chuyển `approved` với `workflow_instance_id` rỗng |
| XL-06 | Medium | Submit proposal kiểm tra lại reviewer (active, có quyền) và quyền tạo hồ sơ của người đề xuất | Submit chỉ kiểm danh sách reviewer không rỗng |

## Môi trường và dữ liệu
- BE `cobo_iam_services` (`internal/adhoc`), FE `cobo_web_design` (`AdHocProposalDetailPage.tsx`). Ad-hoc chỉ chạy với MySQL.
- DEV (`qa_ro`, 16/10). Không có dữ liệu bị hỏng:
  - proposal theo trạng thái: 1 draft, 1 `pending_focal_approval`, 3 `approved`;
  - 0 proposal approved bị mất ngày chốt hoặc ghi chú;
  - 0 proposal bị reject/cancel sau khi đủ phiếu;
  - 0 proposal approved thiếu record hoặc workflow;
  - 0 proposal đang chờ mà đã đủ phiếu;
  - 0 proposal đang mở có reviewer inactive.
- Lịch sử: `git log -- internal/adhoc` gần đây là refactor đề xuất và pagination. Chưa từng có bản sửa nào cho 4 lỗi này.
