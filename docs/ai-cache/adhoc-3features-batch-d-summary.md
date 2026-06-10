# Batch D Summary — Email Regression Gates

**Date:** 2026-06-10  
**Status:** PASS (automated); Email QA thật pending

## Files Changed

- `internal/adhoc/infra/notification/template_render_test.go` — E-CLONE, E-AUTOFILL, E-INLINE, E-REGRESSION
- `internal/adhoc/infra/notification/notifier_test.go` — vars regression tests

## Tests Added

| Gate | Test |
|------|------|
| E-CLONE | `TestEmailRegression_EClone`, `TestEmailRegressionVars_EClone` |
| E-AUTOFILL | `TestEmailRegression_EAutofill`, `TestEmailRegressionVars_EAutofill` |
| E-INLINE | `TestEmailRegression_EInline`, `TestEmailRegressionVars_EInline` |
| E-REGRESSION | `TestEmailRegression_ERegression`, `TestEmailRegressionVars_ERegression` |

## Email Contract

- Visible body: no UUID, `type_id`, `membership_id`, `workflow_instance_id`
- `proposal_id` / `record_id` chỉ trong CTA href (V2.1 P2)
- `portal_url` absolute, no localhost

## Email QA Evidence (real)

- **NOT RUN** — requires deploy + real proposal submit/approve/reject on DEV

## Risks

- CTA URL vẫn chứa ID trong path (P2 allowed) — user có thể thấy trong link text email

## Verdict

**PASS** (Go tests `go test ./internal/adhoc/infra/notification/...`). Real inbox QA **FAIL/PENDING**.

**Cached for:** Team reuse, code reviews, future analysis
