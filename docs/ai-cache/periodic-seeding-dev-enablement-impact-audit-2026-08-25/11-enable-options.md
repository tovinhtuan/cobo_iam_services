# 11 — Options (plan only)

| Option | Summary | Verdict lean |
| --- | --- | --- |
| A Global enable now | Flip PERIODIC only | **UNSAFE** — worker missing WORKFLOW_SNAPSHOT |
| B Controlled window | Flip PERIODIC + SNAPSHOT on worker, restart, observe | **PREFERRED** if conditions met |
| C Allowlist / one-shot | Code change | Out of scope; conceptual only if B rejected |
| D Manual DB cycles | Bypass runtime | Reject for normal recovery |
| E Keep disabled | Target DAILY slot lost after HCM rollover | Acceptable only if product accepts miss |

```text
APPLICATION_CODE_FIX_REQUIRED=false
DEV_CONFIG_CHANGE_REQUIRED=true
DEFAULT_FALSE_REASON=safe default / rollout gate; DEV historically kept false after temporary flips (ai-cache)
FLAG_ROLLOUT_STATUS=rollout_gate (default false; compose.dev wants true; DEV .env forces false)
```
