---
name: premerge-system-review
description: Dùng như cổng kiểm tra cuối trước merge, tập trung vào rủi ro hệ thống, regression, security, observability và maintainability
---

# Premerge System Review

## When to use
Use this skill when:
- a feature is implemented
- a PR is ready for final review
- you want a systematic risk audit before merge

## Review areas
- requirement coverage
- architectural fit
- validation completeness
- permission/security issues
- data consistency risks
- UI state completeness
- test sufficiency
- logging/metrics/debuggability
- migration/deployment risks
- rollback/fallback readiness

## Severity model
- Critical: likely bug, security hole, data corruption, major regression
- Important: missing edge case, weak validation, maintainability risk, unclear contract
- Nice-to-have: cleanup, polish, refactor ideas

## Output format
- Change summary
- Critical findings
- Important findings
- Nice-to-have improvements
- Merge recommendation
