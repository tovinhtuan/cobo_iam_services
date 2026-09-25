-- 0144: live workflow department binding. Expand-only. Flag-off code does not read these tables.

CREATE TABLE IF NOT EXISTS workflow_template_department_code_registry (
  department_code VARCHAR(64) NOT NULL PRIMARY KEY,
  first_seen_at   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  retired_at      DATETIME(3) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO workflow_template_department_code_registry (department_code)
SELECT department_code FROM workflow_template_departments
ON DUPLICATE KEY UPDATE department_code = workflow_template_department_code_registry.department_code;

CREATE TABLE IF NOT EXISTS company_workflow_department_mappings (
  mapping_id               CHAR(36)     NOT NULL,
  company_id               VARCHAR(36)  NOT NULL,
  template_department_code VARCHAR(64)  NOT NULL,
  disclosure_type_id       VARCHAR(64)  NOT NULL DEFAULT '',
  step_code                VARCHAR(64)  NOT NULL DEFAULT '',
  company_department_id    VARCHAR(36)  NOT NULL,
  status                   VARCHAR(16)  NOT NULL,
  effective_from           DATETIME(3)  NOT NULL,
  effective_to             DATETIME(3)  NULL,
  version                  INT          NOT NULL,
  source                   VARCHAR(32)  NOT NULL,
  backfill_batch_id        CHAR(36)     NULL,
  scope_key_hash           BINARY(32)   NOT NULL,
  open_scope_hash          BINARY(32)   NULL,
  created_by               VARCHAR(36)  NOT NULL,
  updated_by               VARCHAR(36)  NOT NULL,
  created_at               DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at               DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (mapping_id),
  UNIQUE KEY uk_wf_dept_map_version (company_id, template_department_code, disclosure_type_id, step_code, version),
  UNIQUE KEY uk_wf_dept_map_open (open_scope_hash),
  KEY idx_wf_dept_map_company_code (company_id, template_department_code, effective_from),
  CONSTRAINT fk_wf_dept_map_company FOREIGN KEY (company_id) REFERENCES companies(company_id),
  CONSTRAINT fk_wf_dept_map_dept FOREIGN KEY (company_department_id) REFERENCES departments(department_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reminder_dispatch_resolutions (
  resolution_id                     CHAR(36)     NOT NULL,
  occurrence_id                     VARCHAR(80)  NOT NULL,
  company_id                        VARCHAR(36)  NOT NULL,
  workflow_instance_id              VARCHAR(36)  NULL,
  disclosure_id                     VARCHAR(64)  NULL,
  step_code                         VARCHAR(64)  NULL,
  workflow_token                    VARCHAR(128) NULL,
  mapping_id                        CHAR(36)     NULL,
  mapping_version                   INT          NULL,
  resolved_department_id            VARCHAR(36)  NULL,
  resolved_department_name_snapshot VARCHAR(255) NULL,
  match_path                        VARCHAR(64)  NULL,
  resolution_status                 VARCHAR(32)  NULL,
  recipient_type                    VARCHAR(32)  NULL,
  recipient_membership_ids          JSON         NULL,
  recipient_email_ciphertext        BLOB         NULL,
  email_key_version                 INT UNSIGNED NULL,
  resolved_at                       DATETIME(3)  NOT NULL,
  send_status                       VARCHAR(32)  NOT NULL,
  provider_message_id               VARCHAR(255) NULL,
  PRIMARY KEY (resolution_id),
  UNIQUE KEY uk_reminder_dispatch_occurrence (occurrence_id),
  KEY idx_reminder_dispatch_send (send_status, resolved_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Single-statement trigger so the migrator can execute this file without DELIMITER.
-- Updating department_code is the primary key; application inserts only, and a
-- retired registry row rejects reuse. Display name updates remain allowed.
DROP TRIGGER IF EXISTS trg_wtd_no_code_delete;

CREATE TRIGGER trg_wtd_no_code_delete
BEFORE DELETE ON workflow_template_departments
FOR EACH ROW
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'catalog department code cannot be deleted';
