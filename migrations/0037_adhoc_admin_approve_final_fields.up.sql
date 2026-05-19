ALTER TABLE ad_hoc_proposals
  ADD COLUMN final_t0_date DATE NULL AFTER change_note,
  ADD COLUMN final_deadline_date DATE NULL AFTER final_t0_date,
  ADD COLUMN adjustment_note TEXT NULL AFTER final_deadline_date;
