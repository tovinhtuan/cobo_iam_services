# BA Decision Questionnaire — Màn «Chi tiết cảnh báo» (căn theo Mock Happy Case)

**Ngày:** 2026-05-25 (cập nhật: căn mock `DeadlineDetail.tsx`)  
**Trạng thái:** ✅ **PO đã chốt** — xem tóm tắt canonical: `deadline-alert-detail-po-decisions-summary.md`  
**Mục đích:** PO/BA chọn **một option** mỗi câu. Mock UI = **happy case** nghiệp vụ (hồ sơ đang chạy, bước 3/4, chưa Done).  
**Khuyến nghị SA/TL:** Giữ **khung trải nghiệm mock** khi có API; tách **Release 1 (dữ liệu thật, không click giả)** vs **Release 2 (persist như mock)**.

**Nguồn mock:** `cobo_web_design/src/pages/portal/DeadlineDetail.tsx` (milestone + WorkflowCard + sidebar + footer).

**Cách dùng:** Điền **Quyết định PO/BA**. Cột **★ SA/TL** = khuyến nghị khi muốn bám mock happy case.

---

## 0. Mock happy case — bức tranh nghiệp vụ (reference)

### 0.1 Kịch bản mock đang vẽ


| Yếu tố              | Giá trị mock (happy path)                                                                 |
| ------------------- | ----------------------------------------------------------------------------------------- |
| Tiến độ tổng        | **4 bước**; bước 1–2 **completed**, bước 3 **current**, bước 4 upcoming                   |
| Thanh progress      | ~**62,5%** (3/4), marker đỏ tại vị trí hiện tại                                           |
| Trạng thái cảnh báo | Chưa Done (UI cho phép «xác nhận hoàn tất»)                                               |
| Phòng               | «Phòng ban xử lý» trên từng card (mock gắn `activeDepartments`)                           |
| Ngày                | Cụ thể: 15/03, 20/03, 25/03, 30/03/2026 trên milestone & form                             |
| Tài liệu            | **Theo bước** (PDF/DOCX/XLSX) + nút «+» thêm hồ sơ                                        |
| Bước 4 đặc thù      | Email đã gửi (SGDK, UBCKNN, Thuế); **4 ô** ngày/link công bố (Website, Sở, UBCK, báo cáo) |
| Sidebar             | Ghi chú; Dịch thuật (file đã hoàn thành); nút lớn **«Đã Công bố/Báo cáo đúng hạn»**       |
| Footer              | **Hủy bỏ** · **Cập nhật thông tin** · **Xác nhận kết thúc** (đỏ)                          |
| Toggle từng card    | Switch **«Hoàn thành»** trên mỗi `WorkflowCard` (local state)                             |


### 0.2 Map mock → dữ liệu thật (engineering)


| Khối mock                                   | API / SoT đề xuất                               | Release 1 (có sẵn)   | Cần thêm (Release 2)            |
| ------------------------------------------- | ----------------------------------------------- | -------------------- | ------------------------------- |
| Header title/due/status/phòng               | `deadline-alerts` + `disclosures`               | ✅                    | —                               |
| 4 milestone + % progress                    | `workflow.instances` + `step_timelines` / steps | ⚠️ Một phần          | BE đủ ngày từng bước            |
| WorkflowCard ×4                             | `workflow[]` snapshot + `current_step_code`     | ⚠️ Tên bước/phòng    | Form field dates                |
| Toggle hoàn thành từng bước                 | —                                               | ❌                    | Sync task status hoặc API riêng |
| DocumentList theo bước                      | `workflow[].documents`                          | ⚠️ Read-only nếu có  | Upload (+)                      |
| Bước 4 email + InfoBox                      | —                                               | ❌                    | Evidence / publish channels     |
| Sidebar ghi chú                             | —                                               | ❌                    | `record_notes` hoặc tương đương |
| Sidebar dịch thuật                          | —                                               | ❌                    | Module dịch thuật               |
| «Đã Công bố đúng hạn» / «Xác nhận kết thúc» | `record.status` + alert `DONE`                  | ⚠️ Read-only reflect | API confirm completion          |
| Footer «Cập nhật thông tin»                 | —                                               | ❌                    | PATCH notes / disclosure        |


---

## A. Hoàn thành & hành động (mock có 3 điểm chạm)

### OQ-DETAIL-01 — «Đánh dấu hoàn thành» — khớp mock happy case


| Option   | Mô tả                                                                                                                                                                  | Khớp mock                          |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| **HC-1** | **Giữ đủ UX mock:** toggle từng bước + sidebar «Đã Công bố/Báo cáo đúng hạn» + footer «Xác nhận kết thúc» — mọi thao tác **ghi DB** (publish / complete / alert DONE). | ★★★                                |
| **HC-2** | **Giữ layout & nút như mock**, nhưng toggle/nút **không đổi state local** — chỉ **phản ánh** workflow + record; khi chưa đủ điều kiện → disabled + hướng dẫn mở hồ sơ. | ★★☆ (hiển thị giống, hành vi thật) |
| **HC-3** | Chỉ giữ **sidebar + footer «Xác nhận kết thúc»**; **bỏ** toggle trên từng card.                                                                                        | ★★☆                                |
| **A**    | Lens mỏng: **bỏ** sidebar xác nhận & footer đỏ; Done read-only + link hồ sơ.                                                                                           | ☆☆☆ (lệch mock)                    |
| **B**    | Thêm **workflow task actions** trên màn (không có trong mock).                                                                                                         | ☆☆☆                                |


**★ Khuyến nghị SA/TL (bám mock):** **HC-2** (Release 1) → **HC-1** (Release 2 sau khi BA chốt API).

**Lý do:** Mock thể hiện **cockpit điều phối deadline**, không phải màn xem passively. **HC-2** giữ PO/demo đúng layout happy case mà không lừa QA bằng `setState`. **HC-1** là target khi có contract `confirm` gắn `published` + `DONE`.

**Quyết định PO/BA:** _______________ **HC-1**

---

### OQ-DETAIL-02 — Footer sticky (mock: 3 nút)


| Option   | Mô tả                                                                                                                                  | Khớp mock |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| **HC-1** | Giữ **đủ 3 nút** như mock; wire: Hủy → quay list; Cập nhật → API lưu (ghi chú/cập nhật bước); Xác nhận kết thúc → theo OQ-01 **HC-1**. | ★★★       |
| **HC-2** | Giữ **3 nút** UI; Cập nhật/Xác nhận **disabled** hoặc redirect hồ sơ cho đến Release 2 API.                                            | ★★★       |
| **HC-3** | Chỉ **Hủy bỏ** + **Xác nhận kết thúc** (bỏ Cập nhật).                                                                                  | ★★☆       |
| **A**    | Một CTA «Tiếp tục trên hồ sơ CBTT»; **bỏ** footer 3 nút.                                                                               | ☆☆☆       |


**★ Khuyến nghị SA/TL:** **HC-2** (khớp mock + an toàn); nếu PO chọn **HC-1** ở OQ-01 thì nâng footer lên **HC-1**.

**Quyết định PO/BA:** _______________ **HC-1**  
**Phụ thuộc:** OQ-DETAIL-01.

---

### OQ-DETAIL-03 — Toggle «Hoàn thành» trên từng WorkflowCard (mock có)


| Option   | Mô tả                                                                                                      |
| -------- | ---------------------------------------------------------------------------------------------------------- |
| **HC-1** | **Giữ toggle**; đổi trạng thái **persist** (map task/step workflow).                                       |
| **HC-2** | **Giữ toggle** UI nhưng **read-only** — vị trí bật/tắt derive từ task/status bước (không `onClick` local). |
| **HC-3** | **Bỏ toggle**; chỉ màu card (completed/current/upcoming) từ backend.                                       |
| **A**    | Không áp dụng (đã bỏ WorkflowCard).                                                                        |


**★ Khuyến nghị SA/TL:** **HC-2** (Release 1) — happy case nhìn vẫn giống mock (bước 1–2 xanh, 3 active). **HC-1** khi có API.

**Quyết định PO/BA:** _______________ **HC-1**

---

### OQ-DETAIL-04 — Sidebar «Đã Công bố/Báo cáo đúng hạn» (mock)


| Option   | Mô tả                                                                                                                      |
| -------- | -------------------------------------------------------------------------------------------------------------------------- |
| **HC-1** | Giữ widget; click → **API** ghi nhận hoàn tất (→ `published`/`completed`, alert `DONE`).                                   |
| **HC-2** | Giữ widget; **chỉ hiển thị trạng thái** (Done = filled xanh, chưa = dashed + «Chờ hệ thống/hồ sơ»); không click đổi local. |
| **HC-3** | Bỏ widget; chỉ badge header.                                                                                               |
| **A**    | Click vẫn `setStatus('Done')` local như mock cũ (**không** khuyến nghị).                                                   |


**★ Khuyến nghị SA/TL:** **HC-2**. Mock happy case **sau khi** xác nhận hiển thị «Hệ thống ghi nhận hoàn tất vào 28/03/2026» → đó là **trạng thái Done**, không phải nút giả.

**Quyết định PO/BA:** _______________ **HC-1**

---

## B. Layout & nội dung (mock = cockpit 4 bước)

### OQ-DETAIL-05 — Phạm vi màn — so với mock


| Option      | Mô tả                                                                                                           | Khớp mock |
| ----------- | --------------------------------------------------------------------------------------------------------------- | --------- |
| **HC**      | **Cockpit đầy đủ (mock):** timeline + 4 WorkflowCard + sidebar 3 khối + footer. Không full prose nội dung CBTT. | ★★★       |
| **HC-LITE** | Cockpit nhưng **ẩn** Dịch thuật + Ghi chú (placeholder) đến khi có API.                                         | ★★☆       |
| **A**       | Lens mỏng: timeline + metadata + link hồ sơ (**bỏ** cards & sidebar).                                           | ☆☆☆       |
| **D**       | Trùng `DisclosureDetail`.                                                                                       | ☆☆☆       |


**★ Khuyến nghị SA/TL:** **HC** (product target) hoặc **HC-LITE** nếu cần cắt scope Q2 — **không** **A** nếu PO đã sign-off mock.

**Quyết định PO/BA:** _______________ **HC-1**

---

### OQ-DETAIL-06 — Timeline & ngày trên milestone (mock có ngày cụ thể)


| Option   | Mô tả                                                                                                                                                                    | Khớp mock |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------- |
| **HC-1** | Thanh **% progress** + 4 ô milestone **có ngày** như mock; ngày từ `step_timelines` BE; thiếu → hiển thị «Dự kiến: {T+N}» từ snapshot (**không** tính calendar trên FE). | ★★★       |
| **HC-2** | Giữ **layout** milestone; ngày để `—` cho đến khi BE có timeline.                                                                                                        | ★★☆       |
| **A**    | Chỉ tên bước + phòng, không thanh % (lệch mock).                                                                                                                         | ☆☆☆       |
| **D**    | Gate release đến khi BE đủ ngày.                                                                                                                                         | ★☆☆ (trì) |


**★ Khuyến nghị SA/TL:** **HC-1** — mock happy case **yêu cầu** cảm giác «đang ở bước 3, ngày 25/03». Ưu tiên ticket BE `step_timelines`; FE fallback label.

**Quyết định PO/BA:** _______________ **HC-1**

---

### OQ-DETAIL-07 — «Danh mục hồ sơ cần thiết» trên từng bước (mock)


| Option   | Mô tả                                                                                                                                           | Khớp mock   |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| **HC-1** | **Giữ** `DocumentList` mỗi card; data từ `workflow[].documents` + `record.attachments` ở bước công bố; nút «+» → mở hồ sơ/upload ở route hồ sơ. | ★★★         |
| **HC-2** | Giữ list **read-only**; ẩn nút «+» đến Release 2.                                                                                               | ★★☆         |
| **A**    | Không hiển thị tài liệu trên màn cảnh báo.                                                                                                      | ☆☆☆         |
| **D**    | Upload trực tiếp trên màn cảnh báo.                                                                                                             | ★★☆ (scope) |


**★ Khuyến nghị SA/TL:** **HC-2** (Release 1) → **HC-1** khi upload flow rõ.

**Quyết định PO/BA:** _______________ **HC-1**

---

### OQ-DETAIL-08 — Header Settings / ExternalLink (mock có cả hai)


| Option   | Mô tả                                                                                                     |
| -------- | --------------------------------------------------------------------------------------------------------- |
| **HC-1** | **ExternalLink** → `/app/history/{id}` tab mới; **Settings** → reminder config record (§3.10) nếu có API. |
| **HC-2** | Chỉ **ExternalLink** (trùng banner); bỏ Settings.                                                         |
| **HC-3** | Giữ cả hai như mock, disabled + tooltip «Sắp có».                                                         |
| **A**    | Bỏ hẳn.                                                                                                   |


**★ Khuyến nghị SA/TL:** **HC-2** hoặc **HC-1** nếu reminder API đã có trên disclosure.

**Quyết định PO/BA:** _______________  **HC-1**

---

## B2. Khối mock chưa có trong contract — PO quyết scope

### OQ-DETAIL-09 — Sidebar «Các vấn đề cần lưu ý» + textarea


| Option   | Mô tả                                                            |
| -------- | ---------------------------------------------------------------- |
| **HC-1** | Giữ; **lưu** ghi chú theo `record_id` (API mới).                 |
| **HC-2** | Giữ UI; placeholder + «Sắp có» / read-only empty state như mock. |
| **A**    | Bỏ hẳn khối sidebar này.                                         |


**★ Khuyến nghị SA/TL:** **HC-2** (Release 1) — mock happy case có ô nhưng text «Chưa có ghi chú…» = acceptable empty.

**Quyết định PO/BA:** _______________ **HC-1**

---

### OQ-DETAIL-10 — Sidebar «Dịch thuật văn bản»


| Option   | Mô tả                                               |
| -------- | --------------------------------------------------- |
| **HC-1** | Giữ; nối module dịch thuật thật.                    |
| **HC-2** | Giữ **card mẫu** read-only / link sang module khác. |
| **A**    | Bỏ (HC-LITE).                                       |


**★ Khuyến nghị SA/TL:** **HC-2** nếu giữ **HC** layout; **A** nếu chọn **HC-LITE**.

**Quyết định PO/BA:** _______________ A

---

### OQ-DETAIL-11 — Bước 4: email đã gửi + InfoBox kênh công bố (mock)


| Option   | Mô tả                                                                                                           |
| -------- | --------------------------------------------------------------------------------------------------------------- |
| **HC-1** | **Giữ đủ** checklist email + 4 InfoBox; data từ `evidenceLink`, publish dates, checklist công bố (API mở rộng). |
| **HC-2** | Giữ **layout**; checklist **read-only** từ record đã published; chưa publish → empty + «Chưa công bố».          |
| **HC-3** | Thu gọn bước 4 như các bước 1–3 (không email/InfoBox).                                                          |
| **A**    | Ẩn nội dung đặc thù bước 4.                                                                                     |


**★ Khuyến nghị SA/TL:** **HC-2** — mock happy case minh họa **giai đoạn sắp công bố**; Release 1 bind `publishedDate` + `evidenceLink` nếu có, phần còn placeholder có nhãn.

**Quyết định PO/BA:** _______________ A 

---

### OQ-DETAIL-12 — Form field trên card (Ngày khởi tạo, Thời hạn còn lại, …)


| Option   | Mô tả                                                                                                      |
| -------- | ---------------------------------------------------------------------------------------------------------- |
| **HC-1** | Giữ **grid field** như mock; giá trị từ BE (`step_timelines`, `deadline_at`, `remaining_days` aggregated). |
| **HC-2** | Giữ nhãn; giá trị `—` hoặc chỉ field có data.                                                              |
| **A**    | Chỉ tiêu đề bước + phòng + documents (bỏ grid).                                                            |


**★ Khuyến nghị SA/TL:** **HC-2** Release 1 → **HC-1** khi BE trả đủ field; mock happy case **có** «15 ngày» highlight đỏ = cần `remaining_days` hoặc tương đương từ alert API (có thể ticket BE nhỏ).

**Quyết định PO/BA:** _______________  **HC-1**

---

## C. Trạng thái & phạm vi dữ liệu (list + detail)

### OQ-DA-01 — Alert **Done** ↔ record (mock Done = «Đã công bố đúng hạn»)


| Option | Mô tả                                                                                                  |
| ------ | ------------------------------------------------------------------------------------------------------ |
| **HC** | Done khi **đã công bố đúng hạn** nghiệp vụ: `published` **hoặc** `completed` (khớp copy mock sidebar). |
| **A**  | Chỉ `published`.                                                                                       |
| **C**  | Mọi terminal status (BA liệt kê).                                                                      |


**★ Khuyến nghị SA/TL:** **HC** (= **B** trước đây).

**Quyết định PO/BA:** _______________ **HC**

---

### OQ-DA-02 — Record `draft` trên tab cảnh báo


| Option | Mô tả           |
| ------ | --------------- |
| **A**  | **Không** (P2). |
| **B**  | Có badge Nháp.  |


**★ Khuyến nghị SA/TL:** **A** — mock happy case là hồ sơ **đang chạy**, không phải draft.

**Quyết định PO/BA:** _______________ A

---

### OQ-DA-03 — Định kỳ không `workflow_instance_id`


| Option | Mô tả                                                                                                    |
| ------ | -------------------------------------------------------------------------------------------------------- |
| **HC** | Vẫn vào detail; timeline **degraded** (1 bước ảo «Chờ khởi tạo quy trình») + banner cảnh báo; phòng `—`. |
| **A**  | Message «Chưa gán quy trình», ẩn cards.                                                                  |
| **C**  | Không list đến khi có workflow.                                                                          |


**★ Khuyến nghị SA/TL:** **HC** — tránh màn trống; khác mock nhưng honest.

**Quyết định PO/BA:** _______________ HC

---

### OQ-DA-04 — Multi-phòng (H2)


| Option  | Mô tả                                                                            |
| ------- | -------------------------------------------------------------------------------- |
| **A**   | Một phòng / bước active (P3). Mock cũng hiển thị **một** pill phòng trên header. |
| **B/C** | Nhiều phòng / nhiều bước active.                                                 |


**★ Khuyến nghị SA/TL:** **A** — khớp mock happy case.

**Quyết định PO/BA:** _______________ A

---

## D. Kiến trúc (vẫn áp dụng khi bind mock)

### OQ-DA-05 (H3) — Ngày trên UI mock


| Option | Mô tả                                                                                                                                               |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **HC** | **Đồng ý H3** + cho phép hiển thị **ngày calendar** chỉ khi BE trả (`step_timelines` / `deadline_at` / field aggregated) — **không** FE tự cộng T0. |
| **B**  | FE tự tính ngày từ T0 (không khuyến nghị).                                                                                                          |


**★ Khuyến nghị SA/TL:** **HC**.

**Quyết định PO/BA:** _______________ **HC**

---

### OQ-DA-06 (H4) — CTA «Tạo cảnh báo mới»


| Option  | Mô tả                                                                      |
| ------- | -------------------------------------------------------------------------- |
| **A**   | Giữ header **list**; **không** trên detail (mock detail không có CTA này). |
| **B/C** | Khác.                                                                      |


**★ Khuyến nghị SA/TL:** **A** — khớp mock.

**Quyết định PO/BA:** _______________ B/C đổi thành "Tạo cảnh báo bất thường" và đi theo flow "Tạo cảnh báo bất thường" đang có

---

## E. Bộ khuyến nghị SA/TL — bám mock happy case

### E.1 Release 1 (dữ liệu thật, layout mock)


| ID           | ★ Chọn đề xuất                    |
| ------------ | --------------------------------- |
| OQ-DETAIL-01 | **HC-2**                          |
| OQ-DETAIL-02 | **HC-2**                          |
| OQ-DETAIL-03 | **HC-2**                          |
| OQ-DETAIL-04 | **HC-2**                          |
| OQ-DETAIL-05 | **HC** hoặc **HC-LITE**           |
| OQ-DETAIL-06 | **HC-1**                          |
| OQ-DETAIL-07 | **HC-2**                          |
| OQ-DETAIL-08 | **HC-2**                          |
| OQ-DETAIL-09 | **HC-2**                          |
| OQ-DETAIL-10 | **HC-2** (hoặc **A** nếu HC-LITE) |
| OQ-DETAIL-11 | **HC-2**                          |
| OQ-DETAIL-12 | **HC-2**                          |
| OQ-DA-01     | **HC**                            |
| OQ-DA-02     | **A**                             |
| OQ-DA-03     | **HC**                            |
| OQ-DA-04     | **A**                             |
| OQ-DA-05     | **HC**                            |
| OQ-DA-06     | **A**                             |


### E.2 Release 2 (đúng hành vi click mock — cần BA + API)


| ID                 | Target                       |
| ------------------ | ---------------------------- |
| OQ-DETAIL-01       | **HC-1**                     |
| OQ-DETAIL-02       | **HC-1**                     |
| OQ-DETAIL-03       | **HC-1**                     |
| OQ-DETAIL-04       | **HC-1**                     |
| OQ-DETAIL-07       | **HC-1** (+ upload)          |
| OQ-DETAIL-09/10/11 | **HC-1** khi module sẵn sàng |


**Không khuyến nghị:** giữ **A** (lens mỏng) hoặc **click local** như mock cũ — lệch happy case **về mặt tin cậy dữ liệu**.

---

## G. PO đã chốt (2026-05-25)

### G.1 Quy tắc «hoàn tất» (bổ sung — canonical)

**Hoàn tất trên màn cảnh báo = hoàn tất workflow + công bố hồ sơ (có bằng chứng).**

Áp dụng OQ-DETAIL-01, 02, 03, 04 (HC-1). Chi tiết triển khai: `deadline-alert-detail-po-decisions-summary.md` §0.

| ID | Quyết định |
|----|------------|
| OQ-DETAIL-01 | HC-1 *(theo §G.1)* |
| OQ-DETAIL-02 | HC-1 |
| OQ-DETAIL-03 | HC-1 |
| OQ-DETAIL-04 | HC-1 |
| OQ-DETAIL-05 | HC *(PO ghi HC-1)* |
| OQ-DETAIL-06 | HC-1 |
| OQ-DETAIL-07 | HC-1 |
| OQ-DETAIL-08 | HC-1 |
| OQ-DETAIL-09 | HC-1 |
| OQ-DETAIL-10 | **A** (bỏ dịch thuật) |
| OQ-DETAIL-11 | **A** (ẩn email/InfoBox bước 4) |
| OQ-DETAIL-12 | HC-1 |
| OQ-DA-01 | HC |
| OQ-DA-02 | A |
| OQ-DA-03 | HC |
| OQ-DA-04 | A |
| OQ-DA-05 | HC |
| OQ-DA-06 | **Custom:** «Tạo cảnh báo bất thường» → flow ad-hoc (`/app/ad-hoc-proposals/new`) |

**Engineering:** PO chọn **HC-1 persist** → ưu tiên **API contract** (§3 trong `deadline-alert-detail-po-decisions-summary.md`) trước khi wire footer/toggle/sidebar.

---

## F. Sau khi PO/BA điền

1. ~~Ghi bảng quyết định~~ → Done (§G).
2. Cập nhật `deadline-alert-detail-screen-implementation-plan.md` §5 + tickets — **đã sync** qua `deadline-alert-detail-po-decisions-summary.md`.
3. **HC-1** đã chốt → mở **API contract** trước code.

---

**Docs consulted:** `DeadlineDetail.tsx`, `mockData.ts` (`mockDeadlines`), `deadline-alert-detail-screen-implementation-plan.md`.

**Cache:** `docs/ai-cache/deadline-alert-detail-ba-decision-questionnaire.md`