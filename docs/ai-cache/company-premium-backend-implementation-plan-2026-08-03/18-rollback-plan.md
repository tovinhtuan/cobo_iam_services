# Rollback plan

| Scenario | Action |
|----------|--------|
| API field bad | Unwire Reader; omit `plan`; redeploy API |
| Migration Case C | down migration if unused; else stop writer + hide FE |
| FE new / BE old | FE hides badge when plan absent — **no user.tier fallback** |
| BE new / FE old | Compatible |
| Cache | flush `company_plan:*` if introduced |
| Partial | Prefer BE-first additive, FE second |
