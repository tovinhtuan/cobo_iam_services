# Cross-layer call chain (exact source)

```text
CMS / DB: disclosure_type_versions.applicability_rules_json
  (use_structure_deadline, deadline_days, deadline_day_type, deadline_by_structure)
→ GET /api/v1/disclosure-types/{type_id}
→ handler.getTypeDetail
→ service.GetTypeDetail(Subject.CompanyID)          # company from JWT subject
→ repo.GetTypeDetail(companyID, typeID)
→ repo.GetCompanyApplicabilityProfile(companyID)    # has_subsidiaries, has_subordinate_accounting_units
→ applicability.ResolveStructure(profile)           # precedence subsidiaries>units>simple
→ applicability.ResolveDeadlineRule(rules, profile) # code + source + days
→ ResolveDeadlineDays (wrapper)                     # N for calculator override
→ buildResolvedDeadlineRuleDTO(...)
     + ResolveDeadlineDurationType → day_type
     + BaseDateSourceCycleStart if PERIODIC
→ CalculateDeadlineSummary / calculatePeriodic(cycleStart + N)
→ attachResolvedDueDate(deadline_summary.deadline_date)
→ JSON resolved_deadline_rule
→ FE createDisclosureTypesApi.getById(typeId)       # Bearer JWT company context
→ normalizeResolvedDeadlineRule (snake→camel)
→ formatResolvedDeadlineRule(dto)                   # no company flags
→ DisclosureDeadlineSection(displayModel, legacyRuleText)
```

## Company identity (FE)
| Concern | Exact source |
|---------|----------------|
| Selected company | `selectedCompany?.id` \|\| `localStorage.cobo_selected_company_id` |
| Token company | JWT `decodeJwtCompanyId(cobo_access_token)` |
| Switch epoch | `cobo_company_switch_epoch` / event `cobo:company-switched` |
| Effect key | `companyContextKey = selected:token:epoch` |
| Request | `GET /api/v1/disclosure-types/{typeId}` + optional `POST /auth/switch-company` |
| Stale guard | `requestSeqRef` + `setType(null)` on key change |
