-- 0080: Persist cycle_start at seed for periodic T0 (contract AC-9, §2.1).
ALTER TABLE periodic_cycles
  ADD COLUMN cycle_start DATE NULL AFTER cycle_label;
