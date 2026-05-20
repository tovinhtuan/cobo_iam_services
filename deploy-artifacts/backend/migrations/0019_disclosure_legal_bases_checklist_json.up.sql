-- Add structured legal bases and checklist payloads for template detail parity.
ALTER TABLE disclosure_type_versions
  ADD COLUMN legal_bases_json JSON NULL AFTER reminder_milestones_json,
  ADD COLUMN checklist_json JSON NULL AFTER legal_bases_json;
