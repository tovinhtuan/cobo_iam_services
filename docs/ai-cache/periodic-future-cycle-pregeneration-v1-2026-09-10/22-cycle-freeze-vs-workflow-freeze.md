# Freeze boundary

Phase A freezes on periodic_cycle: label, T/cycle_start, open_at, due_date (existing upsert semantics)
Does NOT freeze: workflow snapshot, department, roles, assignees
Those remain materialization-time
