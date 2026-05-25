# PO Decisions — Màn «Chi tiết cảnh báo» (đã chốt)

**Ngày chốt:** 2026-05-25  
**Nguồn:** `deadline-alert-detail-ba-decision-questionnaire.md` (cột Quyết định PO/BA)

---

## 1. Bảng quyết định (canonical)

| ID | PO chọn | Ghi chú triển khai |
|----|---------|-------------------|
| **OQ-DETAIL-01** | **HC-1** | Toggle bước + sidebar + footer **persist DB** (publish/complete/DONE). Cần **API contract** trước FE. |
| **OQ-DETAIL-02** | **HC-1** | Footer 3 nút wired: Hủy / Cập nhật / Xác nhận kết thúc. |
| **OQ-DETAIL-03** | **HC-1** | Toggle «Hoàn thành» trên WorkflowCard → map workflow task/step. |
| **OQ-DETAIL-04** | **HC-1** | Widget «Đã Công bố/Báo cáo đúng hạn» → API ghi nhận hoàn tất. |
| **OQ-DETAIL-05** | **HC** *(PO ghi HC-1)* | Cockpit đầy đủ: timeline + 4 card + sidebar + footer (không lens mỏng). |
| **OQ-DETAIL-06** | **HC-1** | Progress % + milestone **có ngày**; từ `step_timelines` BE, fallback label T+N. |
| **OQ-DETAIL-07** | **HC-1** | DocumentList theo bước + nút «+» → route upload hồ sơ. |
| **OQ-DETAIL-08** | **HC-1** | ExternalLink → history; Settings → reminder config record. |
| **OQ-DETAIL-09** | **HC-1** | Sidebar ghi chú **lưu** theo `record_id` (API mới). |
| **OQ-DETAIL-10** | **A** | **Bỏ** khối «Dịch thuật văn bản». |
| **OQ-DETAIL-11** | **A** | **Ẩn** email checklist + InfoBox kênh công bố ở bước 4 (không HC-1/HC-2). |
| **OQ-DETAIL-12** | **HC-1** | Grid form field trên card; giá trị từ BE aggregated. |
| **OQ-DA-01** | **HC** | Done = `published` **hoặc** `completed`. |
| **OQ-DA-02** | **A** | Không list `draft` trên tab cảnh báo (P2). |
| **OQ-DA-03** | **HC** | Detail degraded khi thiếu workflow instance. |
| **OQ-DA-04** | **A** | Một phòng active (P3). |
| **OQ-DA-05** | **HC** | Ngày calendar chỉ từ BE; không derive T0 trên FE. |
| **OQ-DA-06** | **Custom** | Đổi label header list: **«Tạo cảnh báo bất thường»** → flow ad-hoc hiện có (`/app/ad-hoc-proposals/new`), không `/app/disclosures/new`. |

---

## 2. So với khuyến nghị SA/TL

PO chọn **HC-1 / persist đầy đủ** cho hầu hết câu hành động (01–04, 07–09, 12) — tương đương **Release 2 ngay**, không qua HC-2 read-only trước.

**Cắt scope so với mock happy case:**

- Không sidebar dịch thuật (10 = A).
- Không UI đặc thù bước 4 email/InfoBox (11 = A).

**Lệch nhãn:** OQ-DETAIL-05 PO ghi «HC-1» — trong questionnaire chỉ có **HC** / HC-LITE; hiểu là **cockpit đầy đủ (HC)**.

---

## 2.1 Bám business contract (đánh giá 2026-05-25)

**Chi tiết:** `deadline-alert-detail-contract-alignment-assessment.md`

- **Đã bám:** P1–P4, §3.6 list/detail, ad-hoc entry (DA-06), Done/draft/phòng/H3, cockpit từ `workflow[]`.
- **Cần chỉnh trước code HC-1:** Toggle bước (03) → workflow **task actions**, không PATCH/toggle giả; «hoàn tất» (01/04) → **publish + evidence** §3.8, không endpoint alert riêng; FE-DA-D19 thêm gate `irregular` + permission ad-hoc.

## 3. API / BE blockers (bắt buộc trước wire HC-1)

| Capability | Đề xuất endpoint / hành vi | Ticket gợi ý |
|------------|---------------------------|--------------|
| Xác nhận hoàn tất cảnh báo | **Reuse** disclosure publish (evidence bắt buộc) + derive alert `DONE` — **không** invent `POST .../deadline-alerts/complete` | BE-DA-D10 |
| Cập nhật trạng thái bước / toggle | **`workflowInstancesApi.actOnTask`** (review/approve/confirm/reject) — toggle UI = reflect task state | BE-DA-D11 / FE |
| Ghi chú record | `PUT/PATCH .../disclosures/{id}/notes` hoặc field trên record | BE-DA-D12 |
| `remaining_days` / field card | Mở rộng `deadline-alerts` item hoặc workflow instance DTO | BE-DA-D13 |
| `step_timelines` đủ ngày | Workflow instance response | BE-DA-D02 (đã có trong plan) |
| Reminder từ Settings | Reuse disclosure reminder API nếu có | FE-DA-D14 |

**Nguyên tắc:** Không giữ `setState` local cho Done / toggle / footer (cấm theo quyết định HC-1).

---

## 4. FE tickets (Phase 5 — sau PO chốt)

| Ticket | Mô tả | Phụ thuộc |
|--------|--------|-----------|
| FE-DA-D01–D04 | Foundation: VM, workflow DTO, load, navigation state | — |
| FE-DA-D05–D08 | UI cockpit HC + bind workflow steps | BE timelines |
| FE-DA-D15 | Wire HC-1: footer, sidebar confirm, toggles | BE-DA-D10, D11 |
| FE-DA-D16 | Notes sidebar persist | BE-DA-D12 |
| FE-DA-D17 | DocumentList HC-1 + link upload | disclosure routes |
| FE-DA-D18 | Bỏ translation widget; simplify bước 4 (11=A) | — |
| FE-DA-D19 | List header: «Tạo cảnh báo bất thường» → `/app/ad-hoc-proposals/new` | OQ-DA-06 |

---

## 5. Thứ tự triển khai đề xuất

1. **BE contract** (§3) — review PO sign-off trên JSON + permission.  
2. **FE foundation** (D01–D04) + timeline/cards với dữ liệu thật read-only tạm nếu API chưa xong.  
3. **Wire mutations** (D15–D17) khi BE sẵn.  
4. **OQ-DA-06** label + route (có thể ship độc lập).

---

**Docs consulted:** `deadline-alert-detail-ba-decision-questionnaire.md`, `deadline-alert-detail-screen-implementation-plan.md`.

**Cache:** `docs/ai-cache/deadline-alert-detail-po-decisions-summary.md`
