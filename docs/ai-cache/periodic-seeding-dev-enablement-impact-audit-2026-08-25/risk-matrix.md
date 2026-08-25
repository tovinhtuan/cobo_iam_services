# Risk matrix

| Risk | Likelihood | Impact | Evidence | Mitigation |
| --- | --- | --- | --- | --- |
| Unexpected cycle burst | High | Med | 52 projected | thresholds + observe |
| Unexpected record burst | High | Med | ≤52 if snapshot on | same |
| Orphan drafts (snapshot off) | Certain if PERIODIC alone | **P0** | worker env missing SNAPSHOT | require SNAPSHOT=true |
| Immediate overdue obligations | High | Med-High | ~24 OVERDUE | accept DEV noise / warn users |
| Duplicate cycles | Low | Med | UNIQUE key + NOOP upsert | monitor |
| Duplicate records/workflows | Low if snapshot on | High | claim | monitor orphans |
| Worker 5s load | Med | Low-Med | full scan+upsert/tick | accept DEV; follow-up cadence |
| Restart side effects | Low | Low | first run after ~5s | planned |
| Notifications on materialize | Low | Low | none in path | n/a |
| Unknown company impact | Low | Med | only 4 applicable | matrix |
| Missed target slot after rollover | High if delayed | Med | current-slot only | enable same day or accept miss |
| Config rollback ≠ data undo | Certain | Med | persistence | separate cleanup review |
