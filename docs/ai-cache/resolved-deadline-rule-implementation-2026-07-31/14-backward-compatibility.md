# Backward compatibility

| Check | Result |
|-------|--------|
| Additive `resolved_deadline_rule` only | PASS |
| Existing JSON field names unchanged | PASS |
| `deadline_summary` still computed | PASS (tests assert presence) |
| Auth / RBAC / HTTP status | Unchanged code paths |
| Old FE ignoring unknown field | Compatible |
| `ResolveDeadlineDays` signature unchanged | PASS (wrapper) |
| Calculator semantics | Unchanged (const only) |
