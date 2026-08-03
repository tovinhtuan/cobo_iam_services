# Open decisions

| # | Decision | Evidence | Options | Recommendation | Owner | Blocker? |
|---|----------|----------|---------|----------------|-------|----------|
| 1 | Paid plan SoT | Case B only | B1 entitlement display / C new table | Prefer **C** long-term; B1 only with explicit Product copy | Product+Backend | **Yes** for billing-truth badge |
| 2 | Entitlement vs billing | Resolver not billing | Separate forever / conflate | Separate; badge from company SoT | Product | Yes if conflate rejected |
| 3 | Free/null | No company row | omit vs Free | omit for badge FE | Product | Soft |
| 4 | Status enum | No company statuses | ACTIVE only interim / full enum | ACTIVE-only until C | Product | Soft for B1 |
| 5 | Endpoint | A/B/C audited | A+B / C | **A+B** | Backend | Soft |
| 6 | Profile vs me/companies | Both needed | A only / B only / A+B | A+B | Backend | Soft |
| 7 | display_name | FE shows raw user tier today | API display_name / FE i18n | API echo code interim | Product | Soft |
| 8 | Trial/expired UI | effective_to on user only | hide / show status | hide Premium if not active | Product | Soft |
| 9 | Read permission | company.view | same / stricter | same as company.view | Security | Soft |
| 10 | Migration/backfill | No table | none / new table | new if C | Backend | Depends #1 |
| 11 | Enterprise badge | User shows Enterprise string | Premium-only / Premium+Enterprise | Product | Soft |
| 12 | Switch cache | No plan cache | list embed | embed on /me/companies | Backend | Soft |
| 13 | Remove personal Premium timing | Phase A preserved | after BE+FE smoke | after Phase 6–7 | Product | Soft |
