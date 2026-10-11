# Bug worker/alert: intake (wf-bugfix Phase 0) - 2026-10-10

- Workflow: `wf-bugfix`, **chế độ lập kế hoạch**. Chưa sửa code; chưa viết test; chưa ghi DB.
  - Phase 0–2 thực hiện đầy đủ.
  - Phase 3–6 viết dưới dạng kế hoạch: `02-implementation-plan.md`, `03-test-plan.md`, `04-rollout-data-repair.md`.
  - Plan chia phase theo quy ước `spec-authoring`, Phase 0 là "chốt quyết định (HARD STOP)".
- Repo: `cobo_iam_services` `improve/ui-theme` HEAD `a8acb51`; `cobo_web_design` `improve/ui-theme` (`453a3494`/`79853d9d`).
- Môi trường quan sát: DEV `/root/cobo_project`.
  - Một container worker, tick 5 giây.
  - `PERIODIC_SEEDING_ENABLED=true`, `WORKFLOW_REMINDERS_ENABLED=true`.
  - SMTP thật (Gmail).
  - MySQL 8.0.46, strict mode.

## Nguồn (đã có trước)
| Báo cáo | Nội dung |
|---|---|
| `../worker-alert-spam-audit-2026-10-10/00-report.md` | Sự cố 4 triệu Draft (`cabbecf`), PER-*, REM-*, OBX-*, DATA-01; có phần đính chính ở đầu file |
| `../risk-review-2026-10-10-3/10-risk-report.md` | Risk review lần 5: 9 HIGH (REM-06, PER-01, REM-01, REM-05, H1/BES-36, PER-06, SEC-27, SEC-26, H19), ma trận rollout |
| `../worker-alert-functional-audit-2026-10-10/00-report.md` | Audit chức năng: 11 HIGH (RMC-01, AHA-01/02/03/07, DLA-02/03/04, DUE-01/02/03) |
| `../risk-review-2026-10-10-2/10-risk-report.md` | H1, H8, H9, BES-30... (bảo mật toàn repo) |

Việc trước đây trong cùng vùng code (`git log` từ 2026-09-15):
- `cabbecf`/`71e2fc3` (periodic materialization + 504)
- `7207a95`
- `af3a814` (fix test 9-47, có lỗi SQL 1052)
- `7cf7711` (fix 39; thêm `resource_id` cho in-app; script fixture `_tmp_fixture_complete_ready.sql`)
- `3359556`

## Triệu chứng (thực tế → kỳ vọng)
| # | Người dùng thấy | Kỳ vọng | Tần suất / phạm vi | Bằng chứng DEV |
|---|---|---|---|---|
| S1 | Chỉ nhận nhắc việc cho bước 1 của workflow định kỳ | Mỗi bước được nhắc khi tới lượt | Mọi workflow nhiều bước | bước 1: 224/225 được seed; bước 2: 0/125; bước 3: 0/10 |
| S2 | (sắp xảy ra ~13–14/10) Reminder bước workflow ngừng hẳn | Scanner luôn xử lý milestone đến hạn | Toàn hệ thống | 136 milestone chết, +~15/ngày, scanner `LIMIT 200` |
| S3 | CBTT bất thường đã duyệt không có trong tab Hạn | Hiện, có hạn 24h, xác nhận DONE được | Mọi ad-hoc đã duyệt | proposal approved → record `PendingReview` + `submitted_at` |
| S4 | Màn hình record ad-hoc hiển thị workflow template, có task thừa cho người tạo | Hiển thị workflow đã duyệt | Mọi ad-hoc đã duyệt | 2 instance: `global_template` (được chọn) + `proposal_snapshot_v3` |
| S5 | Xác nhận bước cuối, record vẫn "Chờ duyệt", không có email | Record được duyệt + có thông báo | Mỗi lần Confirm/Review ở bước cuối | code (không có dữ liệu DEV) |
| S6 | Duyệt nhiều người: hạn chốt của người duyệt trước bị mất | Áp quy tắc ngày chốt đã thống nhất | Proposal có ≥2 reviewer | code |
| S7 | Cột Bước/Phòng ban/Người phụ trách luôn trống; nhân viên phòng ban không thấy cảnh báo của mình | Có dữ liệu; thấy cảnh báo của task mình | Toàn bộ danh sách | 308/308 Draft có task mở, `department_id='general'` |
| S8 | Dashboard: "Cảnh báo đang xử lý" thiếu mục sắp đến hạn, mất mục quá hạn cũ | Đếm đúng theo công thức KPI | Mọi company | mỗi company: 26 UPCOMING + 6 OVERDUE bị bỏ (preset 7 ngày) |
| S9 | "Tỷ lệ đúng hạn" luôn 0%; khi có dữ liệu thì 100% bị tô đỏ | Số đúng, màu đúng | Mọi company | `completed_at` = 0 trên mọi record |
| S10 | Hạn BCTC định kỳ tính theo ngày làm việc (ví dụ Q3: 28/10 thay vì 20/10) | Ngày lịch theo TT96 (chờ BA xác nhận) | Template không có rules | code |
| S11 | Tính ngày làm việc bỏ qua Tết/30-4/1-5/2-9 | Đủ ngày nghỉ chính thức | Mọi phép tính ngày làm việc 2026 | `2026.json` chỉ có 01/01; DB lịch nghỉ rỗng |
| S12 | Upload lịch nghỉ qua CMS lưu lùi 1 ngày | Đúng ngày | Mỗi lần upload | code (chưa từng chạy trên DEV) |
| S13 | 00:00–06:59 sáng giờ VN: quá hạn hiển thị "Sắp đến hạn" | Theo ngày VN | Hằng ngày, 7 giờ | code |
| S14 | Reminder gửi dồn lúc 1 giờ sáng, trễ 3 ngày, "còn -1 ngày" | Không gửi bản trễ vô nghĩa; chỉ gửi trong giờ hành chính | Sau mỗi lần worker dừng | 02/10 18:07–18:11 UTC: 80 bản |
| S15 | (rủi ro) Rollback worker tạo lại hàng triệu Draft | Không thể rollback qua `cabbecf` | Khi có người rollback | 6 binary cũ trên DEV |

Các rủi ro không có triệu chứng hiện tại (REM-01, REM-05, REM-03, PER-03/04, H1, SEC-26/27, H19...) được lấy theo `../risk-review-2026-10-10-3/10-risk-report.md`.

## Phạm vi kế hoạch
- **PR-1 Reminder (gấp nhất, hạn ~2026-10-12):** RMC-01, REM-06, REM-01, REM-05, RMC-02, REM-03, REM-04, RMC-06, RMC-03, RMC-04, REM-10, RMC-09.
- **PR-2 Tính hạn + worker periodic:** DUE-01/02/03, DLA-01/DUE-04, DLA-05/DUE-05, PER-06, DUE-07/08/09/10, PER-01/03/04/05, PERF-46, BES-38.
- **PR-3 Ad-hoc:** AHA-01/07/02/03/04/05/06/09/12/13.
- **PR-4 Danh sách hạn + dashboard:** DLA-02/03/04/06/07/09/11/12.
- **PR-5 Trạng thái company + kênh thông báo:** RMC-05/AHA-11, RMC-07/AHA-08, RMC-08.
- **Phụ lục (theo risk review lần 5):** PR-W3 outbox/SMTP, PR-W4 H1, PR-W5 vận hành/deploy, PR-W6 retention.

## Nhật ký quyết định
| Ngày | Mã | Quyết định | Người chốt | Ảnh hưởng |
|---|---|---|---|---|
| 2026-10-10 | Q-05 | **Hạn trùng ngày nghỉ không tự dời.** Khi tạo template đã quy định hạn tính theo ngày làm việc hay ngày lịch, thì tính đúng theo quy định đó. Hạn chỉ định là ngày cụ thể thì đó là ngày bắt buộc. | User | DUE-10 đóng (không phải bug). FIXED_DATE giữ hành vi hiện tại (mặc định giữ ngày; chỉ dời khi template chọn `MOVE_TO_NEXT_WORKING_DAY`). Q-04 có hướng: resolver theo khai báo của template, fallback ngày lịch chỉ khi không khai báo, kèm validate bắt buộc khai báo |

Dữ liệu DEV khi chốt (qa_ro, 2026-10-10):
- 12 template đang active: 8 periodic + 1 ad-hoc khai báo `rules.deadline_day_type=calendar`, 3 periodic khai báo `working`.
- `deadline_config.deadline_duration_type` đều trống.
- Không có template FIXED_DATE nào đang active.
- Hệ quả: worker tính đúng theo template, nhưng preview CMS và fallback của danh sách hạn đang mặc định WORKING. Bản sửa DUE-02 của PR-2b xử lý đúng chỗ này.

## Tệp trong thư mục này
- `00-report.md`: intake (file này).
- `01-root-cause.md`: nguyên nhân gốc từng bug (Phase 2).
- `02-implementation-plan.md`: Phase 0 chốt quyết định + thiết kế sửa từng mục theo PR (Phase 3) + phạm vi ảnh hưởng (Phase 5) + review (Phase 6).
- `03-test-plan.md`: test tái hiện phải fail trước (GATE R) + test hồi quy + lệnh verify (GATE V).
- `04-rollout-data-repair.md`: thứ tự deploy, sàn rollback, kill switch, SQL dry-run/sửa dữ liệu (bản nháp, chưa chạy).
