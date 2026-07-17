# Phase 7 — Security Retest Matrix

| Test ID | Finding | Request/Test | Expected | Actual | Result | Evidence |
|---|---|---|---|---|---|---|
| RT-001 | COBO-SEC-001 | POST `/internal/reminders/dispatch` no token | 401 | 401 | PASS | runtime curl |
| RT-002 | COBO-SEC-001 | POST dispatch wrong token | 401/403 | 401 | PASS | runtime curl |
| RT-003 | COBO-SEC-001 | POST dispatch valid token (masked) | auth pass, handler reached | 400 business validation | PASS | runtime curl masked |
| RT-004 | COBO-SEC-002 | GET `/metrics` public | not 200 | 401 | PASS | runtime curl |
| RT-005 | COBO-SEC-003 | login/refresh rate limit | implemented or accepted plan | accepted dated plan | PASS (PLAN) | phase-04 doc |
| RT-006 | COBO-SEC-004 | default CMS secret rejected outside local/test | fail closed | enforced by config/runtime guard | PASS | source + tests |
| RT-007 | Regression | guest `/api/v1/me` | 401 | 401 | PASS | runtime curl |
| RT-008 | Regression | portal user activate CMS | 403 | 403 | PASS | runtime curl |
| RT-009 | Regression | cross-tenant deadline scope | different totals | c001=19, c002=1 | PASS | runtime curl |
| RT-010 | Regression | BE health | healthy | healthz ok, readyz ready | PASS | runtime curl |
| RT-011 | Browser smoke | CMS/Portal routes | load + no severe console + no 5xx | all loaded, 0 severe, 0 5xx | PASS | browser smoke |
