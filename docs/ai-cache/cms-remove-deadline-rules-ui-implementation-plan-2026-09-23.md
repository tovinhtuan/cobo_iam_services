# CMS remove Deadline Rules UI — implementation plan (analysis only)

```text
task_type: implementation-plan (pre-coding)
date: 2026-09-23
repos: cobo_iam_services, cobo_web_design
skills: system-design-feature, integration-cross-repo
code_changed: false
migration: none in phase 1–2
commit: not performed
verdict: READY FOR IMPLEMENTATION AFTER PO LOCKS (§9)
related:
  - docs/ai-cache/portal-resolved-deadline-display-implementation-plan-2026-09-22.md
  - docs/ai-cache/portal-resolved-deadline-display-dev-smoke-2026-09-22.md
  - docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/
```

---

## 1. Executive summary

Admin muốn **bỏ tab CMS `Deadline Rules`** và **bỏ ô nhập `Deadline rule hiển thị trên Portal`**. Deadline Portal cho template **periodic** lấy từ **`applicability_rules` + company profile** qua `resolved_deadline_rule` (đã có). Template **irregular/event-based** không lấy N từ applicability; nguồn là `deadline_config` / FIXED_DATE / DYNAMIC_RULE / mô tả sự kiện — cần lock rõ với PO.

Hai khái niệm hiện đang **tách biệt** trong code:

| Concept | Storage | UI hôm nay | Dùng để |
|---------|---------|------------|---------|
| **Deadline Rules catalog** | bảng `deadline_rule_catalog` | Tab `?tab=deadline-rules` | CRUD pattern codes (`T+N`…); **không** gắn picker vào textarea template |
| **Template `deadline_rule`** | cột `disclosure_type_versions.deadline_rule` | Textarea “Deadline rule hiển thị trên Portal” | Free-text fallback Portal khi thiếu `resolved_deadline_rule` |

**Khuyến nghị:** Phase 1 chỉ **ẩn/gỡ UI** + derive compatibility `deadline_rule` khi save (FE hoặc BE). **Không** xóa cột DB, **không** xóa catalog API, **không** migration cho đến khi chứng minh hết consumer.

Portal precedence semantic **đã đúng hướng** (resolved → raw → empty). Việc cần làm: dừng admin authoring raw text; đảm bảo save/import/activate không regression; không hiển thị compatibility text khi đã resolve.

---

## 2. Findings (file / function / line)

### 2.1 Frontend — CMS tab & catalog

| Finding | Location |
|---------|----------|
| Tab type + label `Deadline Rules` | `cobo_web_design/src/features/cms-core/templates/TemplatesFeatureScreen.tsx` L10–15 |
| Render catalog | same file L149–150 → `DeadlineRuleCatalogScreen` |
| URL | `/cms/templates?tab=deadline-rules` (query only; **không** có route path riêng) |
| Catalog screen | `.../catalog/DeadlineRuleCatalogScreen.tsx` |
| Hook CRUD | `.../catalog/useDeadlineRuleCatalog.ts` → `list/create/update/deleteDeadlineRule*` |
| HTTP | `cmsApi.ts` ~L2141–2177: `GET/POST/PATCH/DELETE /api/v1/platform/cms/deadline-rules` |
| Reference-data seed list | `cmsApi.getDisclosureTemplateReferenceData` ~L1609–1642 đọc `deadline_rule_catalog` từ `/api/v1/admin/disclosure-types/reference-data` |
| Dead path still loads catalog | `features/cms-core/pages.tsx` ~L1487–1517 (sau early return `TemplatesFeatureScreen` vẫn có code cũ) |

### 2.2 Frontend — Template editor field

| Finding | Location |
|---------|----------|
| Label “Deadline rule hiển thị trên Portal” | `TemplateEditorScreen.tsx` ~L1360–1395 (`content-block-deadline`) |
| Required client validation | `templateValidation.ts` L66–71 |
| Default create `T+20` | `templateDefaults.ts` → `PERIODIC_CREATE_DEADLINE_RULE`; `templateMappers.createDefaultTemplateForm` L776 |
| Save derive | `templateMappers.resolveDeadlineRuleForSave` L1231–1242 → upsert `deadlineRule` L1270 |
| Applicability UI (engine SoT) | `TemplateApplicabilitySection.tsx` L148–235: `deadline_days`, `deadline_day_type`, `useStructureDeadline`, structure map |
| CMS list column still shows `deadlineRule` | `TemplatesListScreen.tsx` ~L330 |
| Portal preview | `portalPreview/cmsPortalPreviewAdapter.ts` synth resolved từ days hoặc regex `T+N`; shell **không** pass `rawDeadlineRule` |
| Company-template create modal | `DisclosureTypeList.tsx` vẫn có form `deadlineRule` / API `deadline_rule` (~L861–875) |

### 2.3 Frontend — Portal display precedence

| Layer | Precedence (đã ship) |
|-------|----------------------|
| Detail UI | `DisclosureDeadlineSection.tsx` L69–145: **resolved description** → **raw `deadline_rule` nguyên văn** → no-rule / controlled fallback |
| List UI | `deadlineDisplayHelpers.resolvePortalListDeadlinePrimaryText`: resolved → irregular raw/event → raw → … |
| Formatter | `resolvedDeadlineRuleDisplay.formatResolvedDeadlineRule` — **chỉ** DTO resolved; không invent từ free-text |
| Absolute due | `resolved_due_at` / `resolved_due_source` (cycle → preview); irregular skip absolute |

→ Mục tiêu product **không cần đổi precedence Portal**; chỉ cần ngừng authoring raw và tránh hiện compatibility khi resolved OK.

### 2.4 Backend — storage & save

| Finding | Location |
|---------|----------|
| Column | `migrations/0012_disclosure_catalog_versions.up.sql` L40: `deadline_rule TEXT NULL` |
| Catalog table | `migrations/0053_...`, `0060_deadline_rule_catalog.up.sql` |
| Matrix **required** non-empty | `template_validation.go` L152–154, L197–199: `deadline_rule is required` |
| Display validate | `cms_template_deadline_rule_validation.go` L14–29: empty OK; max 1000 runes; **không** match catalog |
| Upsert gate | `service.go` ~L860–881: nếu `!SkipPublicationMatrix` → matrix + `validatePortalDeadlineRule` + `applicability.ValidateRules` |
| Draft workflow skip | `cms_service.go` L137, L167: `SkipPublicationMatrix=true` |
| Persist | `infra/mysql/repository.go` ~L1090: `NULLIF(?, '')` → empty → SQL NULL |
| Import | schema `template-import-v1.schema.json` required `deadline_rule`; `template_import_validator.go` `DEADLINE_RULE_REQUIRED`; confirm L545 |
| Activate | `service.go` ~L1065: length-only; **không** re-require matrix |
| Archive/Restore | archive không đụng field; restore gọi `validatePortalDeadlineRule` (`cms_archive_restore.go` ~L132) |
| Company template | `CreateCompanyTemplate` / `UpdateCompanyTemplate` `service.go` ~L2024–2087: matrix + display validate |

### 2.5 Backend — resolve & irregular

| Finding | Location |
|---------|----------|
| Resolve N | `applicability/rules.go` `ResolveDeadlineRule` L81–119 từ `deadline_days` / structure map |
| Attach Detail | `service.go` `GetTypeDetail` L669–675: nếu có `ApplicabilityRules` + company profile → `buildResolvedDeadlineRuleDTO` (**không** phân nhánh periodic/irregular ở đây) |
| Attach List | `list_resolved_due.go` `enrichPortalListResolvedDue` |
| Absolute skip irregular | `skipsAbsoluteDueResolution` L93–108 (`irregular` / `ad_hoc` / `event_based` / mode NONE) |
| Calculator modes | `deadline_calculator.go`: `FIXED_DATE`, `DYNAMIC_RULE`, `PERIODIC`, `NONE` |
| Catalog routes | `transport/http/handler.go` L96–99; handlers `cms_handler.go` L266–339 |

### 2.6 Consumers còn lại của catalog / `deadline_rule` (không xóa sớm)

**Giữ catalog API/DB ít nhất đến Phase 4:**

- CMS tab (sẽ gỡ UI Phase 1)
- `reference-data.deadline_rule_catalog`
- `enrichDeadlineRuleDisplay` / legacy helpers (`deadline_rule_display.go`) — Portal hiện ưu tiên exact trim, nhưng API vẫn có field
- Tests + seed migrations

**Giữ cột `deadline_rule`:**

- Portal raw fallback (legacy templates)
- Import schema / confirm
- Company template create/update
- Flat block sync (`template_flat_block_sync.go`)
- Ad-hoc proposal path FE (`AdHocProposalCreatePage` / `inlineWorkflowStepContract`) — **domain khác**, không gộp vào CMS template editor removal
- CMS list column / version diff

---

## 3. Proposed target contract

### 3.1 Periodic (SoT)

```text
Authoring SoT (CMS):
  applicability_rules.deadline_days
  applicability_rules.deadline_day_type
  applicability_rules.use_structure_deadline
  applicability_rules.deadline_by_structure.{has_subsidiaries|has_subordinate_units|simple_structure}.days
  + company applicability profile (runtime)

Portal semantic:
  resolved_deadline_rule  (backend ResolveDeadlineRule)
  → raw deadline_rule chỉ khi KHÔNG resolve được (legacy / thiếu profile / NO_RULE)
  → “Chưa xác định”

Portal absolute due (đã có):
  CYCLE_DUE | PLANNED_DATE → DEADLINE_SUMMARY_PREVIEW → ẩn ngày
```

Admin **không** nhập `deadline_rule`. Compatibility value (nếu BE còn required) **không** được Portal ưu tiên hơn resolved.

### 3.2 Irregular / event-based (SoT — cần PO lock)

**Không** dùng `applicability_rules.deadline_days` / structure map làm “hạn định kỳ”.

| Mode / case | Nguồn chính hiển thị | Absolute due |
|-------------|----------------------|--------------|
| Event-relative mô tả | Free-text legacy `deadline_rule` **hoặc** field mô tả sự kiện trong content (quyết định PO) | Không (`skipsAbsoluteDueResolution`) |
| `deadline_config.deadline_mode = FIXED_DATE` | `fixed_deadline.date` + `deadline_summary` | Theo summary nếu có |
| `deadline_config.deadline_mode = DYNAMIC_RULE` | `dynamic_rule` (base + duration) + summary | Theo summary nếu có |
| Workflow step `due_rule` | Step-level (đã tách) — không thay template deadline | N/A |

**Recommendation (BE Phase 2/3):** Chỉ attach `resolved_deadline_rule` khi template behavior = **periodic** (frequency / category / mode PERIODIC). Irregular: `resolved_deadline_rule = null`; Portal dùng raw hoặc summary FIXED/DYNAMIC — tránh “Trong vòng N ngày theo lịch” nhầm từ leftover applicability.

### 3.3 Field lifecycle

| Field | Phase 1–2 | Phase 3 | Phase 4+ |
|-------|-----------|---------|----------|
| `applicability_rules.*` deadline | **SoT** | SoT | SoT |
| `deadline_config` | Giữ (engine + irregular) | Giữ | Giữ |
| `deadline_rule` column | Compatibility write / legacy read | Optional on write | Deprecate / stop writing |
| `deadline_rule_catalog` table + API | Keep (no UI) | Keep until zero consumers | Drop nếu audit PASS |
| `resolved_deadline_rule` | SoT Portal semantic | SoT | SoT |
| `resolved_due_*` | SoT absolute | SoT | SoT |

### 3.4 Compatibility value (khi save vẫn required)

**Recommendation (ưu tiên BE derive, FE thin):**

```text
On UpsertTypeVersion / CreateCompanyTemplate / Import confirm:
  if deadline_rule empty AND periodic AND applicability has deadline_days > 0:
    set deadline_rule = "T+" + deadline_days   // machine-readable compatibility only
  else if empty AND irregular:
    set deadline_rule = sentinel short text OR keep existing DB value
  else:
    keep client-provided (legacy import)
```

Portal: **không** hiện `T+N` nếu `resolved_deadline_rule` OK (đã đúng với `DisclosureDeadlineSection`).

**Alternative FE-only (Phase 1 nhanh):** mở rộng `resolveDeadlineRuleForSave` luôn derive khi field ẩn; vẫn gửi `deadline_rule`. Rủi ro: hai chỗ derive (FE+BE) lệch — chỉ dùng tạm nếu Phase 1 ship trước BE.

---

## 4. Phased implementation plan

### Phase 1 — UI removal (FE-first, BE unchanged)

**Scope**

1. `TemplatesFeatureScreen.tsx`: xóa tab `deadline-rules`, import, branch render.
2. `TemplateEditorScreen.tsx`: xóa block textarea “Deadline rule hiển thị trên Portal”; cập nhật callout → trỏ admin sang **Phạm vi áp dụng**.
3. `templateValidation.ts`: **bỏ** required `deadlineRule` phía form (field ẩn).
4. `templateMappers.resolveDeadlineRuleForSave`: luôn derive compatibility khi periodic; irregular → sentinel / giữ existing khi edit (`domainToForm` vẫn map field ẩn trong state nếu cần merge).
5. `TemplatesListScreen`: cột Deadline — hiển thị preview từ applicability days **hoặc** “Theo phạm vi áp dụng” thay vì raw column (không hiện `T+20` compatibility như “nội dung Portal”).
6. CMS Portal Preview: dựa `deadlineDays` / structure default (không regex textarea); disclaimer “preview theo config template, chưa theo company”.
7. Tests FE: catalog screen tests có thể giữ file nhưng unmount từ feature; editor tests bỏ assert textarea.
8. **Chưa** xóa `DeadlineRuleCatalogScreen.tsx` / `useDeadlineRuleCatalog.ts` / `cmsApi` CRUD (dead code tạm OK; hoặc mark deprecated comment).
9. Company-template create modal trên Portal: **cùng phase hoặc follow-up ngay** — bỏ ô `deadlineRule` nếu trong scope “admin không nhập”; derive khi POST.

**Out of scope Phase 1:** BE matrix, import schema, catalog DELETE API, migration, AdHoc.

**Rollback:** revert FE commit; BE/API không đổi.

### Phase 2 — Compatibility save contract (BE + FE align)

**Scope**

1. BE: helper `EnsureCompatibilityDeadlineRule(req)` trước matrix **hoặc** nới matrix: allow empty nếu periodic+days>0 rồi fill server-side.
2. Import: nếu JSON thiếu `deadline_rule` nhưng có `applicability_rules.deadline_days` → auto-fill trước validate; schema có thể nới `minLength` trong phase sau.
3. Document: `deadline_rule` = deprecated display/compatibility; SoT = applicability.
4. Unit tests: empty client `deadline_rule` + valid applicability → upsert PASS; DB non-empty compatibility.
5. Irregular attach policy (recommendation): skip `buildResolvedDeadlineRuleDTO` khi không periodic.

**Rollback:** feature flag `COMPAT_DEADLINE_RULE_DERIVE=off` → yêu cầu client gửi như cũ.

### Phase 3 — Backend contract cleanup (optional write)

1. `validatePortalTemplateMatrix` / `validateTemplateMatrix`: `deadline_rule` **optional**.
2. Import schema: `deadline_rule` optional với derive.
3. Stop writing compatibility nếu resolved luôn đủ (chỉ khi product đồng ý Portal không cần raw fallback cho template mới).
4. CMS list/API docs cập nhật.

**Không** drop column.

### Phase 4 — Catalog / API / DB cleanup (chỉ khi an toàn)

Gate trước khi làm:

```text
[ ] Zero callers FE của listDeadlineRulesCatalog / CRUD (grep CI)
[ ] Zero callers reference-data.deadline_rule_catalog trong UI
[ ] enrichDeadlineRuleDisplay không còn cần catalog map
[ ] Staging smoke: Portal / CMS / import / company-template
[ ] PO approve drop table
```

Thứ tự: deprecate API → remove FE dead files → migration drop `deadline_rule_catalog` (riêng) → **cột template `deadline_rule` giữ lâu hơn** (legacy read).

---

## 5. API / data migration impact

| Area | Phase 1 | Phase 2 | Phase 3–4 |
|------|---------|---------|-----------|
| Portal list/detail response | Không đổi shape | Có thể: irregular `resolved_deadline_rule` null hơn | Docs only |
| Upsert request | Vẫn gửi `deadline_rule` (derived) | Có thể omit; BE fill | Optional |
| Catalog endpoints | Giữ | Giữ | Deprecate → remove |
| DB `deadline_rule` | Giữ | Giữ | Giữ (deprecate write) |
| DB `deadline_rule_catalog` | Giữ | Giữ | Drop nếu gate PASS |
| Migration mới Phase 1–2 | **Không** | **Không** | Phase 4 only |
| Activate / archive / restore | Không đổi | Length-only vẫn OK | Optional field |
| Company template | FE derive | BE ensure | Optional |

**Authorization / audit:** Catalog CRUD permissions không đổi Phase 1 (API còn). Upsert audit vẫn log `deadline_rule` derived — OK. Không đổi authz Portal.

---

## 6. Frontend implementation checklist

1. Remove tab + wiring `TemplatesFeatureScreen`.
2. Remove editor section; update help copy → Applicability.
3. Validation: không bắt `deadlineRule`; vẫn validate applicability periodic days / structure / FIXED_DATE / DYNAMIC_RULE.
4. Mapper: derive-only save; edit load có thể clear UI state nhưng giữ hidden merge từ server nếu cần.
5. List CMS column: không quảng cáo compatibility `T+N` như copy Portal.
6. Portal: **không** đổi precedence trừ khi Phase 2 đổi attach irregular.
7. States: loading/error Applicability giữ nguyên; empty periodic days → block save (đã có `validateApplicabilityRulesClient`).
8. i18n: xóa/đổi string label Deadline Rules tab nếu có.
9. Tests: `TemplateEditorScreen*`, `templateValidation*`, `templateMappers*`, `TemplatesFeatureScreen*`, `useDeadlineRuleCatalog*` (orphan OK), Portal regression helpers.

---

## 7. Backend implementation checklist

1. Phase 2: `EnsureCompatibilityDeadlineRule` gần `UpsertTypeVersion` / company create / import confirm.
2. Optionally: `GetTypeDetail` / list enrich — attach resolved **chỉ periodic**.
3. Keep `ValidateRules` E01–E06; cân nhắc siết `deadline_days > 0` cho periodic (hiện `ValidateRules` **không** bắt `deadline_days > 0` — FE client bắt; **open question** align BE).
4. Do **not** change ResolveDeadlineRule math (đã smoke PASS).
5. Catalog handlers: no-op Phase 1–3.
6. Tests: matrix empty+days; import missing rule+days; activate archived restore; company template; irregular no false resolved; list/detail parity.

---

## 8. Test matrix

| Case | Layer | Expect |
|------|-------|--------|
| Periodic no structure | FE+BE+Portal | resolved DEFAULT days; UI không hiện compatibility nếu resolved OK |
| Periodic structure override A/B | Portal/API | days khác nhau (đã smoke) |
| Calendar vs working | Portal copy | `day_type` đúng |
| Missing company profile | Portal | không resolved → raw/compatibility hoặc “Chưa xác định”; không crash |
| Irregular FIXED_DATE | CMS+Portal | summary/date; không periodic sentence từ applicability |
| Irregular DYNAMIC_RULE | CMS+Portal | duration path |
| Irregular event text | Portal | raw/event; no absolute due |
| Legacy template có `deadline_rule` dài | Portal | fallback raw khi NO_RULE |
| Import JSON có rule | Import | PASS |
| Import thiếu rule + có days | Phase 2 | derive → PASS |
| Activate | BE | PASS |
| Archive/restore | BE | PASS |
| Company template create | FE+BE | không bắt admin nhập text |
| Catalog API still up | Phase 1–3 | 200 CRUD (unused UI) |
| Regression absolute due List=Detail | Portal | giữ SoT đã ship |

---

## 9. Acceptance criteria

1. Admin **không** thấy tab `Deadline Rules`.
2. Admin **không** nhập `Deadline rule hiển thị trên Portal` trên template editor.
3. Periodic Portal semantic = applicability + company (`resolved_deadline_rule`).
4. Khi đã resolve: **không** hiện compatibility `T+N` / sentinel như primary text.
5. Template cũ đọc được (raw fallback khi cần).
6. Save / activate / import / archive-restore / company-template **không** regression.
7. Catalog API/DB **chưa** xóa cho đến audit consumer = 0.
8. Irregular không bị ép periodic absolute due / structure N (theo policy đã lock).

---

## 10. Risks / open questions (cần PO lock trước code)

| # | Question | Recommendation |
|---|----------|----------------|
| Q1 | Compatibility derive ở **FE** hay **BE**? | **BE Ensure** (Phase 2) + FE derive tạm Phase 1 nếu ship UI trước |
| Q2 | Irregular còn cho phép free-text mô tả sự kiện ở đâu nếu bỏ textarea? | Option A: giữ textarea **chỉ irregular**; Option B: dùng `description`/block content; Option C: chỉ FIXED/DYNAMIC. **Ưu tiên Option A** (scope nhỏ, rõ) |
| Q3 | Có attach `resolved_deadline_rule` cho irregular hôm nay → có thể sai | Phase 2: attach **chỉ periodic** |
| Q4 | BE `ValidateRules` không bắt `deadline_days > 0` | Align BE với FE periodic required days trong Phase 2 |
| Q5 | Company-template modal + CMS list column có trong Phase 1? | **Có** — cùng mục tiêu “không nhập tay” |
| Q6 | Xóa file catalog FE ngay Phase 1? | **Không** — unmount tab only; xóa file Phase 4 |
| Q7 | Import schema breaking? | Phase 2 soft-derive; Phase 3 optional field |
| Q8 | AdHoc `deadline_rule` | **Out of scope** — không đụng |

**Không làm ở phase đầu:** migration drop column/table; xóa catalog API; đổi ResolveDeadlineRule; FE tự tính ngày theo company thật trong CMS preview.

---

## 11. Rollout / rollback

```text
Phase 1 FE → DEV smoke CMS tab gone + editor + Portal A/B semantic
Phase 2 BE derive → DEV import/upsert empty rule
Phase 3 optional matrix → staging
Phase 4 catalog drop → chỉ sau consumer audit
```

Rollback từng phase độc lập. Feature flag BE cho derive nếu cần.

Observability: log upsert khi auto-fill compatibility (`type_id`, `derived=true`) — không log secrets.

---

## 12. Pre-implementation review (gaps trước khi coding)

### Gaps phải lock

1. **Irregular authoring UX** (Q2) — ảnh hưởng có xóa hẳn textarea hay conditional.
2. **Periodic-only resolved attach** (Q3) — ảnh hưởng Portal irregular regression.
3. **Phase 1 ship FE-only** vs đợi BE Ensure cùng PR — tránh dual-derive drift.

### Risks

| Risk | Mitigation |
|------|------------|
| Save 422 `deadline_rule is required` sau khi ẩn field | Derive trước gửi **bắt buộc** Phase 1 |
| Portal hiện `T+N` compatibility | Giữ precedence resolved-first; smoke A/B |
| Import FAIL thiếu field | Phase 2 derive; đến lúc đó import vẫn gửi rule |
| Admin nhầm Applicability với display | Copy/help TemplateApplicabilitySection đã có — tăng cường Phase 1 |
| Dead catalog API bị tool khác gọi | Giữ API; grep monorepo trước Phase 4 |

### Điều kiện READY TO CODE

```text
[ ] PO lock Q2 (irregular textarea policy)
[ ] PO lock Q1 timing (FE-only Phase 1 OK?)
[ ] Agree Phase 1 includes company-template modal + CMS list column copy
[ ] Agree không migration / không xóa catalog API
[ ] Smoke plan reuse portal-resolved-deadline DEV fixtures
```

### Final recommendation

```text
IMPLEMENT: Phase 1 FE UI removal + derive-on-save
THEN: Phase 2 BE EnsureCompatibilityDeadlineRule + periodic-only resolved attach
DEFER: catalog/API/DB drop to Phase 4
DO NOT: drop deadline_rule column in near term
```

**Verdict plan:** `READY FOR IMPLEMENTATION AFTER PO LOCKS (Q1, Q2)`.

---

## 13. Docs consulted

- `docs/ai-cache/README.md`
- `docs/ai-cache/portal-resolved-deadline-display-implementation-plan-2026-09-22.md`
- `docs/ai-cache/portal-resolved-deadline-display-dev-smoke-2026-09-22.md`
- `docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/` (pointer set)
- Source audits: FE TemplatesFeatureScreen / TemplateEditor / mappers / validation / Portal section; BE template_validation / applicability/rules / service GetTypeDetail / catalog routes / import

```text
Không sửa code.
Không tạo migration.
Không commit / không push.
```
