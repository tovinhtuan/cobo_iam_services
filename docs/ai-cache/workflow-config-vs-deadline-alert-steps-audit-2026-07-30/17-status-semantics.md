# 17 — Status / current-step semantics

- Design steps: 4 (effective/snapshot)
- Runtime task: 1 pending on first step
- step_states: empty (no completions)
- Alert list: shows **current** step name only (runtime pointer)
- Alert steps API: all 4 with status current/overdue+locked
- Contract: list may show current; detail should show snapshot timeline (WF cards)
- Detail UI blocked by tasks 500 (separate issue)
