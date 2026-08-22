-- Purpose-scoped workflow document template files (sample/biểu mẫu).
-- Separate from cms_media_assets so Office MIME + Company ACL do not widen global CMS media.
CREATE TABLE IF NOT EXISTS workflow_document_template_assets (
  file_id VARCHAR(64) NOT NULL PRIMARY KEY,
  owner_scope ENUM('cms', 'company') NOT NULL,
  company_id VARCHAR(64) NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  content_type VARCHAR(128) NOT NULL,
  size_bytes BIGINT NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  state ENUM('ready', 'deleted') NOT NULL DEFAULT 'ready',
  created_by VARCHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  UNIQUE KEY uq_wf_doc_tpl_object_key (object_key),
  KEY idx_wf_doc_tpl_company_scope (company_id, owner_scope, state),
  KEY idx_wf_doc_tpl_created (created_at)
);
