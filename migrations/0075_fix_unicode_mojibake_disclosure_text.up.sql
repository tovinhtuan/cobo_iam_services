-- 0075: Repair Mojibake in disclosure/ad-hoc text columns (draft).
-- Same root cause as 0036/0072: UTF-8 bytes stored via latin1 connection.
-- Per-column guard: only convert when HEX lacks Vietnamese Extended UTF-8 (%E1B%).
-- Safe to re-run: already-correct columns are left unchanged.

SET NAMES utf8mb4;

UPDATE disclosure_records
SET
  title = CASE
    WHEN title IS NOT NULL AND title != '' AND HEX(title) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(title USING latin1) USING utf8mb4) ELSE title END,
  summary = CASE
    WHEN summary IS NOT NULL AND summary != '' AND HEX(summary) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(summary USING latin1) USING utf8mb4) ELSE summary END,
  content = CASE
    WHEN content IS NOT NULL AND content != '' AND HEX(content) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(content USING latin1) USING utf8mb4) ELSE content END
WHERE (title IS NOT NULL AND title != '' AND HEX(title) NOT LIKE '%E1B%')
   OR (summary IS NOT NULL AND summary != '' AND HEX(summary) NOT LIKE '%E1B%')
   OR (content IS NOT NULL AND content != '' AND HEX(content) NOT LIKE '%E1B%');

UPDATE disclosure_type_versions dtv
INNER JOIN disclosure_types dt ON dt.type_id = dtv.type_id AND dt.active_version_no = dtv.version_no
SET
  dtv.name = CASE
    WHEN dtv.name IS NOT NULL AND dtv.name != '' AND HEX(dtv.name) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(dtv.name USING latin1) USING utf8mb4) ELSE dtv.name END,
  dtv.category = CASE
    WHEN dtv.category IS NOT NULL AND dtv.category != '' AND HEX(dtv.category) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(dtv.category USING latin1) USING utf8mb4) ELSE dtv.category END,
  dtv.description = CASE
    WHEN dtv.description IS NOT NULL AND dtv.description != '' AND HEX(dtv.description) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(dtv.description USING latin1) USING utf8mb4) ELSE dtv.description END
WHERE (dtv.name IS NOT NULL AND dtv.name != '' AND HEX(dtv.name) NOT LIKE '%E1B%')
   OR (dtv.category IS NOT NULL AND dtv.category != '' AND HEX(dtv.category) NOT LIKE '%E1B%')
   OR (dtv.description IS NOT NULL AND dtv.description != '' AND HEX(dtv.description) NOT LIKE '%E1B%');

UPDATE ad_hoc_proposals
SET
  change_note = CASE
    WHEN change_note IS NOT NULL AND change_note != '' AND HEX(change_note) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(change_note USING latin1) USING utf8mb4) ELSE change_note END,
  reject_reason = CASE
    WHEN reject_reason IS NOT NULL AND reject_reason != '' AND HEX(reject_reason) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(reject_reason USING latin1) USING utf8mb4) ELSE reject_reason END,
  adjustment_note = CASE
    WHEN adjustment_note IS NOT NULL AND adjustment_note != '' AND HEX(adjustment_note) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(adjustment_note USING latin1) USING utf8mb4) ELSE adjustment_note END
WHERE (change_note IS NOT NULL AND change_note != '' AND HEX(change_note) NOT LIKE '%E1B%')
   OR (reject_reason IS NOT NULL AND reject_reason != '' AND HEX(reject_reason) NOT LIKE '%E1B%')
   OR (adjustment_note IS NOT NULL AND adjustment_note != '' AND HEX(adjustment_note) NOT LIKE '%E1B%');

UPDATE global_workflow_steps
SET
  stage_name = CASE
    WHEN stage_name IS NOT NULL AND stage_name != '' AND HEX(stage_name) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(stage_name USING latin1) USING utf8mb4) ELSE stage_name END,
  department = CASE
    WHEN department IS NOT NULL AND department != '' AND HEX(department) NOT LIKE '%E1B%'
    THEN CONVERT(CONVERT(department USING latin1) USING utf8mb4) ELSE department END
WHERE (stage_name IS NOT NULL AND stage_name != '' AND HEX(stage_name) NOT LIKE '%E1B%')
   OR (department IS NOT NULL AND department != '' AND HEX(department) NOT LIKE '%E1B%');
