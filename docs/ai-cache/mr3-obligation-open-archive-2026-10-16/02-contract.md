# 02: Khoá contract (GATE A của MR-3)

## Endpoint
| Method | Path | Thay đổi |
|---|---|---|
| GET | `/api/v1/company/deadline-alerts` | Hồ sơ đang mở của template đã lưu trữ hoặc chưa kích hoạt **xuất hiện lại**. Mỗi item thêm `template_archived: bool` |
| GET | `/api/v1/company/deadline-alerts/next` | Loại thêm kỳ `SUSPENDED`/`SKIPPED`. Vẫn yêu cầu template có version đang hoạt động, vì chỉ template như vậy mới sinh hồ sơ (template đã lưu trữ tự bị loại) |
| GET | `/api/v1/company/dashboard/overview` | `immediate_actions[]` thêm `template_archived`; số đếm tính cả hồ sơ của template đã lưu trữ |
| GET | `/api/v1/me/operational-overview` | `my_tasks[]` thêm `template_archived` |
| GET | `/api/v1/platform/cms/dashboard/overview` | Số nghĩa vụ đang mở tính cả template đã lưu trữ (dùng chung predicate) |
| POST | `/api/v1/platform/cms/templates/{type_id}/archive` | Response thêm `suspended_cycles: int` |
| POST | `/api/v1/platform/cms/templates/{type_id}/restore` | Body thêm `backfill_cycle_ids?: string[]` (tối đa 1000). Response thêm `backfilled_cycles`, `skipped_cycles`, `resumed_cycles`. 422 `INVALID_REQUEST`, `details.field=backfill_cycle_ids` nếu có id sai |
| GET (mới) | `/api/v1/platform/cms/templates/{type_id}/archive-impact` | `{type_id, status, open_records: {total, companies: [{company_id, company_name, open_count}]}, cycles: {pending: n, suspended: [{cycle_id, company_id, company_name, cycle_label, due_date}], suspended_total: n, auto_resume: n}}`. `suspended` chỉ gồm kỳ **đã lỡ** (tối đa 1000) |

## Quyền và phạm vi
- Portal: không đổi, dữ liệu vẫn theo `company_id` trong token.
- CMS: archive-impact dùng `requireCMSTemplateArchive`, giống archive/restore. Chỉ áp cho template chung (`company_id IS NULL`); template doanh nghiệp trả 404 như restore.

## Trạng thái kỳ
| Từ | Sự kiện | Tới |
|---|---|---|
| `PENDING`/`RETRY`, chưa có hồ sơ | Lưu trữ | `SUSPENDED` (`last_error_code=TEMPLATE_ARCHIVED`) |
| `CLAIMED` | Worker nhận 409 `TEMPLATE_ARCHIVED` hoặc 404, và template vẫn đang lưu trữ | `SUSPENDED` (template đã khôi phục: `RETRY`/`FAILED` như cũ) |
| `PENDING`/`RETRY` sót lại (worker cũ, reaper) | Khôi phục | Treo trước (`SUSPENDED`), rồi xử lý như các dòng dưới |
| `SUSPENDED` | Khôi phục, có trong `backfill_cycle_ids` | `PENDING` |
| `SUSPENDED` | Khôi phục, kỳ chưa đến ngày mở hoặc đã có hồ sơ từ lần thử trước | `PENDING` (tự chạy lại, không cần chọn) |
| `SUSPENDED` | Khôi phục, kỳ đã lỡ và không có trong danh sách | `SKIPPED` |
| `SKIPPED` | — | Kết thúc; không được claim, không hiện ở "kỳ sắp tới" |

## Idempotency và đồng thời
- Lưu trữ và khôi phục dùng khoá `FOR UPDATE` sẵn có trên `disclosure_types`. Thay đổi kỳ nằm trong cùng transaction.
- Claim yêu cầu `PENDING/RETRY`, nên kỳ đã `SUSPENDED`/`SKIPPED` không bị claim.
- Gọi khôi phục lần hai trả 409 `STATE_CONFLICT` như hiện nay (template không còn ở trạng thái lưu trữ).

## Cờ và migration
- Không có cờ tính năng, không đổi schema. `SUSPENDED` và `SKIPPED` vừa cột `VARCHAR(16)`.
- **Bổ sung sau GATE A:** migration chỉ đổi dữ liệu `0154_periodic_cycles_suspend_archived_templates` (đã có trong `run_dev_migrations.sh`).
  - Nó treo hồi tố các kỳ `PENDING/RETRY` chưa có hồ sơ của template đã lưu trữ trước bản này. Nếu không, khôi phục sẽ làm sống lại chúng (DL-05).
  - Chạy lại an toàn, và đúng ở cả trước lẫn sau khi binary mới lên; `down` không làm gì (forward-only).
  - Đã chạy 2 lần trên MySQL 8 thật: kỳ của template đang hoạt động không bị đụng tới.

## Tương thích khi deploy lệch phiên bản
| Kịch bản | Kết quả |
|---|---|
| FE cũ, BE mới | Chạy được; thiếu nhãn. Khôi phục không có lựa chọn, nên mọi kỳ treo bị bỏ qua (an toàn) |
| FE mới, BE cũ | Không có cờ nên không có nhãn. Archive-impact 404/405 thì hộp thoại ghi "Chưa tải được số liệu". BE cũ bỏ qua `backfill_cycle_ids` và làm sống lại mọi kỳ (hành vi cũ) |
| Worker cũ, API mới | Worker cũ không biết `SUSPENDED`/`SKIPPED`. Claim của nó chỉ nhận `PENDING/RETRY`, nên không claim nhầm. Kỳ claim mà gặp 409 vẫn chuyển `RETRY`; kỳ đó nằm im (template không có version hoạt động) và được khôi phục gom về treo trước khi xử lý (bước 1), nên không bị tạo hồ sơ hàng loạt |
| Rollback BE (sau khi đã lưu trữ bằng bản mới) | Binary cũ không đụng `periodic_cycles` khi khôi phục, nên kỳ `SUSPENDED` sẽ kẹt, và `/next` cũ còn hiện kỳ `SUSPENDED`/`SKIPPED`. **Runbook trước khi rollback** (chỉ đổi dữ liệu): `UPDATE periodic_cycles SET materialization_state='PENDING', materialized_at=NULL, next_attempt_at=NULL, last_error_code=NULL, last_error_message=NULL WHERE record_id IS NULL AND materialization_state='SUSPENDED';`. Câu này trả về hành vi cũ: worker cũ vẫn bỏ qua kỳ của template còn lưu trữ, và khôi phục bằng binary cũ làm sống lại chúng. Kỳ `SKIPPED` giữ nguyên (đó là quyết định của Admin Web lúc khôi phục) |
| Thứ tự deploy | Migration 0154, rồi BE (api + worker), rồi FE |

## Dữ liệu
- **DEV:** 312 hồ sơ QA sẽ hiện lại (4 công ty × 78), kèm nhãn. Đã chốt tại GATE A: không đụng dữ liệu, dọn ở MR-8.
- **Prod:** trước khi deploy, đếm hồ sơ đang mở theo template đã lưu trữ (truy vấn ở `05-verification.md`).

## Ma trận test
| Quy tắc | Test |
|---|---|
| Hồ sơ Draft của template đã lưu trữ có trong list, count và snapshot dashboard, với `template_archived=true` | MySQL integration (deadlinealerts), test SQL contract |
| Template chưa kích hoạt (version 0, `active`): hồ sơ đang mở hiện ra, `template_archived=false` | MySQL integration |
| Hạn của hồ sơ không có `planned_date` lấy từ version `archived_from` | MySQL integration (`GetActiveVersionDeadlineConfig`) |
| `/next` loại `SUSPENDED`/`SKIPPED` và template đã lưu trữ | MySQL integration |
| Lưu trữ chuyển `PENDING/RETRY` sang `SUSPENDED` trong cùng transaction; không đụng `CLAIMED` và kỳ đã có hồ sơ | MySQL integration (disclosure) |
| Worker nhận 409 `TEMPLATE_ARCHIVED` thì chuyển `SUSPENDED` | Unit (periodic materialize) |
| Khôi phục: kỳ được chọn về `PENDING`, kỳ đã lỡ còn lại `SKIPPED`; id sai (kể cả id của template khác) trả 422 và không đổi gì; mặc định bỏ qua tất cả kỳ đã lỡ | MySQL integration, unit (app) |
| Khôi phục: kỳ chưa đến ngày mở và kỳ đã có hồ sơ tự chạy lại; kỳ `PENDING`/`RETRY` sót lại được gom về treo trước | MySQL integration |
| Worker: 404 từ template đã lưu trữ thì treo; 404 khi template đang hoạt động thì vẫn FAILED; treo lỗi (deadlock) thì RETRY; template vừa khôi phục thì RETRY | Unit (periodic materialize), MySQL integration (`MarkPeriodicCycleSuspended`) |
| Archive-impact: số hồ sơ đang mở theo công ty, kỳ đã lỡ, `suspended_total`, `auto_resume`; 403 khi thiếu quyền (kể cả có `cms.template.archive` mà thiếu `platform.cms.view`); 404 với template doanh nghiệp | Unit (app), MySQL integration |
| FE: danh sách bị cắt (`suspended_total` lớn hơn danh sách) thì chặn khôi phục | Vitest |
| "Việc của tôi" và dashboard mang `template_archived` | Unit (mapper/aggregate) |
| FE: chip trên thẻ Cảnh báo, header chi tiết, "Việc của tôi", dashboard; hộp thoại CMS (tác động, lỗi tải, checkbox tạo bù) | Vitest |

## Bổ sung sau review (Phase 6, 16/10)
Các thay đổi dưới đây chỉ bổ sung trường hoặc làm chặt quy tắc. Chưa deploy, nên không có client nào phụ thuộc phiên bản trước.
- **Kỳ chưa đến ngày mở tự chạy lại khi khôi phục** (API-01). "Tạo bù" chỉ áp cho kỳ đã lỡ (ngày mở `COALESCE(open_at, cycle_start, due_date)` không sau hôm nay, giờ HCM). Nếu không, lưu trữ rồi khôi phục ngay sẽ bỏ luôn các kỳ sắp tới, mà seeder không tạo lại được (unique key). Như vậy vẫn đúng Q1-B: không tạo hàng loạt hồ sơ cho kỳ đã lỡ.
- **Kỳ đã có hồ sơ từ lần thử trước tự chạy lại** (DB-06): worker hoàn tất hồ sơ đó (id cố định), không tạo thêm. Bỏ qua kỳ này sẽ để lại hồ sơ Draft mồ côi.
- **Khôi phục gom kỳ sót lại về treo trước** (API-02, DB-01): xử lý kỳ `PENDING`/`RETRY` của template đã lưu trữ do worker cũ, reaper hoặc seeder chạy chồng tạo ra. Archive-impact liệt kê chúng.
- **Worker treo cả khi nhận 404** (DB-02): adapter tra workflow trước `CreateRecord`, nên với template đã lưu trữ lỗi thường là 404 chứ không phải 409. Câu treo kiểm tra template vẫn đang lưu trữ (ROLE-04). Treo lỗi thì chuyển `RETRY`, không dừng tick (DB-03).
- **Giới hạn** (ROLE-01, DB-05, API-05): danh sách kỳ đã lỡ tối đa 1000, bằng số id một lần khôi phục nhận. Thêm `suspended_total`; FE chặn khôi phục khi danh sách bị cắt.
- **Audit khôi phục** ghi thêm `backfill_cycle_ids` và `resumed_cycles` (ROLE-02).

## Ngoài phạm vi (đề xuất MR-3b, sửa theo v1.2, không cần PO quyết)
Ad-hoc ở tab Cảnh báo (AC-11), "Việc của tôi" hiện hồ sơ Draft (AC-13), KPI trạng thái mở không lọc theo khoảng ngày, nhắc hạn dùng `ObligationOpen`.

## GATE A
| Mục | Giá trị |
|---|---|
| Trạng thái | **Đã duyệt** |
| Phạm vi | MR-3 làm gọn như contract này; 4 chỗ lệch v1.2 tách sang MR-3b |
| Dữ liệu QA trên DEV | Để 312 hồ sơ hiện lại kèm nhãn; dọn bằng "đóng nghĩa vụ" ở MR-8 |
| Người duyệt | User, trong hội thoại (trả lời câu hỏi tại GATE A) |
| Thời điểm | 16/10/2026 |
