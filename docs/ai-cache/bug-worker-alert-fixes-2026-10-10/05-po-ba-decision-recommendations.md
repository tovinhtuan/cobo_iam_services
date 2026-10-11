# Khuyến nghị PO/BA để chốt các điểm còn mở (2026-10-10)

Vai trò: PO/BA cho hệ thống quản lý nghĩa vụ CBTT (TT96/2020, NĐ 156/2020). Khuyến nghị dựa trên việc đối chiếu contract hiện hành, quyết định sản phẩm đã có, code ở HEAD `a8acb51` và dữ liệu DEV (qa_ro).

## 0. Nguồn contract đã đối chiếu

| Nguồn | Hiệu lực | Nội dung chính |
|---|---|---|
| `docs/ai-cache/business-contract-workflow-deadline-final.md` v1.1 (2026-05-27) | Canonical | WF-A, DC-B (terminal → `PENDING_CONFIRM`, confirm → `DONE`), AL-B, AC-1..AC-10 |
| WEB `deadline-alert-domain-review-plan-2026-08-25/08-recommended-domain-contract.md` + `deadline-alert-v1-solution-plan-2026-08-25/` | Triển khai V1 (thay §5.1 trên thực tế) | **Cảnh báo = nghĩa vụ đang mở = `Draft AND submitted_at IS NULL`**. Submit = hết nghĩa vụ. Ad-hoc approve (Create+Submit) → **không** cảnh báo. `OPTIONAL_COMPAT` (terminal → `PENDING_CONFIRM`): "Product must confirm", **chưa từng được chốt** |
| WEB `dashboard-kpi-real-aggregation-2026-08-03/06-approved-kpi-formulas.md` (2026-08-03, **trước** V1) | Đã duyệt | #1 và #5 không theo range; #3 "in range"; #4 và #6 dựa vào terminal + `completed_at` |
| WEB `adhoc-multi-reviewer-single-round-implementation-plan-v3-2026-06-18.md` | Đã triển khai | Một vòng, nhiều reviewer, cần tất cả đồng ý (D4). **A4: ngày chốt là last-writer-wins**, có cảnh báo UI (`AdHocProposalDetailPage.tsx:662-668`) |
| `cobo-domain-cbtt/references/tt96-reference.md` | Pháp lý | Hạn định kỳ tính theo ngày lịch; bất thường 24h; Đ7 k7–8: phát sinh vào ngày nghỉ thì đăng website, hoàn tất CBTT vào ngày làm việc kế tiếp |
| Quyết định user 2026-10-10 (Q-05) | Mới | Tính hạn theo quy định của template; ngày cụ thể là ngày bắt buộc; không tự dời |

### Mâu thuẫn contract phát hiện được (phải xử lý khi chốt)

| # | Mâu thuẫn | Hệ quả đang thấy |
|---|---|---|
| C-1 | Contract §5.1 / AC-4 yêu cầu terminal → `PENDING_CONFIRM`; V1 chỉ giữ Draft, `OPTIONAL_COMPAT` chưa chốt | `PENDING_CONFIRM` không thể xảy ra; KPI hoàn thành #4/#6 luôn bằng 0 (DLA-04) |
| C-2 | Công thức KPI duyệt 08-03 giả định có terminal trong danh sách; V1 (08-25) bỏ terminal mà không sửa công thức | DLA-03, DLA-04 |
| C-3 | V1 coi ad-hoc approve (tự động Submit) là hết nghĩa vụ; one-pager và TT96 coi "duyệt đề xuất" chỉ là **bắt đầu** quy trình công bố 24h | AHA-01: nghĩa vụ 24h biến mất khỏi tab Hạn đúng lúc gấp nhất |
| C-4 | V1 đổi mô hình sang "Draft = còn nghĩa vụ", nhưng các nơi đọc khác vẫn lọc `<> 'draft'` | DLA-02 (enrich trống), **PO-01 mới**: "Việc của tôi" không hiện task định kỳ |
| C-5 | Contract §3.2 vẫn mô tả 2 vòng focal → admin; thực tế đã là một vòng nhiều reviewer (v3) | Chỉ là tài liệu lệch |
| C-6 | A4 last-writer-wins được thiết kế cho "giá trị nhập", nhưng code ghi đè cả khi người duyệt cuối để trống | AHA-03: hạn đã rút ngắn bị xoá âm thầm |

## 1. Nguyên tắc sản phẩm đề xuất làm "xương sống" (Contract v1.2)

1. **Một định nghĩa nghĩa vụ đang mở (`ObligationOpen`) duy nhất, dùng chung cho tab Hạn, dashboard, Việc của tôi và reminder:**
   - Kỳ định kỳ / tuỳ chỉnh / tạo thủ công: `Draft AND submitted_at IS NULL` (giữ đúng V1).
   - Bất thường (ad-hoc): **từ khi đề xuất được duyệt cho tới khi workflow công bố hoàn tất** (record sang Approved/Published/Completed), bất kể việc tự động Submit lúc duyệt.
2. **Một định nghĩa thời điểm hoàn thành nghĩa vụ (`CompletedAt`)**, ưu tiên theo thứ tự:
   1. `confirmed_at`: xác nhận đã công bố, kể cả công bố ngoài hệ thống;
   2. định kỳ / thủ công: `submitted_at`;
   3. ad-hoc: thời điểm record đạt Approved/Published/Completed.
   KPI đúng hạn so `CompletedAt` (theo ngày HCM) với hạn.
3. **Hạn tính theo đúng quy định của template** (Q-05). Ngày cụ thể là bắt buộc. Hệ thống không tự diễn giải lại luật.
4. **Thông báo phục vụ việc hoàn thành nghĩa vụ, không gây nhiễu:** đúng người, đúng bước, trong giờ hành chính rộng, không gửi bản tin trễ vô nghĩa.
5. **Fail-safe cho tuân thủ:** khi thiếu cấu hình thì nghiêng về **vẫn nhắc** (mặc định) thay vì im lặng. Ngược lại, khi chưa chắc dữ liệu đúng thì **không** tự sửa hạn pháp lý.

## 2. Khuyến nghị cho từng điểm

> Ký hiệu: ✅ chốt theo khuyến nghị; ⚠️ khuyến nghị **khác** với kế hoạch trước (đã cập nhật `02-implementation-plan.md`).

### Nhóm A: hạn pháp lý
- **Q-04 (loại ngày):** ✅ Theo Q-05. Resolver dùng khai báo của template (`rules.deadline_day_type` → `deadline_duration_type`). Fallback ngày lịch chỉ khi template không khai báo, và **bắt buộc khai báo khi publish template**.
  - Lý do: TT96 dùng "ngày" cho hạn định kỳ; DEV 12/12 template đã khai báo.
  - AC: preview CMS, worker và tab Hạn ra **cùng một ngày** cho cùng template/kỳ.
- **Q-05:** đã chốt.
- **Q-06 (ngày nghỉ 2026/2027):** ✅
  - File cấu hình chỉ chứa các ngày **chắc chắn**:
    - 2026: 01/01; Tết 16–20/02; 27/04 (bù Giỗ Tổ); 30/04; 01/05; 02/09.
    - 2027: 01/01; 30/04; 03/05 (bù 01/05); 02/09.
  - Các ngày phụ thuộc thông báo hằng năm (ngày nghỉ liền kề 2/9, Tết 2027, Giỗ Tổ 2027, ngày hoán đổi) **nhập qua CMS** sau khi có văn bản chính thức (Bộ Nội vụ / Thủ tướng). Việc này cần DUE-01 đã được sửa.
  - Quy trình vận hành: **tháng 10 mỗi năm** BA cập nhật lịch năm sau; hệ thống cảnh báo khi `holiday_calendar_year_available{next}=0`.
  - Lý do: không hard-code điều chưa ban hành; DB là nguồn sự thật, file chỉ là lưới an toàn. Lịch nghỉ chỉ ảnh hưởng template ngày làm việc (3 template trên DEV) và ad-hoc khai báo ngày làm việc.
- **Q-07 (T0 khi submit thủ công):** ✅ T0 = **ngày submit (giờ HCM)**, `t0_policy=system_date`. Nếu tổng `processing_days` vượt hạn thì hiện cảnh báo "Quy trình dự kiến vượt hạn" (Q-08).
  - Lý do: công việc bắt đầu khi hồ sơ được nộp vào quy trình. Lấy T0 = hạn khiến mọi bước kết thúc sau hạn, sai bản chất.
- **Q-08:** ✅ Chỉ phát hiện và cảnh báo; không tự nén quy trình (nén là quyết định quản trị của doanh nghiệp).
- **Q-09:** ✅ Tạo kỳ trước tại `max(lead, open_days_before_t)`. Preview hứa ngày mở nào thì hệ thống phải giữ đúng ngày đó.
- **Q-10 (Draft không có `planned_date`):** ✅ Neo vào kỳ chứa `created_at`, **và ghi luôn** `planned_date` khi tạo record thủ công có template tính được hạn (không tính lại mỗi lần đọc).
  - AC: hạn của một record không đổi theo thời gian; một record lỗi không làm sập cả danh sách.

### Nhóm B: ad-hoc (CBTT bất thường)
- **Q-12 (ad-hoc đã duyệt trong tab Hạn):** ✅⚠️ Chọn **(a′)**, sửa contract C-3.
  - Record ad-hoc nằm trong tab Hạn **từ lúc duyệt tới khi record đạt Approved/Published/Completed**.
  - SQL: nhánh OR `EXISTS approved proposal AND status NOT IN ('approved','published','completed','done')`.
  - Giữ tự động Submit lúc duyệt (không đổi luồng ghi, không đụng personalops/enrich theo kiểu phương án b).
  - Lý do: nghĩa vụ 24h chỉ thật sự bắt đầu sau khi đề xuất được duyệt. Ẩn nó đi là rủi ro tuân thủ nghiêm trọng nhất trong cả bộ lỗi (phạt 50–70 triệu cho chậm CBTT, NĐ 156 Đ42).
  - AC:
    - đề xuất duyệt xong → xuất hiện ngay với hạn 24h / hạn chốt;
    - bước cuối hoàn tất → biến mất;
    - không trùng với proposal pending.
- **Q-13 (ngày chốt khi nhiều người duyệt):** ⚠️ **Giữ A4 của contract v3 (last-writer-wins), nhưng hiểu là "giá trị được nhập cuối cùng"**, tức ô để trống **không xoá** giá trị người trước đã nhập. Đồng thời **sửa DUE-06**: hạn hiển thị phải dùng `final_t0_date` khi có.
  - Khuyến nghị "lấy ngày sớm nhất" trong kế hoạch trước bị huỷ, vì trái A4 và trái cảnh báo UI đã phát hành.
  - Bổ sung UX: trong modal duyệt, hiện giá trị hiện hành ("Hạn chốt hiện tại: …, do A nhập lúc …"), để người duyệt sau biết mình đang ghi đè.
  - AC: A nhập 11/10, B duyệt để trống → giữ 11/10; B nhập 12/10 → 12/10 (đúng cảnh báo UI).
- **Q-14:** ✅ F2. Kiểm quyền người tạo và tính hợp lệ của reviewer **lúc gửi**, báo lỗi rõ ràng. Không mở cơ chế materialize "nội bộ không authz".
- **Q-15:** ✅⚠️ **Cả A và B.**
  - A (quy tắc nghiệp vụ): khi đã đủ phiếu thì quyết định đã xong, không cho Reject/Cancel nữa (409 "Đề xuất đã được duyệt đủ").
  - B (an toàn kỹ thuật): nếu finalize lỗi giữa chừng thì bù trừ.
  - Lý do: tránh tình trạng "đã duyệt nhưng bị từ chối" gây mất dấu nghĩa vụ.
- **Q-16 (thông báo khi được giao bước):** ⚠️ Nâng lên **P1, làm ngay sau PR-3** (in-app, sau flag).
  - Lý do: khi RMC-01 được sửa, bước 2..N mới có nhắc 3 ngày / 1 ngày trước hạn. Người nhận vẫn cần biết **ngay khi tới lượt mình**, nhất là ad-hoc 24h, nơi nhắc "3 ngày trước" vô nghĩa.

### Nhóm C: tab Hạn / dashboard / "Việc của tôi"
- **PO-01 (MỚI, HIGH, confirmed):** `personalops` `ListMineRecords` / `ListMineOpenTasks` lọc `dr.status <> 'draft'` (`internal/personalops/infra/mysql/repository.go:115,246`).
  - Hệ quả: **toàn bộ task của kỳ định kỳ (record Draft) không hiện trong "Việc của tôi"**. DEV có 308 record Draft có task mở.
  - Khuyến nghị: dùng chung `ObligationOpen` (§1); đưa vào PR-4 cùng DLA-02.
  - AC: người được giao task định kỳ thấy task trong "Việc của tôi".
- **Q-17 (nhân viên phòng ban thấy cảnh báo của bước thuộc phòng mình):** ✅ Có, đây là đúng mục đích của phạm vi dữ liệu theo phòng ban. Làm P1 sau PR-4, có đo hiệu năng. Tạm thời người được giao task đã thấy (DLA-02).
- **Q-18 (KPI dashboard):** ✅
  - Các KPI trạng thái mở (#1 cần xử lý ngay, #3 đang xử lý, #5 quá hạn chưa xong) là **ảnh chụp hiện tại, không theo range**.
  - #2 dùng cửa sổ 7 ngày.
  - Range chỉ áp cho KPI hoàn thành (#4, #6) và biểu đồ xu hướng.
  - Sửa câu chữ #3 trong `06-approved-kpi-formulas.md`.
  - Lý do: #1 và #5 vốn đã duyệt là không theo range (code đang sai); "đang xử lý trong 7 ngày qua" không có nghĩa nghiệp vụ.
- **Q-19 (KPI hoàn thành và PENDING_CONFIRM):** ⚠️
  - Hoàn thành = `CompletedAt` theo §1 (gồm `submitted_at`, không chỉ `confirmed_at`). Truy vấn trực tiếp `disclosure_records` có hạn trong range; **không** suy từ danh sách cảnh báo.
  - **Bỏ `PENDING_CONFIRM` khỏi contract** (sửa §5.1/AC-4): submit hoặc hoàn tất workflow đã là tín hiệu hoàn thành.
  - **Giữ "Xác nhận"** cho trường hợp **đã công bố ngoài hệ thống** khi record còn Draft → `DONE`.
  - Màu tỉ lệ đúng hạn: ≥95% xanh, 85–95% vàng, <85% đỏ, không đủ mẫu thì trung tính.
  - Lý do: V1 đã khiến `PENDING_CONFIRM` không bao giờ xảy ra. Giữ nó trong contract chỉ tạo nợ. Ngưỡng 95% phù hợp kỳ vọng tuân thủ của doanh nghiệp niêm yết.
  - AC: DEV có 16 record đã submit, nên KPI hoàn thành > 0.
- **Q-20:** ✅ Xác nhận lần hai trả 200 với người xác nhận gốc (giữ tính toàn vẹn audit).
- **Q-21:** ✅ Cho phép thêm `Unknown` vào `DeadlineStatus` (`src/types.ts`) và đổi nhãn DUE_SOON thành **"Đến hạn hôm nay"**. Thay đổi nhỏ, tăng độ đúng của nhãn.

### Nhóm D: nhắc việc / thông báo
- **Q-01:** ✅ (điều chỉnh nhẹ)
  - Khung gửi **07:00–21:00 giờ VN mọi ngày** (hạn định kỳ tính theo ngày lịch nên cả cuối tuần vẫn có hạn).
  - Grace 2 ngày sau hạn bước.
  - Quá 48h thì hết hạn, không gửi.
  - Khi đã trễ, mỗi bước chỉ **một** thông báo "Đã quá hạn N ngày" rồi dừng.
  - Mở rộng P2: leo thang tới admin công ty khi bước quá hạn ≥ 1 ngày.
- **Q-02:** ✅ Phương án B (defer 15 phút + hết hạn 48h). Không làm rơi nhắc khi DB gián đoạn ngắn.
- **Q-03:** ✅ Tách PR-1a.
- **Q-11:** ✅ 400/403/404/422 → FAILED; tối đa 20 lần; backoff 1→60 phút; có metric và cảnh báo.
- **Q-22 (trạng thái company):** ✅
  - **inactive**: dừng mọi nhắc, cảnh báo và tạo kỳ mới (khớp với khoá truy cập `c43b9cc`).
  - **suspended**: **vẫn** tạo kỳ và **vẫn** nhắc, vì nghĩa vụ pháp lý không tạm ngưng theo gói dịch vụ. Email kèm một dòng "Tài khoản đang tạm ngưng – liên hệ …".
  - Lý do: im lặng với khách đang tạm ngưng tạo rủi ro tuân thủ và uy tín lớn hơn lợi ích thương mại.
  - DEV: 21/21 company active.
- **Q-23 (thiếu cấu hình kênh):** ✅ Dùng **mặc định** (email + in-app, lịch mặc định).
  - Lý do: DEV chỉ **3/21** company có row prefs. Fail-closed sẽ làm 18 company mất toàn bộ nhắc khi bật flag.
- **Q-24:** ✅ Lịch gửi của company chỉ áp cho nhắc **hạn nộp**. Nhắc **bước workflow** theo offset của template (3 ngày, 1 ngày).

### Nhóm E: vận hành / chất lượng
- **Q-25:** ✅ Làm (a) cách ly binary cũ, (b) tắt 6 template QA chu kỳ hằng ngày, (c) allowlist/redirect email trên DEV, (d) rotate `INTERNAL_REMINDER_TOKEN` **trước** PR-1a. (e) Sửa dữ liệu từng bước, có kiểm tra trước/sau.
- **Q-26:** ✅ Dựng MySQL test. Khoảng 1/3 test tái hiện cần DB thật; không có thì GATE R/V yếu.

## 3. Đề xuất sửa contract (Business Contract v1.2; PO ký)

| Mục | Sửa |
|---|---|
| §1 (mới) | Định nghĩa `ObligationOpen` và `CompletedAt` (§1 ở trên) là nguồn duy nhất cho tab Hạn, dashboard, Việc của tôi, reminder |
| §3.2 | Cập nhật state machine ad-hoc theo v3 (một vòng, nhiều reviewer, cần tất cả đồng ý); quy tắc A4 làm rõ thành "giá trị được nhập cuối cùng"; chặn Reject/Cancel khi đủ phiếu |
| §5.1 / AC-4 | Bỏ `PENDING_CONFIRM`. `DONE` = xác nhận "đã công bố ngoài hệ thống" khi record còn mở. Record đã submit/hoàn tất rời tab Hạn (xem ở lịch sử) |
| §5.2 | Ad-hoc: hiện từ khi duyệt tới khi hoàn tất workflow |
| §7 | Quy tắc reminder: khung giờ, grace, hết hạn, một thông báo quá hạn, nhắc theo bước hiện tại (không theo task) |
| KPI `06-approved-kpi-formulas` | #3 không theo range; #4/#6 dựa vào `CompletedAt`; ngưỡng màu đúng hạn |
| Hạn | Tính theo khai báo của template; ngày cụ thể là bắt buộc; validate khai báo khi publish (Q-04/Q-05) |
| AC mới | AC-11 ad-hoc duyệt xong hiện ngay trên tab Hạn. AC-12 bước 2..N được nhắc. AC-13 Việc của tôi có task định kỳ. AC-14 preview = worker = tab Hạn về ngày hạn. AC-15 không gửi email ngoài 07–21h |

## 4. Bảng chốt (để ký)

| Mã | Khuyến nghị | Thay đổi so với kế hoạch | Ký |
|---|---|---|---|
| Q-01 | 07–21h mọi ngày; grace 2 ngày; hết hạn 48h; 1 thông báo quá hạn mỗi bước | khung "mọi ngày" ghi rõ | ☐ |
| Q-02 | B | — | ☐ |
| Q-04 | Theo template + validate khai báo | — | ☐ |
| Q-06 | File chỉ chứa ngày chắc chắn; còn lại nhập qua CMS theo văn bản; quy trình tháng 10 | ⚠️ không hard-code ngày TBC | ☐ |
| Q-07 | T0 = ngày submit | — | ☐ |
| Q-08/09/10/11 | Như §2 | Q-10 thêm "ghi `planned_date` lúc tạo" | ☐ |
| Q-12 | (a′) hiện từ khi duyệt tới khi workflow hoàn tất | ⚠️ thêm điều kiện "chưa terminal" | ☐ |
| Q-13 | Giữ A4 (giá trị được nhập cuối) + sửa DUE-06 + hiện giá trị hiện hành | ⚠️ bỏ "sớm nhất" | ☐ |
| Q-14 | F2 | — | ☐ |
| Q-15 | A + B | ⚠️ thêm A | ☐ |
| Q-16 | P1 ngay sau PR-3 | ⚠️ nâng ưu tiên | ☐ |
| Q-17 | Có, P1 sau PR-4 | — | ☐ |
| Q-18 | KPI mở theo hiện tại; range cho KPI hoàn thành | — | ☐ |
| Q-19 | `CompletedAt` (gồm `submitted_at`); bỏ `PENDING_CONFIRM`; giữ confirm "công bố ngoài hệ thống"; ngưỡng 95/85 | ⚠️ khác `COALESCE(completed_at, confirmed_at)` | ☐ |
| Q-20/21/23/24 | Như §2 | — | ☐ |
| Q-22 | inactive dừng; suspended vẫn nhắc + dòng thông báo | — | ☐ |
| Q-25/26 | Làm trước PR-1a | — | ☐ |
| PO-01 | Sửa "Việc của tôi" theo `ObligationOpen` (PR-4) | ⚠️ mục mới | ☐ |

Câu hỏi còn lại cần đúng người có thẩm quyền: ngày nghỉ TBC 2026/2027 do **BA/HR** đối chiếu văn bản chính thức. Ngoài ra, các khuyến nghị trên đều có thể chốt ở cấp PO.
