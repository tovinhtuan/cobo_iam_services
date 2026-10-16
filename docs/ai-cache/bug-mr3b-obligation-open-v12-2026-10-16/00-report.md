# 00: Intake, MR-3b (4 chỗ lệch contract v1.2 §2.2)

Đề xuất trong `mr3-obligation-open-archive-2026-10-16/02-contract.md`, mục "Ngoài phạm vi". Đây là sửa theo quyết định PO đã có, không cần quyết định mới:
- contract v1.2 §2.2 (`business-contract-workflow-deadline-final.md`);
- `bug-worker-alert-fixes-2026-10-10/05-po-ba-decision-recommendations.md`, PO duyệt toàn bộ ngày 11/10: Q-12 (a′), PO-01, Q-18.

## Quy tắc (v1.2 §2.2)
- `ObligationOpen` là nguồn sự thật duy nhất cho tab Cảnh báo, dashboard, "Việc của tôi" và reminder.
  - Periodic/custom/thủ công: `Draft AND submitted_at IS NULL`, cộng cổng `OpenAt` của kỳ.
  - Ad-hoc: proposal `approved` và record chưa `Approved/Published/Completed/Done`, kể cả khi record đã auto-submit lúc duyệt.
- KPI trạng thái mở (#1 `needs_action_now`, #3 `pending_approval`, #5 `blocked_or_exception`) là ảnh chụp hiện tại, không theo range. #2 có cửa sổ 7 ngày riêng. Range chỉ áp cho KPI hoàn thành (#4, #6).

## Triệu chứng (đọc code ngày 16/10, HEAD `3031d8f`)
| # | Tiêu chí | Hiện trạng | Vị trí |
|---|---|---|---|
| 1 | AC-11: ad-hoc đã duyệt hiện ngay ở tab Cảnh báo, với hạn đúng | Không hiện. Finalize gọi `SubmitRecord`, nên record thành `PendingReview` và có `submitted_at`, rồi trượt điều kiện `draft AND submitted_at IS NULL` | `disclosure/app/obligation_open.go` (`ObligationOpenSQL`) |
| 2 | AC-13: "Việc của tôi" hiện task của record periodic (Draft) | Cả hai truy vấn lọc `dr.status <> 'draft'` | `personalops/infra/mysql/repository.go:116` (`ListMineRecords`), `:249` (`ListMineOpenTasks`) |
| 3 | KPI mở là ảnh chụp hiện tại | Mọi bucket (OVERDUE/DUE_SOON/UPCOMING/PENDING_CONFIRM) bị lọc theo range. Preset kết thúc ở hôm nay, nên #3 không bao giờ đếm UPCOMING, và #1, #5 rơi mất mục quá hạn cũ hơn ngày đầu range | `portaldashboard/app/service.go:136` |
| 4 | Reminder dùng `ObligationOpen` | Nhắc hạn nộp (scope DISCLOSURE) được tạo khi `submitted_at IS NULL`: ad-hoc đã duyệt không bao giờ được nhắc; record khác Draft mà chưa submit vẫn bị nhắc. Lúc dispatch không kiểm lại, nên record vừa nộp vẫn nhận nhắc (ví dụ bản bị hoãn ngoài khung 07–21h) | `reminder/infra/mysql/repository.go:344-422`, `ListDispatchCandidates` |

## Môi trường và dữ liệu
- DEV (đọc qua `qa_ro` ở Phase 5): số ad-hoc đã duyệt còn mở, record Draft có task mở, `reminder_configs` của record ad-hoc.
- Vai trò: Portal (ENT/thành viên), worker reminder. Không đụng CMS hay quyền.

## Công việc trước đó
- Kế hoạch PR-3 (AHA-01) và PR-4 (PO-01, DLA-03) ngày 10/10 chưa được triển khai; `git log` chỉ có PR-1a, PR-2a.
- MR-3 (`642e3ce`) đã gom `ObligationOpenSQL` về `disclosure/app`. MR-3b mở rộng định nghĩa này.

## Ngoài phạm vi (ghi lại làm follow-up)
- DUE-06: hạn ad-hoc bỏ qua `final_t0_date` khi chưa có `final_deadline_date` (`adhoc/app/proposal_due.go`).
- DLA-04: KPI hoàn thành đọc theo `CompletedAt` từ `disclosure_records`.
- KPI cá nhân của "Việc của tôi" vẫn theo `PENDING_CONFIRM` cũ.
- DLA-02: enrich dòng Draft.
