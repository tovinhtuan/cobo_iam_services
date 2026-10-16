# 01: Phương án và khuyến nghị cho 6 quyết định

Nhãn bằng chứng:
- **[Đã xác minh]**: tự đọc code, tài liệu hoặc truy vấn DEV (`qa_ro`, chỉ đọc).
- **[Theo review]**: reviewer ngày 16/10 xác nhận trên code, chưa đọc lại lần hai.
- **[Giả định]**: suy luận, cần xác nhận.

Đường dẫn BE tính từ `cobo_iam_services/`, FE từ `cobo_web_design/`.

---

## Q1. Hồ sơ đang mở của template bị lưu trữ

### Hiện trạng
- [Đã xác minh] **PO đã chốt từ 24/05**, OQ-006 trong `docs/doc-002-po-decisions-2026-05-24.md:35`: "Soft archive — hồ sơ đang chạy vẫn hoàn thành được; lưu trữ chỉ chặn tạo hồ sơ mới". Requirements OQ-005 cũng mặc định "cảnh báo nhưng vẫn cho lưu trữ, hồ sơ hiện có giữ nguyên".
- [Đã xác minh] v1.2 §2.2: `ObligationOpen` là nguồn sự thật duy nhất cho tab Cảnh báo, dashboard, Việc của tôi và nhắc hạn.
- [Đã xác minh] Contract ngày 18/08 (`deadline-alert-active-template-filter-2026-08-18/`):
  - ra đời để **ẩn hồ sơ QA trên DEV**, sau khi phương án xoá vật lý bị từ chối;
  - chỉ ghi "hồ sơ cũ + template active → hiện; hồ sơ mới + template đã lưu trữ → ẩn". Không nói gì về hồ sơ cũ có template bị lưu trữ **sau đó**.
- [Đã xác minh] Code:
  - tab Cảnh báo JOIN `dt.active_version_no > 0` (`internal/deadlinealerts/app/active_template.go:13-17`);
  - lưu trữ đặt `active_version_no = 0` (`internal/disclosure/infra/mysql/cms_repository.go:428-429`);
  - comment của hàm lưu trữ lại hứa "In-flight disclosure records are unaffected" (`internal/disclosure/app/cms_archive_restore.go:11-12`).
- [Theo review] Việc của tôi (`personalops`), dashboard Portal và nhắc hạn không lọc theo trạng thái template, nên vẫn hiện và vẫn nhắc.
- [Đã xác minh] DEV:
  - 312 hồ sơ Draft của 4 công ty thuộc **6 template QA** (`qa-...`, lưu trữ ngày 11/10) đang bị ẩn khỏi tab Cảnh báo;
  - [Theo review] 3.120 mốc nhắc chưa gửi thuộc các hồ sơ này.
- [Đã xác minh] Hồ sơ chưa có trạng thái kết thúc kiểu "Huỷ / Không còn áp dụng". DEV chỉ có Draft, PendingReview, NotStarted.

**Mâu thuẫn:** bộ lọc dọn dữ liệu QA đang áp cho cả nghĩa vụ thật, trái OQ-006 và v1.2, và mỗi màn hình cho ra một kết quả khác nhau.

### Phương án
| | Mô tả | Ưu | Nhược |
|---|---|---|---|
| A. Giữ hiện trạng | Ẩn ở tab Cảnh báo; các màn khác vẫn hiện, vẫn nhắc | Không phải làm gì | Trái OQ-006 và v1.2. Email nhắc về một mục người dùng không tìm thấy. Rủi ro nộp trễ |
| **B. Hiện theo `ObligationOpen` ở mọi màn** | Hồ sơ đang mở vẫn hiện, gắn nhãn "Mẫu đã ngừng áp dụng", vẫn nhắc. Không sinh kỳ, không tạo hồ sơ mới | Đúng quyết định đã có. Một quy tắc cho mọi màn | 312 hồ sơ QA trên DEV sẽ hiện lại, cần dọn trước |
| C. Ẩn ở mọi màn và dừng nhắc | Nhất quán | — | Nghĩa vụ pháp lý thật biến mất mà không ai hay, rủi ro bị phạt theo NĐ156 |
| B + đóng nghĩa vụ | Như B. Khi lưu trữ, Admin Web có thể chọn đóng các hồ sơ chưa bắt đầu, kèm lý do (cần trạng thái kết thúc mới) | Xử lý được trường hợp quy định thật sự bỏ nghĩa vụ. Có công cụ dọn dữ liệu test đúng cách | Thêm trạng thái, audit và thông báo |

### Khuyến nghị
**B ngay; bổ sung "đóng nghĩa vụ" ở bước sau.**
- Nghĩa vụ CBTT phát sinh từ quy định, không phụ thuộc việc nền tảng còn dùng mẫu hay không. Lưu trữ mẫu chỉ là thao tác trên danh mục.
- B khớp quyết định PO đã có (OQ-006) và v1.2, nên không cần đặt ra quy tắc mới.
- Đóng nghĩa vụ phải là thao tác tường minh, có lý do và audit, chứ không phải hệ quả ngầm của trạng thái template. Đây cũng là cách đúng để dọn dữ liệu test, thay cho việc ẩn.

### Khung thiết kế
- Một predicate `ObligationOpen` dùng chung cho `deadlinealerts`, `personalops`, dashboard và `reminder`.
- Trạng thái template chỉ ảnh hưởng tới: danh mục Portal, việc seed kỳ mới, việc tạo hồ sơ mới (đã có 409 `TEMPLATE_ARCHIVED`).
- Response có thêm `template_archived`, để FE gắn nhãn.
- **Khi lưu trữ:**
  - kỳ chưa sinh hồ sơ chuyển sang `SUSPENDED_BY_ARCHIVE`, không materialize;
  - hộp thoại cho Admin Web thấy số hồ sơ đang mở theo từng công ty.
- **Khi khôi phục:** Admin Web chọn bỏ qua hay tạo bù từng kỳ đã bị treo (gắn với Decision 4 ngày 21/08).
- **Bước sau, đóng nghĩa vụ:**
  - trạng thái hồ sơ mới `Cancelled` kèm `close_reason=TEMPLATE_RETIRED`;
  - hồ sơ ra khỏi `ObligationOpen`, các nhắc chưa gửi bị huỷ;
  - có audit và thông báo cho doanh nghiệp.

### Kịch bản lỗi
- Khôi phục sau nhiều tuần: các kỳ cũ đã quá hạn không được tự tạo hàng loạt khi Admin Web chưa chọn.
- Lưu trữ đúng lúc worker đang tạo hồ sơ:
  - kỳ đã được claim thì hoàn tất bình thường;
  - kỳ chưa claim thì bị treo, đánh dấu trong cùng transaction với việc lưu trữ.

### Dữ liệu và rollout
- **B:** không đổi schema; chỉ đổi query và nhãn FE. FE cũ vẫn chạy.
- **DEV:** trước khi bật B, dọn 312 hồ sơ QA (cần kế hoạch sửa dữ liệu riêng và người duyệt), hoặc chấp nhận chúng hiện lại.
- **Prod:** cần đếm số hồ sơ đang mở của template đã lưu trữ trước khi deploy [chưa có số liệu].

### Test chấp nhận
- Template đã lưu trữ có hồ sơ Draft:
  - hồ sơ hiện ở tab Cảnh báo, Việc của tôi và dashboard, có nhãn;
  - nhắc hạn vẫn gửi;
  - không sinh kỳ mới; tạo hồ sơ mới trả 409.
- Khôi phục template: không tự tạo hồ sơ cho các kỳ đã quá hạn khi chưa chọn.
- (Bước sau) Đóng nghĩa vụ: hồ sơ biến mất ở mọi màn, không còn nhắc, có audit.

---

## Q2. Sửa lịch hoặc hạn của template: kỳ đã tạo trước và hồ sơ đang mở

### Hiện trạng
- [Đã xác minh] Kỳ định kỳ là "sticky": upsert **không** cập nhật `cycle_start`, `open_at`, `due_date` của kỳ đã có (`internal/disclosure/infra/mysql/repository.go:2480-2493`). Hồ sơ lấy `planned_date` từ kỳ lúc được tạo.
- [Theo review]
  - Kỳ được tạo trước tối đa 90 ngày (`periodic_cycle_generation_lead_days` từ 0 đến 90).
  - Activate không đụng tới kỳ (`ActivateTypeVersion`, `repository.go:1414-1530`).
- [Đã xác minh] v1.2 AC-14: "Cùng template và kỳ: preview CMS, worker và tab Cảnh báo ra **cùng** ngày hạn." Quy định này bị vi phạm khi kỳ đã tạo trước vẫn giữ hạn cũ, còn preview hiện hạn mới.
- [Đã xác minh] Khi đổi tần suất:
  - nhãn kỳ daily và weekly cùng dạng `YYYY-MM-DD` (`internal/disclosure/app/effective_schedule.go:243-246`);
  - unique key `(type_id, company_id, cycle_label)` (migration 0039) không chứa tần suất, nên đổi qua lại daily/weekly làm kỳ đè nhau.
  - [Theo review] Đổi monthly sang quarterly sinh hai nghĩa vụ cho cùng một giai đoạn.
- [Theo review] Lúc tạo hồ sơ, hệ thống không kiểm lại `applicable_to` hay `auto_create_enabled`. Đổi lịch nghỉ cũng không tính lại hạn.
- [Đã xác minh] Tài liệu ngày 21/08 đã khuyến nghị, nhưng vẫn ở trạng thái `WAIT_FOR_PO_CONFIRMATION`:
  - **Decision 7:** "giữ hồ sơ đã tạo, thay thế các kỳ chưa sinh hồ sơ khi Activate";
  - **Decision 8:** "điều chỉnh hạn từng kỳ, có audit".
- [Đã xác minh] DEV: 1 template active có lead days; 4 kỳ đã tạo trước chưa sinh hồ sơ (hạn 14/11).
- **Pháp lý** (`cobo-domain-cbtt`):
  - TT68/2024 (hiệu lực 02/11/2024) không đổi hạn của tổ chức phát hành.
  - Dự thảo thay TT96 (báo cáo thường niên 120 ngày) chưa ban hành.
  - Khi văn bản mới đổi hạn, **điều khoản chuyển tiếp** của văn bản đó quyết định kỳ nào áp hạn mới [Giả định: theo thông lệ; phải đọc văn bản cụ thể].

### Phương án
| | Mô tả | Ưu | Nhược |
|---|---|---|---|
| A. Giữ sticky | Như hiện tại | Không phải làm gì | Kỳ tạo trước (tới 90 ngày) giữ hạn cũ. Vi phạm AC-14. Đổi tần suất sinh nghĩa vụ trùng |
| B. Thay thế kỳ chưa sinh hồ sơ khi Activate | Kỳ `record_id IS NULL` bị đánh dấu thay thế và sinh lại theo quy tắc mới. Hồ sơ đã tạo giữ nguyên | Đúng Decision 7. AC-14 đúng cho mọi kỳ chưa sinh hồ sơ | Hồ sơ đang mở vẫn giữ hạn cũ |
| C. B + tự cập nhật hạn hồ sơ đang mở | Tính lại `planned_date` của mọi hồ sơ Draft | Mọi nơi chung một hạn | Đổi hạn của việc đang làm. Kỳ trước ngày văn bản có hiệu lực bị áp nhầm hạn mới. Nhắc đã gửi bị lệch |
| **D. B + "Áp dụng từ kỳ" + tuỳ chọn cập nhật hồ sơ mở** | Khi Activate, Admin Web chọn kỳ đầu tiên áp quy tắc mới (mặc định: kỳ chưa sinh hồ sơ sớm nhất). Có tuỳ chọn, mặc định tắt, để cập nhật hạn các hồ sơ đang mở từ kỳ đó, kèm audit và thông báo | Khớp cách văn bản pháp luật quy định thời điểm áp dụng. Kiểm soát được | Phức tạp hơn B: thêm bước preview và lựa chọn |

### Khuyến nghị
**D, làm theo 2 bước:**
1. B, kèm preview tác động khi Activate.
2. "Áp dụng từ kỳ", tuỳ chọn cập nhật hồ sơ mở, và điều chỉnh hạn từng hồ sơ có audit (Decision 8).

Lý do:
- Hồ sơ đã tạo là bản chụp bất biến, cần giữ cho audit.
- Kỳ chưa sinh hồ sơ mới chỉ là kế hoạch, nên phải theo quy tắc đang hiệu lực.
- Thời điểm áp dụng do văn bản pháp luật quy định, nên Admin Web phải chọn được.

Các sửa đi kèm sau đây là lỗi kỹ thuật, **không cần PO quyết**:
- nhãn kỳ có tiền tố tần suất;
- lúc tạo hồ sơ kiểm lại `applicable_to` và `auto_create_enabled`;
- upload lịch nghỉ dùng chung cơ chế thay thế, áp cho template tính theo ngày làm việc.

### Khung thiết kế
- **Trạng thái kỳ:** thêm `SUPERSEDED`, ghi lý do và version mới. Kỳ mới được seed với khoá không đụng dòng cũ (đổi unique key, hoặc thêm namespace cho nhãn).
- **Activate:**
  - cùng transaction với việc đổi `active_version_no`, đánh dấu `SUPERSEDED` cho mọi kỳ chưa claim của template;
  - kỳ đã claim thì hoàn tất theo quy tắc cũ.
- **Preview Activate:** số kỳ và số công ty bị thay thế, hạn cũ → hạn mới, và (nếu chọn) các hồ sơ đang mở bị ảnh hưởng.
- **Audit:** sự kiện `template.activate.cycles_superseded`, kèm danh sách.

### Kịch bản lỗi
- Worker claim kỳ đúng lúc Activate: Activate bỏ qua kỳ đã claim; lúc tạo hồ sơ, worker kiểm lại trạng thái kỳ.
- Activate lỗi giữa chừng: rollback toàn bộ, vì nằm trong một transaction.
- Nhiều replica cùng seed lại: idempotent theo khoá mới.

### Dữ liệu và rollout
- **Migration:** trạng thái kỳ mới, đổi unique key hoặc namespace cho nhãn (áp `backend-db-migration-safe`).
- **Thứ tự:** BE sau cờ `PERIODIC_SUPERSEDE_ON_ACTIVATE` → FE preview → bật cờ.
- Không cần sửa dữ liệu cũ. 4 kỳ đang lệch trên DEV sẽ được thay khi template được Activate lại.

### Test chấp nhận
- Đổi `deadline_days` từ 20 xuống 10 rồi Activate:
  - kỳ tạo trước chưa sinh hồ sơ nhận hạn mới;
  - hồ sơ đã tạo giữ hạn cũ;
  - preview, worker và tab Cảnh báo ra cùng một ngày (AC-14).
- Monthly → quarterly: không có hai nghĩa vụ cho cùng một giai đoạn. Daily → weekly: không mất tuần nào.
- (Bước 2) Chọn "cập nhật hồ sơ mở": hạn đổi, có audit, doanh nghiệp nhận thông báo.

---

## Q3. Loại template của doanh nghiệp và việc tự sinh kỳ

### Hiện trạng
- [Đã xác minh] Form tạo template ở Portal:
  - cho chọn "Định kỳ" (mặc định) hoặc "Bất thường", kèm ô tần suất và quy tắc hạn dạng văn bản tự do (`src/pages/portal/disclosure-type-list/CreateCompanyTypeModal.tsx:99,140-145`);
  - request BE chỉ có `periodicity` và `deadline_rule` dạng chuỗi (`internal/disclosure/app/contracts.go:504-515`).
- [Đã xác minh] Worker chỉ seed template có `deadline_config_json.template_category IN ('periodic','custom')` và `frequency_unit` hợp lệ (`internal/disclosure/infra/mysql/repository.go:2425-2445`). Template doanh nghiệp được tạo không có `deadline_config_json`. Hệ quả: template doanh nghiệp "Định kỳ" **không bao giờ sinh kỳ, cảnh báo hay nhắc**, và người dùng không được báo gì.
- [Theo review] Đường CMS cho template thuộc công ty chỉ nhận `custom` (`internal/disclosure/app/service.go:902`), còn Portal chỉ nhận `periodic` hoặc `irregular` (`internal/disclosure/app/template_validation.go:170`).
- [Đã xác minh] Hai tài liệu định nghĩa `custom` khác nhau:
  - contract §3: "Tần suất tùy chỉnh", chạy cùng worker;
  - business guide: không còn là loại template, mà là phạm vi sở hữu.
- [Đã xác minh] DEV: 6 template doanh nghiệp, tất cả "Bất thường", đang draft, thuộc 1 công ty, chưa có hồ sơ.

### Phương án
| | Mô tả | Ưu | Nhược |
|---|---|---|---|
| **A. "Định kỳ" tự sinh kỳ như template hệ thống** | Form Portal bắt buộc lịch có cấu trúc: tần suất, mốc T, số ngày, loại ngày. Dùng lại bộ cấu hình lịch của CMS. Worker chỉ seed kỳ cho công ty sở hữu | Đúng kỳ vọng của người chọn "Định kỳ"; có cảnh báo và nhắc | Phải làm UI cấu hình lịch ở Portal; phải quy định khi nào bắt đầu sinh kỳ |
| B. Chỉ cho "Bất thường" | Bỏ "Định kỳ" ở Portal; nghĩa vụ định kỳ dùng template hệ thống | Đơn giản, hết mâu thuẫn ngay | Doanh nghiệp không tự quản được nghĩa vụ định kỳ nội bộ hay đặc thù |
| C. "Định kỳ" chỉ là nhãn, hồ sơ tạo tay | Giữ như hiện tại nhưng ghi rõ "không tự tạo kỳ" | Ít việc | Dễ hiểu lầm; không có nhắc nếu chưa tạo tay; không đạt mục tiêu sản phẩm |

Về `custom`: bỏ khỏi danh sách loại template. Quyền sở hữu đã thể hiện qua phạm vi (công ty). Đường CMS cho template thuộc công ty dùng chung bộ kiểm tra với Portal.

### Khuyến nghị
**A. Trong lúc chờ A, tạm áp B** (ẩn "Định kỳ" ở form Portal) để không tạo kỳ vọng sai.
- PO xác nhận thêm một điểm: chỉ sinh kỳ khi template doanh nghiệp đã `published`, không sinh nghĩa vụ từ bản nháp.
- Lý do:
  - người dùng chọn "Định kỳ" là để hệ thống nhắc theo kỳ;
  - DEV chưa có dữ liệu, nên chuyển đổi lúc này rẻ nhất.

### Khung thiết kế
- Template doanh nghiệp dùng `deadline_config_json` cùng schema với template hệ thống, `template_category=periodic`.
- Seed đã hỗ trợ template có chủ sở hữu (chỉ seed cho công ty sở hữu).
- Validate bằng một hàm chung cho Portal và CMS:
  - `irregular` không có lịch;
  - `periodic` bắt buộc có lịch.
- Sửa lịch thì theo quy tắc Q2; lưu trữ thì theo Q1 và Q6.

### Dữ liệu và rollout
- **Thứ tự:** FE trước (ẩn hoặc chuyển form), rồi mới BE bắt buộc lịch. Nếu làm ngược, FE cũ gửi "Định kỳ" không có lịch sẽ nhận 422.
- **DEV:** không cần chuyển đổi.
- **Prod:** cần đếm template doanh nghiệp "Định kỳ" hiện có [chưa có số liệu]. Nếu có, bổ sung lịch cho chúng hoặc chuyển sang "Bất thường".

### Test chấp nhận
- Tạo "Định kỳ" thiếu lịch: 422.
- Có lịch và đã published: worker sinh kỳ đúng cho công ty sở hữu, không sinh cho công ty khác.
- "Bất thường": không sinh kỳ.

---

## Q4. Ý nghĩa của `due_rule` so với `processing_days`

### Hiện trạng
- [Đã xác minh] Engine lập lịch bước **neo vào T0 và cộng dồn** (`internal/disclosure/app/timeline.go:59-88`):
  - bước 1 bắt đầu ở T0; bước i kết thúc ở T0 + (pd₁ + … + pdᵢ) − 1, tính cả hai đầu;
  - lịch chỉ dịch về sau theo số ngày trễ được ghi nhận (`internal/deadlinealerts/app/steps_algorithm.go:292-315`).
- [Đã xác minh] Nguồn số ngày của bước là `processing_days`. `due_rule` chỉ được dùng khi `processing_days = 0`; nếu vẫn không có thì mặc định 1 ngày (`timeline.go:219-231`).
- [Đã xác minh] File mẫu import ghi `due_rule` **cộng dồn**: 10/T+10, 5/T+15, 3/T+18. Cách ghi này khớp lịch của engine, chỉ lệch quy ước đếm ngày (xem 4b).
- [Theo review] CMS editor ghi `due_rule = T+processing_days` **theo từng bước** (T+10, T+5, T+3). `CLAUDE.md` cũng mô tả như vậy.
- [Đã xác minh] Portal hiển thị theo `due_rule` và hiểu là thời lượng của riêng bước: "Trong vòng N ngày kể từ khi bước bắt đầu" (`src/services/formatWorkflowStepDeadlineRule.ts:38-55`). Vì vậy template import hiện "110 ngày" cho một bước chỉ có 22 ngày.
- [Theo review]
  - Rebase override so sánh `due_rule` mà không đổi `processing_days`.
  - Ad-hoc sửa `processing_days` nhưng giữ `due_rule` cũ.
  - `H+N` bị đổi âm thầm thành 1 ngày ở runtime.

### Phương án
| | Mô tả | Ưu | Nhược |
|---|---|---|---|
| **A. `processing_days` là nguồn duy nhất** | `due_rule` không còn là dữ liệu nhập; server ngừng lưu và đọc (vẫn trả thêm 1 phiên bản để tương thích). Màn hình hiển thị "Xử lý trong N ngày · Hạn bước dd/mm/yyyy", với ngày do engine tính | Khớp engine hiện tại. Hết hiểu nhầm "T". Sửa ít | Mất nhãn T+N quen thuộc với BA. CMS có thể hiện thêm "ngày a–b tính từ T" |
| B. `due_rule` cộng dồn là nguồn | Nhập mốc T+X cho từng bước; `processing_days` được suy ra | Khớp cách BA viết file import | Đổi ngữ nghĩa engine, override và ad-hoc. Dễ sai khi chèn hoặc xoá bước |
| C. Giữ cả hai, bắt nhất quán | Từ chối khi `due_rule` lệch quy tắc | Ít đổi code | Vẫn phải chọn một nghĩa cho T+N. Hai trường cùng mang một thông tin |

### Khuyến nghị
**A.**
- Import vẫn nhận `due_rule` để tương thích nhưng bỏ qua, kèm cảnh báo. Sửa file mẫu và hướng dẫn import.
- Từ chối `H+N` và `processing_days < 1` ở mọi đường ghi: CMS, override, import, ad-hoc.
- Dữ liệu cũ không cần chuyển đổi cho runtime (engine đã dùng `processing_days`). Chỉ cần FE ngừng đọc `due_rule`.
- Cập nhật `CLAUDE.md` và tài liệu hướng dẫn import.

### 4b. Quy ước đếm ngày (Decision 5 ngày 21/08, vẫn đang treo)
- [Đã xác minh] Engine tính hạn hồ sơ theo kiểu **tính cả ngày T** (`addDurationInclusive`, `internal/disclosure/app/deadline_calculator.go:167`): 30/09 + 20 ngày → 19/10. Ví dụ của PO trong tài liệu 21/08 lại kỳ vọng 20/10.
- [`cobo-domain-cbtt`] BLDS 2015 Điều 147: thời hạn tính từ ngày tiếp theo ngày xảy ra sự kiện. Việc áp điều này cho hạn định kỳ của TT96 chỉ là suy luận, **cần pháp chế xác nhận**.
- Phương án:
  - **(i) Giữ "tính cả ngày T":** không bao giờ muộn hơn hạn luật nếu T là ngày xảy ra sự kiện. Nhưng có thể sớm 1 ngày so với văn bản, và lệch ví dụ của PO.
  - **(ii) "Không tính ngày T" theo BLDS Đ147:** khớp văn bản và ví dụ của PO. Nhưng mọi hạn lùi 1 ngày, nên template nào đang đặt T là ngày *sau* sự kiện sẽ thành trễ 1 ngày.
- **Khuyến nghị: (ii).** Chỉ đổi sau khi pháp chế xác nhận và đã rà mốc T của từng template active, có preview hạn trước và sau khi đổi. Trong lúc chờ, giữ (i). Hạn bước nội bộ (SLA) giữ cách đếm thời lượng hiện tại.

### Test chấp nhận
- Bước 22 ngày hiển thị "22 ngày" và đúng ngày hạn mà engine tính.
- Import có `due_rule` lệch: có cảnh báo, lịch không đổi.
- `H+48`: 422.

---

## Q5. Maker-checker cho override workflow của doanh nghiệp

### Hiện trạng
- [Đã xác minh] **D-04** (requirements, đã khoá 24/05): "Lưu override workflow có hiệu lực ngay"; không có vòng draft/publish.
- [Đã xác minh] Kế hoạch custom workflow ngày 17/05, §8, còn để ngỏ hai hướng:
  - (1) giữ 2 bước + maker-checker;
  - (2) dùng `publish:true` cho doanh nghiệp không phải gói Enterprise; "product quyết định".
- [Đã xác minh] Code có **ba đường với ba quy tắc khác nhau**:

  | Đường | Quy tắc hiện tại |
  |---|---|
  | `PUT .../draft` + `publish:true` (nút "Kích hoạt" ở Portal) | Cần quyền duyệt nhưng **bỏ qua kiểm tra tự duyệt** (`internal/disclosure/app/service.go:1304`) |
  | `POST .../approve` | Chặn người soạn tự duyệt. Portal không gọi đường này |
  | `rebase-apply` | Chỉ cần quyền **ghi**, rồi ghi thẳng thành bản active (`internal/disclosure/app/workflow_override_apply.go:78`) |

- [Đã xác minh] Migration 0087 cấp cả 4 quyền override (đọc, ghi, duyệt, reset) cho **mọi** role `admin_doanh_nghiep`.
- [Đã xác minh] DEV:
  - cả 8/8 bản override đã duyệt đều do chính người soạn duyệt;
  - số người có quyền duyệt theo công ty: 7 công ty có 0 người, 8 công ty có 1 người, 3 công ty có 2 người, 3 công ty có nhiều hơn.
- [Theo review]
  - "Merge manual" khi rebase gửi `{}`, làm tắt nhắc hạn của bước.
  - Nút "Kích hoạt" hiện cả với người chỉ có quyền ghi.

### Phương án
| | Mô tả | Ưu | Nhược |
|---|---|---|---|
| A. Bắt buộc hai người | Người kích hoạt phải khác người soạn, ở mọi đường | Kiểm soát chặt | 8/14 công ty có người duyệt chỉ có đúng 1 người, sẽ bị khoá cho tới khi cấp thêm người duyệt |
| **B. Theo quyền** | Người có quyền duyệt được kích hoạt ngay, kể cả bản mình soạn; ghi audit "tự duyệt" và thông báo cho người duyệt khác. Người chỉ có quyền ghi chỉ lưu nháp và gửi duyệt. Mọi đường (kích hoạt, rebase-apply, duyệt) dùng chung một quy tắc | Khớp D-04 ("có hiệu lực ngay" với người có quyền). Doanh nghiệp muốn kiểm soát bốn mắt thì tách role | Doanh nghiệp không tách role thì vẫn chỉ một người quyết |
| C. Cấu hình theo doanh nghiệp | Mặc định là B; doanh nghiệp (hoặc gói Enterprise) bật "Bắt buộc người duyệt khác" thì áp A cho riêng họ | Linh hoạt | Thêm cấu hình và UI |

### Khuyến nghị
**B ngay; C khi có khách hàng yêu cầu** (gói Enterprise).
- Sửa đi kèm:
  - rebase-apply phải có quyền duyệt;
  - `merge_manual` phải có giá trị hợp lệ cho từng field;
  - Portal thêm "Gửi duyệt" cho người chỉ có quyền ghi, và danh sách bản chờ duyệt cho người duyệt.
- Ranh giới vai trò (`cobo-admin-role-guard`): chỉ áp trong công ty của token, không ảnh hưởng Admin Web.

### Khung thiết kế
- **Trạng thái bản override:**
  - `draft` → `pending_approval` (gửi duyệt) → `active` (người có quyền duyệt) hoặc `rejected`;
  - `draft` → `active` trực tiếp nếu người lưu có quyền duyệt (audit `self_approved=true`).
- Rebase-apply tạo bản mới theo cùng quy tắc này.

### Kịch bản lỗi
- Hai người cùng sửa: dùng `base_version_no`/ETag (sửa XL-14).
- Duyệt một bản nháp cũ sau khi đã rebase: từ chối (WF-08c).

### Dữ liệu và rollout
- Không cần chuyển đổi: các bản tự duyệt hiện có vẫn hợp lệ theo B.
- **Thứ tự:** FE (đổi nút theo quyền) đi trước hoặc cùng lúc với BE.

### Test chấp nhận
- Chỉ có quyền ghi: không kích hoạt được (403), nhưng gửi duyệt được.
- Có quyền duyệt: kích hoạt được; audit ghi "tự duyệt".
- Rebase-apply khi chỉ có quyền ghi: 403.
- `merge_manual` với `{}`: 422.

---

## Q6. Template doanh nghiệp bị từ chối hoặc đã lưu trữ

### Hiện trạng
- [Đã xác minh] Bảng vòng đời template doanh nghiệp (`internal/disclosure/app/service.go:2098-2109`):
  - gửi duyệt chỉ từ `draft`;
  - publish và reject chỉ từ `in_review`;
  - lưu trữ chỉ từ `published`.
- [Theo review] Sửa một template `rejected` không đưa nó về `draft`, và không có thao tác xoá.
- [Đã xác minh] Requirements cho phép gửi duyệt hoặc publish từ `draft` **hoặc `rejected`** (`cobo_web_design/docs/cms-portal-template-workflow-requirements.md:421`).
- [Theo review] Quota:
  - đếm mọi template chưa lưu trữ, kể cả `rejected`;
  - lấy gói của **người dùng** thay vì của công ty;
  - lỗi khi đếm thì cho qua; có race.
- [Đã xác minh] Khi tạo hồ sơ, template doanh nghiệp bỏ qua mọi kiểm tra vòng đời, kể cả khi đã lưu trữ (`service.go:2266-2269`). Template hệ thống đã lưu trữ thì trả 409 `TEMPLATE_ARCHIVED`.
- [Đã xác minh] D-03 (đã khoá): luồng tạo hồ sơ chỉ chặn theo `has_workflow`, không theo `review_status`.
- [Đã xác minh] DEV: 6 template doanh nghiệp, đều draft, chưa có hồ sơ.

### Phương án và khuyến nghị
| Câu hỏi | Khuyến nghị | Phương án khác |
|---|---|---|
| 6a. Template bị từ chối | Sửa thì tự quay về `draft`; cho gửi duyệt lại từ `rejected` (đúng requirements) | Giữ ngõ cụt như hiện tại |
| 6b. Lưu trữ | Cho lưu trữ từ mọi trạng thái chưa lưu trữ: draft, rejected, in_review, published | Chỉ từ published như hiện tại |
| 6c. Xoá | Không xoá cứng; lưu trữ là cách giải phóng quota | Cho xoá template chưa từng dùng (không có hồ sơ, override hay đề xuất) |
| 6d. Template đã lưu trữ | Không tạo hồ sơ mới, không sinh kỳ, giống template hệ thống. Hồ sơ đang mở vẫn tiếp tục (theo Q1). Cách hiểu D-03: không chặn theo draft/in_review/rejected/published, **nhưng chặn archived** | Vẫn tạo được hồ sơ như hiện tại |
| 6e. Quota | Đếm template chưa lưu trữ; lấy gói của **công ty** (`company_subscriptions`); lỗi đếm thì trả lỗi thay vì cho qua; đếm và tạo trong cùng một transaction | Lấy theo gói người dùng là sai nguồn sự thật, tức là lỗi, không cần PO quyết |

### Dữ liệu và rollout
- DEV không cần chuyển đổi.
- Prod cần đếm template doanh nghiệp theo trạng thái [chưa có số liệu].

### Test chấp nhận
- `rejected` → sửa → `draft` → gửi duyệt được.
- Lưu trữ từ `draft` thì giải phóng quota.
- Tạo hồ sơ từ template doanh nghiệp đã lưu trữ: 409 `TEMPLATE_ARCHIVED`.
- Công ty gói Premium có admin mang tier Free: không bị chặn ở mức 5 template.

---

## Không cần PO quyết: đã có quy định, chỉ cần làm
| Mục | Quy định | Phát hiện |
|---|---|---|
| Ad-hoc: với ngày chốt, giá trị nhập sau cùng thắng; ô để trống không xoá giá trị trước | v1.2 §3.2 | H1 (XL-01, WF-10) |
| Ad-hoc: đủ phiếu thì không cho Reject/Cancel (409); finalize lỗi phải bù trừ | v1.2 §3.2 | H2, H3 |
| Ad-hoc: submit phải kiểm lại reviewer | v1.2 §3.2 | XL-06 |
| Công ty `suspended` vẫn tạo kỳ và vẫn nhắc; `inactive` thì dừng | v1.2 §2.2 | H13 (DL-03) |
| Submit thủ công: T0 = ngày submit | v1.2 §2.2 | WF-11 |
| `published` cần evidence link | `CLAUDE.md` | WF-12 |
| Quota theo gói của công ty, kiểm ở server | `CLAUDE.md`, migration 0125 | TPL-07 |
| Workflow nhiều bước hoàn tất ngay từ bước 1; người submit tự duyệt | Lỗi: trái chính định nghĩa workflow nhiều bước | WF-01 (Critical) |

## Quyết định kỹ thuật: nhóm dev/SA chốt, không cần PO
- Runtime chuẩn cho instance template là task hay step-state (WF-01, WF-02, WF-06).
- Nguồn để so override lỗi thời: dùng version active của template thay vì `global_workflows` (H7).
- Nhãn kỳ có tiền tố tần suất; lúc tạo hồ sơ kiểm lại `applicable_to` và `auto_create_enabled` (Q2).

---

## Phụ lục A: các phát hiện liên quan (16/10/2026)
| ID | Mức | Tóm tắt | Vị trí chính |
|---|---|---|---|
| WF-01 | Critical | Duyệt task đầu là hoàn tất workflow nhiều bước; người submit tự duyệt | `internal/workflow/app/service.go:117-137,508-550`; `internal/disclosure/infra/workflow/bootstrap.go:50-61` |
| H1 (XL-01, WF-10) | High | Ad-hoc mất ngày chốt của người duyệt trước | `internal/adhoc/infra/mysql/repository.go:483-521`; `internal/adhoc/app/service.go:553-606` |
| H2 (XL-05, WF-04) | High | Reject/Cancel được sau khi đủ phiếu | `internal/adhoc/app/service.go:760-843` |
| H3 (XL-04, WF-05) | High | Finalize lỗi: kẹt ở FE, hoặc duyệt mà không có workflow instance | `AdHocProposalDetailPage.tsx:323-327`; `internal/adhoc/infra/disclosure/record_creator.go:115-135` |
| H4 (WF-02) | High | Approve bỏ qua cổng tài liệu bắt buộc | `internal/workflow/app/service.go:333-374` |
| H5 (WF-06) | High | Hai runtime không đồng bộ; không kiểm người phụ trách bước | `internal/deadlinealerts/app/steps_service.go:48-83` |
| H6 (WF-03, XL-13, XL-15) | High | Override vượt maker-checker; `merge_manual {}` tắt nhắc hạn | `workflow_override_apply.go:78`; `service.go:1304`; `workflow_override_patch_apply.go:350-397` |
| H7 (XL-03, TPL-05, WF-08a) | High | Phát hiện override lỗi thời so với `global_workflows` | `workflow_override_staleness_repository.go:87-97` |
| H8 (XL-02, XL-16, WF-09) | High | `due_rule` mang 3 nghĩa | `timeline.go:219-231`; `formatWorkflowStepDeadlineRule.ts:38-55` |
| H9 (TPL-01, DL-04) | High | Lưu trữ làm ẩn nghĩa vụ đang mở | `active_template.go:13-17`; `cms_repository.go:428` |
| H10 (DL-01, TPL-04) | High | Sửa template không lan tới kỳ đã tạo trước | `repository.go:2480-2493` |
| H11 (DL-02) | High | Đổi tần suất làm trùng hoặc đè kỳ | `effective_schedule.go:243-246`; migration 0039 |
| H12 (TPL-02) | High | Template doanh nghiệp "Định kỳ" không sinh kỳ | `repository.go:2425-2445` |
| H13 (DL-03) | High | Công ty suspended không được seed kỳ | `repository.go:2803-2804` |
| DL-05 | Medium | Lưu trữ treo kỳ; khôi phục làm sống lại kỳ cũ | `repository.go:2604-2626` |
| DL-06 | Medium | Tạo hồ sơ không kiểm lại `applicable_to`/`auto_create_enabled` | `periodic.go:382-456` |
| DL-08 | Medium | Đổi lịch nghỉ không tính lại hạn | `internal/holiday/app/service.go` |
| DL-09 | Medium | Preview CMS và worker dùng loại ngày khác nhau | `first_occurrence_preview.go:68-80` |
| TPL-06, XL-17 | Medium | Template doanh nghiệp bị từ chối rơi vào ngõ cụt | `service.go:2098-2109` |
| TPL-07 | Medium | Quota lấy theo tier người dùng | `service.go:2136-2139` |
| XL-14, WF-08b/c | Medium | Override bị ghi đè lẫn nhau; duyệt được bản nháp cũ | `repository.go:1633-1645` |
| XL-06 | Medium | Submit đề xuất ad-hoc không kiểm lại reviewer | `internal/adhoc/app/service.go:378-389` |
| WF-11 | Medium | Submit thủ công: T0 sai, có thể sinh 2 instance | `bootstrap.go:45-61` |
| WF-12 | Medium | Không kiểm vòng đời hồ sơ; publish không cần evidence link | `internal/disclosure/app/service.go:268-387` |
| TPL-09 | Low | File mẫu import bị chính validator từ chối (3 test đỏ trong baseline) | `internal/disclosure/app/artifacts/cobo-template-import-example-v1.0.json` |
