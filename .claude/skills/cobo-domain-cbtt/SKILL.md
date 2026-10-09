---
name: cobo-domain-cbtt
description: Kiến thức nghiệp vụ công bố thông tin (CBTT) chứng khoán Việt Nam cho Cobo - Thông tư 96/2020/TT-BTC (sửa đổi bởi TT68/2024), Nghị định 156/2020 về xử phạt; loại CBTT định kỳ/bất thường 24h/theo yêu cầu, thời hạn BCTC, báo cáo thường niên, quản trị, ĐHĐCĐ, cách tính ngày và ngày nghỉ. Dùng khi làm disclosure, template, deadline, workflow, nhắc hạn.
---

# Cobo Domain: CBTT (securities disclosure)

CoBo Portal helps Vietnamese public/listed companies track, approve and
publish mandatory disclosures. This skill gives the legal frame; the
implementation lives in `cobo_iam_services/internal/disclosure` (records,
templates, deadline engine, periodic cycles), `workflow*`, `deadlinealerts`,
`reminder`, `holiday`.

Full reference with articles and sources: `references/tt96-reference.md`.
Laws change: before hard-coding a rule, re-check the current text
(thuvienphapluat.vn / vbpl.vn) and record the source in the code comment or
spec. A draft circular to replace TT96 was under consultation in 2026.

## Core concepts
- **Categories**: định kỳ (periodic, Đ10/Đ14), bất thường (extraordinary,
  Đ11, within 24h), theo yêu cầu (on request, Đ12, within 24h), hoạt động khác
  (Đ13: offerings, listing, treasury shares, foreign ownership...).
- **Who**: legal representative or one authorized disclosure person (người
  được ủy quyền CBTT); change of that person disclosed within 24h.
- **Channels**: company website + SSC system (IDS) + exchange, sent at the
  same time.
- **Language**: Vietnamese prevails; English phased in (listed/large public
  companies: periodic from 2025, others from 2026; other public companies
  2027/2028) per TT68/2024.
- **Retention**: periodic info kept ≥10 years and online ≥5 years;
  extraordinary info online ≥5 years.

## Key periodic deadlines (summary - verify in reference)
| Item | Deadline |
|---|---|
| Audited annual FS | 10 days after audit report signed, max 90 days after FY end |
| Semi-annual reviewed FS (listed/large) | 5 days after review signed, max 45 days (parent with subsidiaries: 60) |
| Quarterly FS (listed/large) | 20 days after quarter end (parent: 30) |
| Annual report | 20 days after audited FS published, max 110 days after FY end |
| Corporate governance report | 30 days after end of H1 and of the year |
| AGM documents | ≥21 days before the meeting |
| AGM minutes/resolutions, extraordinary events | 24 hours |

## Day counting (important for the deadline engine)
- TT96 periodic deadlines say "ngày" → **calendar days**; 24h items count
  hours from the event. Working days appear only in specific clauses.
- If the obligation falls on a weekend/holiday, the company posts on its own
  website and completes full disclosure on the next working day (Đ7).
- Civil Code 2015 Đ147-148: a period ending on a rest day ends at the end of
  the next working day (applied to TT96 deadlines by inference).
- **Check the code**: the deadline engine (`disclosure/app/deadlineengine`,
  `add_days.go`) skips Sat/Sun/holidays, and workflow `dueRule` uses `T+N`
  (calendar days) / `H+N` (hours). For each template confirm whether its rule
  is calendar-day with roll-forward or working-day, and cite the clause.

## Penalties (motivation for alerts)
NĐ 156/2020 Điều 42 (organizations; individuals pay half): late disclosure
50-70M VND; non-disclosure 70-100M; false/misleading 100-200M. NĐ 306/2025
amended NĐ 156 from 2026 - verify current amounts.

## Domain rules in the product (from CLAUDE.md; verify)
- Only `draft` disclosures can be deleted.
- `published` requires an evidence link (SSC/HNX/HOSE link).
- System disclosure types are platform-managed, visible to all tenants;
  company types are scoped to the creating company.
- Subscription quota enforced server-side (HTTP 402).

## When writing code or specs
- Name the legal basis (article/khoản) for every deadline rule; store it with
  the template (`legal_basis` feature exists).
- Keep timezone `Asia/Ho_Chi_Minh` for all deadline math.
- Model "event time" separately from "disclosure time" and "report time".
- Don't present legal conclusions to users as advice; show the rule source.
