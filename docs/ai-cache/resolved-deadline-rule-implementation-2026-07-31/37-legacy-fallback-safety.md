# Legacy fallback safety (C13)

## Priority
1. `resolved_deadline_rule` present → `formatResolvedDeadlineRule` only (legacy bait never shown). Proven in cross-layer tests (legacy text `T+999 · Có công ty con (legacy bait)` not rendered when DTO present).
2. DTO absent → `legacyRuleText` from CMS `deadline_rule` / display (passthrough). No company-flag inspection. No FE criterion matching. No structure context line.

## Safety claims proven
| Claim | Result |
|-------|--------|
| Does not inspect company flags | PASS |
| Does not FE-match criterion | PASS |
| DTO takes priority over legacy | PASS |
| New semantic path does not claim period end | PASS |

## Residual (documented, not fixed in Phase 4)
CMS free-text `deadline_rule` may historically contain arbitrary wording (including period-end). Rolling deploy without BE field uses that CMS text as-is. Preferred Product copy (“Kỳ hạn theo cấu hình hiện hành…”) **not** implemented — would be feature change; requires user approval.

**Not** `BLOCKED_LEGACY_FALLBACK_SAFETY`: FE cannot invent “Có công ty con” from company profile when DTO missing.
