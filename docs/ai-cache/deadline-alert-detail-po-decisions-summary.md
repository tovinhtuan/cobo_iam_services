# PO Decisions — Màn «Chi tiết cảnh báo» (đã chốt)

**Ngày chốt:** 2026-05-25  
**Nguồn:** `deadline-alert-detail-ba-decision-questionnaire.md` (cột Quyết định PO/BA)

---

## 0. Quy tắc nghiệp vụ PO chốt bổ sung (canonical — 2026-05-25)

> **«Hoàn tất trên màn cảnh báo = hoàn tất workflow + công bố hồ sơ (có bằng chứng)»**

| Không phải | Là |
|------------|-----|
| Nút đổi `alert.status` / `setState('Done')` trên FE | Chuỗi nghiệp vụ trên **`disclosure_records`** + **`workflow_instances`** |
| `POST /deadline-alerts/.../complete` | **Publish** hồ sơ (`published`) với **`evidence_link` bắt buộc** (§3.8) |
| Toggle bước độc lập | Tiến bước qua **`actOnTask`** (review / approve / confirm / reject) §3.9 |

**Thứ tự logic (happy path):**

1. Các task workflow bắt buộc đã xử lý xong (instance không còn bước chặn publish).
2. User **công bố hồ sơ** — nhập/link bằng chứng SSC/HNX → `status = published` (permission `disclosure.publish`).
3. Hệ thống derive cảnh báo **`DONE`** (OQ-DA-01: `published` hoặc `completed`).

**UI màn cảnh báo (map HC-1 đã chốt):**

| Control mock | Hành vi sau quy tắc PO |
|--------------|-------------------------|
| Toggle từng card | Reflect + trigger **workflow task** (không PATCH step giả) |
| «Cập nhật thông tin» | Lưu phụ trợ (ghi chú, field hồ sơ) — **không** thay thế publish |
| «Xác nhận kết thúc» / sidebar «Đã Công bố đúng hạn» | Mở **publish flow** (modal evidence hoặc chuyển `/app/history/{id}` / edit) — chỉ enable khi đủ điều kiện workflow |
| Badge Done | Read-only sau bước 2–3 thành công |

---

## 1. Bảng quyết định (canonical)

| ID | PO chọn | Ghi chú triển khai |
|----|---------|-------------------|
| **OQ-DETAIL-01** | **HC-1** | §0: workflow xong + **publish có evidence** → alert `DONE`. Không API alert-complete. |
| **OQ-DETAIL-02** | **HC-1** | Footer: Hủy / Cập nhật (phụ trợ) / Xác nhận kết thúc = **publish flow** §0. |
| **OQ-DETAIL-03** | **HC-1** | Toggle card → **`actOnTask`** (§3.9), UI reflect task state. |
| **OQ-DETAIL-04** | **HC-1** | Sidebar «Đã Công bố đúng hạn» = **publish + evidence** (cùng §0), không click Done local. |
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
- **PO đã chốt §0** — alignment assessment **đóng** phần «hoàn tất»; còn FE-DA-D19 gate `irregular` + permission ad-hoc.

## 3. API / BE blockers (bắt buộc trước wire HC-1)

| Capability | Đề xuất endpoint / hành vi | Ticket gợi ý |
|------------|---------------------------|--------------|
| Hoàn tất (§0) | Disclosure **update/publish** + `evidence_link`; list `deadline-alerts` → `DONE` | BE-DA-D10 = verify map; FE publish modal/redirect |
| Bước workflow (§0) | **`workflowInstancesApi.actOnTask`** only | FE-DA-D15 / D11 |
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

**Dev-ready (step-by-step):** `deadline-alert-detail-phase5-execution-plan.md`

1. STEP 0–1: chuẩn bị + D19/D20 (ad-hoc CTA, fix submit path).  
2. STEP 2–3: foundation + UI cockpit read-only.  
3. STEP 4–5: workflow tasks + publish modal (§0).  
4. STEP 6–8: documents, BE optional, QA.

---

**Docs consulted:** `deadline-alert-detail-ba-decision-questionnaire.md`, `deadline-alert-detail-screen-implementation-plan.md`.

**Cache:** `docs/ai-cache/deadline-alert-detail-po-decisions-summary.md`
