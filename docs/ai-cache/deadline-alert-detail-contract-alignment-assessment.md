# Đánh giá bám contract — PO Detail Decisions vs Template / Alert / Deadline

**Ngày:** 2026-05-25  
**Đối chiếu:** PO chốt (`deadline-alert-detail-po-decisions-summary.md`) với contract đã chốt (P1–P4, §3.6–3.9, ad-hoc, template workflow).

---

## Kết luận ngắn

| Mức | Ý nghĩa |
|-----|---------|
| **~70% bám** | Identity alert, list/detail SoT, template category tạo bất thường, Done/draft/phòng/H3, cockpit workflow từ snapshot. |
| **~20% cần làm rõ** | «Đánh dấu hoàn thành» HC-1 phải map **lifecycle hồ sơ + workflow task**, không API alert riêng. |
| **~10% lệch / cắt** | Toggle bước kiểu mock; ẩn bước 4 công bố; API ghi chú chưa có trong contract gốc. |

**Verdict (cập nhật sau PO chốt §0):** Plan Phase 5 + PO decisions **bám contract** khi implement theo quy tắc: *«Hoàn tất trên màn cảnh báo = hoàn tất workflow + công bố hồ sơ (có bằng chứng)»* (`deadline-alert-detail-po-decisions-summary.md` §0).

---

## 1. Template & nguồn cảnh báo (§1.1 plan, ad-hoc contract §2)

| Contract đã chốt | PO / plan detail | Khớp? |
|------------------|------------------|-------|
| `periodic` → hệ thống push (worker) | Detail hiển thị record materialized; OQ-DA-03 degraded nếu chưa workflow | ✅ |
| `irregular` → DN **tạo thủ công** qua đề xuất ad-hoc | **OQ-DA-06:** «Tạo cảnh báo bất thường» → `/app/ad-hoc-proposals/new` | ✅ **Cải thiện** so với plan cũ (link `/disclosures/new`) |
| `custom` → push theo tần suất | Cùng nguồn `deadline-alerts` + record (không tách UI riêng) | ✅ |
| Chỉ `templateCategory === irregular` cho tạo thủ công | FE-DA-D19 cần thêm **gate** `irregular` + permission `ad_hoc_alert.propose` (contract §2.2) | ⚠️ Plan chưa ghi gate |

---

## 2. Tab «Cảnh báo thời hạn» & list (§3.6, P2–P4, tab plan)

| Contract | PO / implementation | Khớp? |
|----------|---------------------|-------|
| List `Upcoming` / `Due Soon` / `Overdue` / `Done` | OQ-DA-01 **HC** = published \| completed → DONE | ✅ |
| P2: không list `draft` | OQ-DA-02 **A** | ✅ |
| P4: detail theo `record_id` | Route + load disclosure + alert | ✅ |
| Card: title, due, **phòng đang thực hiện** (không owner) | P3 + `active_departments` API | ✅ |
| §3.6: chi tiết + đánh dấu hoàn thành | PO **HC-1** — có ý định đúng §3.6 | ⚠️ Cách implement phải theo §3.8/3.9 (xem §4) |
| Tab **không** list proposal pending | Không thay đổi; entry tạo mới → ad-hoc | ✅ |
| H4: giữ CTA header list; bỏ per-item | PO đổi label ad-hoc — vẫn **một CTA header** | ✅ |

---

## 3. Hồ sơ CBTT lifecycle (§3.8)

| Contract | PO detail | Khớp? |
|----------|-----------|-------|
| `draft → submitted → confirmed → published → completed` | Done khi published/completed; không draft trên tab | ✅ |
| **`published` bắt buộc evidence link** (SSC/HNX) | HC-1 sidebar/footer «hoàn tất» — **phải** gọi flow publish có validate `evidenceLink`, không shortcut | ❌ nếu BE-DA-D10 là POST «complete» không qua publish |
| Chỉ `draft` được xóa | Không liên quan detail | ✅ |
| Attachments trên hồ sơ | OQ-DETAIL-07 HC-1: doc theo bước + link upload hồ sơ | ✅ (qua record, không invent upload trên alert) |

---

## 4. Workflow (§3.9, portal-template §6.2, H3)

| Contract | PO detail | Khớp? |
|----------|-----------|-------|
| Workflow instance + bước phê duyệt (review/approve/confirm/reject) | OQ-DETAIL-03 **HC-1: toggle** hoàn thành từng card | ❌ **Drift:** contract không có «toggle bước»; có **task actions** |
| Bước: `stage_name`, `department`, `step_deadline` (T+N) | OQ-DETAIL-06 HC-1: timeline + ngày từ BE / label T+N; OQ-DA-05 HC | ✅ |
| FE không tự tính T0 (H3) | OQ-DA-05 **HC** | ✅ |
| Một `current_step_code` → một phòng (P3) | OQ-DA-04 **A** | ✅ |
| Số bước = **snapshot template**, không cố định 4 | Cockpit HC: bind `workflow[]` length, không hardcode mock 4 | ⚠️ Chỉ đúng nếu FE bỏ `milestones[]` cứng |

**Khuyến nghị chỉnh PO HC-1 (03, 11):**

- **03:** Toggle → **reflect** task status; click mở action **review/approve/confirm** (subset `DisclosureDetail`) hoặc deep-link bước trên hồ sơ.
- **11 = A:** Ẩn email/InfoBox OK trên **màn cảnh báo** nếu bước publish/evidence chỉ trên hồ sơ §3.8 — ghi rõ trong UX copy.

---

## 5. Ad-hoc vs deadlines (audit, one-pager)

| Contract | PO detail | Khớp? |
|----------|-----------|-------|
| Ad-hoc scope = **proposal CRUD**, không thay tab deadlines | Detail không CRUD proposal; list CTA → ad-hoc new | ✅ |
| Sau approve → record + workflow → xuất hiện trên deadlines | Cùng `record_id` / `GET deadline-alerts` | ✅ |
| Người kiểm soát quy trình (không admin bất kỳ) | Không đổi trên detail | ✅ |

---

## 6. Các điểm PO thêm — ngoài contract gốc (additive)

| PO quyết định | Đánh giá |
|---------------|----------|
| OQ-DETAIL-09 HC-1 — ghi chú sidebar | **Additive** — không mâu thuẫn; cần ADR/API mới |
| OQ-DETAIL-10 **A** — bỏ dịch thuật | **OK** — không có trong §3.6 |
| OQ-DETAIL-08 HC-1 — Settings → reminder | **OK** — §3.10 reminder trên record |
| Cockpit UI mock | **OK** nếu data từ API; mock 4 bước cố định là **anti-pattern** |

---

## 7. Hành động để «bám sát 100%» trước code HC-1

1. **Sửa spec BE-DA-D10:** Không `POST .../deadline-alerts/complete`. Dùng:
   - `workflowInstancesApi.actOnTask` cho tiến bước;
   - disclosure **publish** (evidence bắt buộc) → alert `DONE` derive (OQ-DA-01).
2. **Sửa FE-DA-D15/D03:** Toggle = trạng thái task, không PATCH step giả.
3. **FE-DA-D19:** Permission + chỉ hiện CTA khi user có quyền ad-hoc; copy contract «chỉ bất thường».
4. **Cập nhật** `deadline-alerts-real-data-implementation-plan.md` §1.4: CTA header = «Tạo cảnh báo bất thường» → ad-hoc (theo PO).
5. ~~PO xác nhận~~ **Đã chốt (§0 po-decisions-summary):** Hoàn tất trên màn cảnh báo = hoàn tất workflow + công bố hồ sơ (có bằng chứng).

---

## 8. Ma trận OQ → contract

| OQ | Bám contract? | Ghi chú |
|----|---------------|---------|
| DETAIL-01 HC-1 | ⚠️ | Đúng §3.6 nếu map publish/workflow |
| DETAIL-02 HC-1 | ⚠️ | «Cập nhật» = notes/evidence trên record, không toast giả |
| DETAIL-03 HC-1 | ❌ | Đổi thành task/workflow semantics |
| DETAIL-04 HC-1 | ⚠️ | = publish + Done badge, không entity alert riêng |
| DETAIL-05 HC | ✅ | |
| DETAIL-06 HC-1 | ✅ | |
| DETAIL-07 HC-1 | ✅ | |
| DETAIL-08 HC-1 | ✅ | |
| DETAIL-09 HC-1 | ➕ additive | |
| DETAIL-10 A | ✅ | |
| DETAIL-11 A | ✅ | với lưu ý §3.8 trên hồ sơ |
| DETAIL-12 HC-1 | ✅ | nếu field từ BE |
| DA-01 HC | ✅ | |
| DA-02 A | ✅ P2 | |
| DA-03 HC | ✅ P1 note | |
| DA-04 A | ✅ P3 | |
| DA-05 HC | ✅ H3 | |
| DA-06 Custom | ✅ ad-hoc §2 | thêm gate irregular |

---

**Docs consulted:** `deadline-alerts-real-data-implementation-plan.md`, `deadline-alert-detail-po-decisions-summary.md`, `business-contract-summary.md`, `business-contract-adhoc-alert-create.md`, `adhoc-alert-business-one-pager.md`, `adhoc-alert-crud-current-state-business-audit-summary.md`, `portal-template-va-workflow.md`, `deadline-alerts-tab-plan-review-summary.md`.

**Cache:** `docs/ai-cache/deadline-alert-detail-contract-alignment-assessment.md`
