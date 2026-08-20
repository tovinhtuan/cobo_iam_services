-- Model A: the released disclosure template version owns the CMS workflow authority.
-- Expand-only migration: legacy readers/writers remain compatible during backfill.

ALTER TABLE disclosure_type_versions
  ADD COLUMN workflow_authority_mode VARCHAR(32) NOT NULL DEFAULT 'LEGACY_COMPAT' AFTER is_released,
  ADD COLUMN workflow_manifest_json JSON NULL AFTER workflow_authority_mode,
  ADD COLUMN workflow_manifest_schema_version SMALLINT NULL AFTER workflow_manifest_json,
  ADD COLUMN workflow_source VARCHAR(64) NULL AFTER workflow_manifest_schema_version,
  ADD COLUMN workflow_source_version_no INT NULL AFTER workflow_source,
  ADD COLUMN workflow_semantic_hash CHAR(64) NULL AFTER workflow_source_version_no,
  ADD COLUMN publication_candidate_hash CHAR(64) NULL AFTER workflow_semantic_hash,
  ADD KEY idx_dtv_workflow_authority (workflow_authority_mode, type_id, version_no);

ALTER TABLE global_workflow_versions
  ADD COLUMN template_version_no INT NULL AFTER version_no,
  ADD KEY idx_gwv_template_version (type_id, template_version_no);
