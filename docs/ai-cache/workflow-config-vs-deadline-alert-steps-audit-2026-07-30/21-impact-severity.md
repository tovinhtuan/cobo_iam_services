# 21 — Impact / severity

- Scope: types with portal `enterprise_workflow` but **no** global_workflow versions (clone/QA pattern); not “one bad record only”
- Old vs new: both use effective at materialize; CMS tab confusion is systemic UX
- Semantic code/order/actor on runtime path: **correct**
- Wrong assignment from CMS empty vs alert: **no evidence** of wrong actor on snapshot
- Data loss: none
- Severity: **P3** (operator confusion / dual-SoT UX); tasks 500 on detail: **P2** if blocks all detail actions (separate ticket)
