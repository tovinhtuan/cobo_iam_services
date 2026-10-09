# Finding format (all Cobo reviewers)

Every reviewer subagent returns exactly this structure.

```markdown
## Reviewer: <agent-name>
Scope reviewed: <repos/paths/diff range>
Skill(s) applied: <skill names>

### Findings
#### [CRITICAL|HIGH|MEDIUM|LOW|INFO] <short title>
- id: <PREFIX>-<NN>            # FES, BES, ROLE, PERF, CACHE, API, SEC
- group: <one of the 6 risk groups>
- location: <repo>/<path>:<line>[, ...]
- evidence: <≤5 lines quoted code/config, secrets replaced by ***>
- failure scenario: <concrete input/state → wrong outcome>
- fix: <smallest safe change>
- confidence: confirmed | plausible

### Checked, no issue
- <area>: <what was checked, file refs>

### Not checked / BLOCKED
- <area>: <reason>
```

## Risk groups
1. Security Frontend
2. Security Backend
3. Backend Performance & Reliability
4. Cache & Versioning
5. API Compatibility
6. CORS & Credentials
(+ Admin role / tenant isolation is reported under Security Backend or
Security Frontend, whichever side the defect lives on.)

## Severity rubric
- CRITICAL: exploitable now without special preconditions, cross-tenant data
  exposure, privilege escalation, secret usable against a running
  environment, data loss/corruption.
- HIGH: likely under normal use or exploitable with modest preconditions;
  outage risk under expected load; breaking change for deployed clients.
- MEDIUM: needs unusual conditions, defense-in-depth gap, degraded
  performance, missing guard with partial mitigation.
- LOW: hygiene, hardening, unlikely edge case.
- INFO: observation, no action required.

`confirmed` = verified by reading the exact code path (or running a safe
local check). `plausible` = pattern suggests a problem but a link in the
chain was not verified. Never mark something CRITICAL/HIGH and plausible
without saying which link is unverified.
