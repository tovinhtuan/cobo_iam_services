ALTER TABLE global_workflow_versions
  DROP KEY idx_gwv_template_version,
  DROP COLUMN template_version_no;

ALTER TABLE disclosure_type_versions
  DROP KEY idx_dtv_workflow_authority,
  DROP COLUMN publication_candidate_hash,
  DROP COLUMN workflow_semantic_hash,
  DROP COLUMN workflow_source_version_no,
  DROP COLUMN workflow_source,
  DROP COLUMN workflow_manifest_schema_version,
  DROP COLUMN workflow_manifest_json,
  DROP COLUMN workflow_authority_mode;
