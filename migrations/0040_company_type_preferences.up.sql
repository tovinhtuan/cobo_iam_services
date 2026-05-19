SET NAMES utf8mb4;

-- company_type_preferences: per-company opt-out for auto-create.
-- No row = default = auto_create_enabled true.
CREATE TABLE IF NOT EXISTS company_type_preferences (
  company_id          VARCHAR(64)  NOT NULL,
  type_id             VARCHAR(64)  NOT NULL,
  auto_create_enabled TINYINT(1)   NOT NULL DEFAULT 1,
  updated_by          VARCHAR(64)  NULL,
  updated_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (company_id, type_id),
  CONSTRAINT fk_ctp_type FOREIGN KEY (type_id)
    REFERENCES disclosure_types(type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
