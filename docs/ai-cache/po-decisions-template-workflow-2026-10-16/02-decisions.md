# 02: Khoá quyết định (GATE A), chờ PO duyệt

Phân tích và bằng chứng: `01-design.md`. Ở đây chỉ chốt **quy tắc nghiệp vụ**. Chi tiết request/response của từng MR sẽ khoá lại ở Phase 2 của MR đó.

## Bảng quyết định
| # | Câu hỏi | Khuyến nghị | Phương án khác | PO chọn |
|---|---|---|---|---|
| Q1 | Hồ sơ đang mở của template bị lưu trữ | **B**: hiện theo `ObligationOpen` ở mọi màn, gắn nhãn "Mẫu đã ngừng áp dụng", vẫn nhắc; không sinh kỳ hay hồ sơ mới (đúng OQ-006 đã chốt). Bước sau: cho đóng nghĩa vụ kèm lý do | A giữ hiện trạng · C ẩn mọi nơi và dừng nhắc | ☐ |
| Q2 | Sửa lịch hoặc hạn template | **D**: thay thế các kỳ chưa sinh hồ sơ khi Activate, có preview; Admin Web chọn "Áp dụng từ kỳ"; tuỳ chọn cập nhật hồ sơ đang mở (mặc định tắt), có audit | A giữ sticky · B chỉ thay kỳ · C tự cập nhật mọi hồ sơ mở | ☐ |
| Q3 | Loại template của doanh nghiệp | **A**: "Định kỳ" phải có lịch cấu trúc và tự sinh kỳ khi đã published. Bỏ `custom` khỏi danh sách loại. Tạm ẩn "Định kỳ" cho tới khi xong A | B chỉ "Bất thường" · C "Định kỳ" là nhãn | ☐ |
| Q4 | `due_rule` so với `processing_days` | **A**: `processing_days` là nguồn duy nhất; màn hình hiển thị số ngày và ngày hạn do engine tính; từ chối `H+N` | B `due_rule` cộng dồn là nguồn · C giữ cả hai | ☐ |
| Q4b | Quy ước đếm ngày của hạn hồ sơ | **(ii)** "không tính ngày T" (BLDS Đ147), chỉ đổi sau khi pháp chế xác nhận và rà mốc T của từng template; trong lúc chờ giữ nguyên | (i) giữ "tính cả ngày T" | ☐ |
| Q5 | Maker-checker cho override | **B** theo quyền: người có quyền duyệt kích hoạt ngay, có audit "tự duyệt"; người chỉ có quyền ghi thì gửi duyệt; mọi đường dùng chung quy tắc. Sau này thêm C (tuỳ chọn bắt buộc 2 người) | A luôn 2 người · C ngay | ☐ |
| Q6 | Template doanh nghiệp bị từ chối hoặc lưu trữ | Sửa bản `rejected` thì về `draft` và gửi duyệt lại được; lưu trữ được từ mọi trạng thái; không xoá cứng; đã lưu trữ thì không tạo hồ sơ hay sinh kỳ mới; quota theo gói công ty | Giữ hiện trạng (ngõ cụt, tạo được hồ sơ từ template đã lưu trữ) | ☐ |

## Hệ quả contract nếu duyệt theo khuyến nghị
| # | Quy tắc / API | Mã lỗi, trạng thái mới | Dữ liệu |
|---|---|---|---|
| Q1 | Predicate `ObligationOpen` dùng chung cho tab Cảnh báo, Việc của tôi, dashboard, reminder. Response có thêm `template_archived`. Khôi phục template có lựa chọn `skip`/`backfill` cho kỳ đang treo | Kỳ: `SUSPENDED_BY_ARCHIVE`. Bước sau: hồ sơ `Cancelled` + `close_reason` | Không có migration cho bước đầu. DEV: dọn 312 hồ sơ QA trước khi bật (kế hoạch sửa dữ liệu riêng) |
| Q2 | Preview Activate trả danh sách kỳ bị thay thế. Activate thay thế kỳ chưa claim. Bước 2 có thêm `effective_from_cycle_label`, `apply_to_open_records` | Kỳ: `SUPERSEDED`. Audit `template.activate.cycles_superseded` | Migration: trạng thái kỳ, unique key hoặc namespace cho nhãn. Cờ `PERIODIC_SUPERSEDE_ON_ACTIVATE` |
| Q3 | Tạo hoặc sửa template doanh nghiệp `periodic` bắt buộc `deadline_config` có cấu trúc; Portal và CMS dùng chung một validator | 422 khi thiếu lịch; 422 với `custom` (sau thời gian tương thích) | DEV: không có. Prod: cần đếm |
| Q4 | `due_rule` không nhận ghi; vẫn trả thêm 1 phiên bản để tương thích. `processing_days ≥ 1` | 422 cho `H+N`; import cảnh báo `DUE_RULE_IGNORED` | Không cần chuyển đổi. Sửa file mẫu import (gỡ luôn 3 test đang đỏ của TPL-09) |
| Q4b | Không đổi gì cho tới khi pháp chế xác nhận | — | Khi đổi: preview hạn trước và sau cho từng template |
| Q5 | Kích hoạt và rebase-apply cần `template.workflow.override.approve`. Thêm endpoint gửi duyệt và danh sách chờ duyệt | 403 khi chỉ có quyền ghi; 422 cho `merge_manual` không hợp lệ; audit `self_approved` | Không cần chuyển đổi |
| Q6 | Bảng chuyển trạng thái mới cho template doanh nghiệp. Quota theo `company_subscriptions`, đếm và tạo trong cùng transaction | 409 `TEMPLATE_ARCHIVED` cho template doanh nghiệp đã lưu trữ; 503 khi lỗi đếm quota | DEV: không có. Prod: cần đếm |

## Tương thích khi deploy lệch phiên bản
- **Q1:** chỉ đổi query ở BE. FE cũ vẫn chạy, chỉ thiếu nhãn.
- **Q3:** FE phải đi trước (ẩn hoặc chuyển form). Nếu không, FE cũ gửi "Định kỳ" không có lịch sẽ nhận 422.
- **Q4:** BE vẫn trả `due_rule` một phiên bản. FE mới ngừng đọc trường này trước khi BE bỏ hẳn.
- **Q5:** FE (nút theo quyền) đi trước hoặc cùng lúc. FE cũ hiện "Kích hoạt" cho người chỉ có quyền ghi; hiện họ đã nhận 403 (chuyển sang trang Forbidden), nên không có gì xấu đi.

## Kế hoạch MR sau khi duyệt
| MR | Nội dung | Phụ thuộc PO | Thứ tự |
|---|---|---|---|
| MR-1 | WF-01 (workflow nhiều bước hoàn tất từ bước 1, tự duyệt), WF-02 (cổng tài liệu) | Không | 1, Critical |
| MR-2 | Ad-hoc theo v1.2: H1–H3, XL-06 | Không | 2 |
| MR-3 | Q1 bước đầu: `ObligationOpen` dùng chung + treo kỳ khi lưu trữ + dọn dữ liệu QA trên DEV | Q1 | 3 |
| MR-4 | Q5: thống nhất quyền override, validate `merge_manual`, "Gửi duyệt" | Q5 | 4 |
| MR-5 | Q2 bước 1 (thay thế kỳ, preview, nhãn kỳ, kiểm lại khi tạo hồ sơ) + H13 công ty suspended | Q2 | 5 |
| MR-6 | Q4: `processing_days` là nguồn, FE hiển thị, sửa import | Q4 | 6 |
| MR-7 | Q3 + Q6: lịch cho template doanh nghiệp, vòng đời, quota | Q3, Q6 | 7 |
| MR-8 | Q2 bước 2, Q1 "đóng nghĩa vụ", Q4b | Q1, Q2, Q4b + pháp chế | 8 |

MR-1 và MR-2 làm được ngay vì không cần PO quyết.

## Test chấp nhận (tổng hợp)
Chi tiết theo từng câu hỏi ở `01-design.md`. Mỗi MR phải có ít nhất:
- MySQL và in-memory cho mọi quy tắc mới;
- AC-14: preview CMS, worker và tab Cảnh báo ra cùng ngày hạn (Q2, Q4b);
- phân quyền: 403 đúng vai trò (Q5); chặn cross-tenant cho template doanh nghiệp (Q3, Q6);
- dữ liệu: không sinh trùng nghĩa vụ, không mất kỳ (Q2); không có hồ sơ mở nào bị ẩn (Q1).

## GATE A
| Mục | Giá trị |
|---|---|
| Trạng thái | **Đã duyệt theo khuyến nghị** (Q1 B, Q2 D, Q3 A, Q4 A, Q4b (ii), Q5 B, Q6 theo bảng) |
| Người duyệt | User, trong hội thoại: "duyệt theo khuyến nghị, làm MR-1" |
| Thời điểm | 16/10/2026 |
| Ghi chú | Q4b chỉ được đổi sau khi pháp chế xác nhận và đã rà mốc T của từng mẫu. Mỗi MR khoá contract chi tiết ở Phase 2 của MR đó. MR-1 bắt đầu ngay theo `wf-bugfix` |
