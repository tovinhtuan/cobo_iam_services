# 22 — Fix options (NOT implemented)

| Opt | Scope | Benefit | Risk | Data | Rematerialize | Tests | Rollout | Rec |
|-----|-------|---------|------|------|---------------|-------|---------|-----|
| D/E | FE CMS+Alert show source/version labels | Clarity | Low | None | No | UI unit | DEV→stage | **Yes** |
| A | Empty-state copy + link dual-SoT | Clarity | Low | None | No | UI | DEV | **Yes** |
| F | Future types publish global workflow | Align SoT | Medium confuse | New only | No historical | E2E | gated | Optional |
| B | Alert API mapping | N/A steps already OK | — | — | — | — | — | No |
| C | Materializer | Already correct | High if change | — | — | — | — | No |
| Tasks-500 | Fix list tasks endpoint | Unblock detail UI | Med | None | No | API+FE | DEV | **Separate P2** |

Do **not** rematerialize historical records before contract lock.
