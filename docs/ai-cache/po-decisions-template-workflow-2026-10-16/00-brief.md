# 00: Brief, bộ quyết định PO cho các mâu thuẫn template và workflow (16/10/2026)

- **Nguồn:** rà soát nghiệp vụ CRUD template và workflow ngày 16/10/2026 (4 reviewer song song, chỉ đọc): 1 Critical, 13 High. Các phát hiện liên quan liệt kê ở `01-design.md`, Phụ lục A.
- **User yêu cầu:** "sử dụng workflow skill phù hợp để đề xuất các khuyến nghị cho PO chốt các mâu thuẫn trên".
- **Workflow:** `wf-feature`, chỉ Phase 0–2, dừng ở **GATE A** (`02-decisions.md`). Chưa viết code.
- **Skill dùng:**
  - `cobo-domain-cbtt`: căn cứ pháp lý;
  - `system-design-feature`: khung thiết kế từng phương án;
  - `cobo-admin-role-guard`: ranh giới vai trò, áp cho Q5.
- **Size:** normal. Thay đổi chạm 2 repo, có trạng thái mới và migration. Phase 3 trở đi chỉ bắt đầu sau khi PO duyệt, mỗi MR khoá contract chi tiết riêng.

## Mục tiêu
Với mỗi mâu thuẫn, PO chốt **một** quy tắc nghiệp vụ, đủ rõ để làm contract cho từng MR.

## Vai trò liên quan
| Vai trò | Quyết định liên quan |
|---|---|
| Admin Web (Platform, `/cms`) | Q1 (lưu trữ template hệ thống), Q2 (sửa và kích hoạt template), Q4 (cấu hình bước) |
| Admin doanh nghiệp (Portal, `/app/admin`) | Q3, Q6 (template của doanh nghiệp), Q5 (override workflow) |
| Người dùng Portal | Nhìn thấy hạn, cảnh báo, nhắc hạn (Q1, Q2, Q4) |

Ranh giới vai trò: Q1, Q2 chỉ dành cho Admin Web. Q3, Q5, Q6 chỉ trong phạm vi công ty của token. Không phương án nào cho admin doanh nghiệp đụng tới template hệ thống.

## Các mâu thuẫn cần chốt
| # | Mâu thuẫn | Phát hiện |
|---|---|---|
| Q1 | Hồ sơ đang mở của template bị lưu trữ: ẩn hay hiện | H9 (TPL-01, DL-04), DL-05 |
| Q2 | Sửa lịch hoặc hạn của template: xử lý kỳ đã tạo trước và hồ sơ đang mở ra sao | H10, H11 (DL-01, DL-02, TPL-04), DL-06, DL-08, DL-09 |
| Q3 | Loại template (category) của doanh nghiệp, và có tự sinh kỳ hay không | H12 (TPL-02) |
| Q4 | Ý nghĩa của `due_rule` so với `processing_days`; 4b: quy ước đếm ngày | H8 (XL-02, XL-16, WF-09) |
| Q5 | Maker-checker cho override workflow của doanh nghiệp | H6 (WF-03, XL-13, XL-15), XL-14, WF-08 |
| Q6 | Template doanh nghiệp bị từ chối hoặc đã lưu trữ: vòng đời, tạo hồ sơ, quota | TPL-06/XL-17, TPL-07 |

## Tiêu chí hoàn thành Phase 0–2
- Mỗi câu hỏi có: hiện trạng kèm bằng chứng, phương án, đánh đổi, khuyến nghị, hệ quả cho dữ liệu và rollout, test chấp nhận.
- Tách riêng các mục **đã có quy định** (không cần PO) và các **quyết định kỹ thuật** (nhóm dev chốt), để không bàn lại.
- GATE A có chỗ ghi người duyệt và thời điểm.

## Căn cứ đã đọc
| Tài liệu | Nội dung dùng |
|---|---|
| BE `docs/ai-cache/business-contract-workflow-deadline-final.md` | §2.2 (Amendment v1.2: `ObligationOpen`, AC-14, công ty suspended), §3 (ba loại template) |
| FE `docs/doc-002-po-decisions-2026-05-24.md` | OQ-006: soft archive |
| FE `docs/cms-portal-template-workflow-requirements.md` | D-03, D-04; điều kiện publish (dòng 421) |
| FE `docs/ai-cache/cms-portal-template-workflow-requirements-summary.md` | OQ-005 (mặc định khi lưu trữ) |
| FE `docs/ai-cache/deadline-alert-active-template-filter-2026-08-18/` | Contract lọc tab Cảnh báo theo template active |
| FE `docs/ai-cache/recurring-disclosure-business-contract-extraction-2026-08-21.md` | Decision 4, 5, 7, 8 (đều `WAIT_FOR_PO_CONFIRMATION`) |
| FE `docs/ai-cache/custom-workflow-plan-and-solutions-2026-05-17.md` §8 | Quyết định treo về maker-checker |
| FE `docs/ai-cache/cms-template-flow-business-guide-summary.md` | `custom` là phạm vi sở hữu |
| `cobo-domain-cbtt` (`references/tt96-reference.md`) | TT68/2024, dự thảo thay TT96, BLDS Đ147 |
| Code BE/FE và DEV (`qa_ro`, chỉ đọc) | Chi tiết theo từng mục trong `01-design.md` |
