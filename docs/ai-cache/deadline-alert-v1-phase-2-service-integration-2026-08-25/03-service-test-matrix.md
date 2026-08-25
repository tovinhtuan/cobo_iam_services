# 03 — Service test matrix

| Flag | Test |
|------|------|
| ACTIONABLE_DRAFT_SURVIVES_SERVICE | TestListDeadlineAlerts_actionableDraftSurvivesAndPaginates |
| ACTIONABLE_DRAFT_FUTURE_DUE_STATUS | TestListDeadlineAlerts_actionableDraftStatusClassification (UPCOMING) |
| DUE_TODAY_EXISTING_STATUS | same (DUE_SOON) |
| ACTIONABLE_DRAFT_OVERDUE_STATUS | same (OVERDUE) |
| MULTI_ROW_STATUS_CLASSIFICATION | same |
| UNRESOLVED_DUE_BEHAVIOR_PRESERVED | TestListDeadlineAlerts_unresolvedDueSkipped |
| CONFIRMATION_PRESENTATION_PRESERVED | TestListDeadlineAlerts_confirmedDraftRemainsActiveObligation + existing confirmed/published tests |
| SERVICE_COUNT_LIST_PARITY | pagination test total==2 with pageSize=1 |

Legacy presentation tests (non-Draft stub rows) retained for ad-hoc / terminal / scope / filters — not membership authority.
