-- Deactivate active company overrides whose active version has an empty workflow snapshot.
-- Preserves version history; only clears active_version_no so runtime falls back to global/template.
-- Safe repair for legacy rows created before activation validation (WORKFLOW_OVERRIDE_EMPTY guard).

UPDATE company_template_workflow_overrides o
INNER JOIN company_template_workflow_override_versions v
  ON v.override_id = o.override_id AND v.version_no = o.active_version_no
SET
  o.active_version_no = 0,
  o.status = 'archived',
  o.updated_at = UTC_TIMESTAMP()
WHERE o.active_version_no > 0
  AND (
    v.workflow_json IS NULL
    OR JSON_LENGTH(v.workflow_json) = 0
    OR v.workflow_json = JSON_ARRAY()
  );
