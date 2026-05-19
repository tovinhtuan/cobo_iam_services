SET NAMES utf8mb4;

-- periodic_cycles: one row per (type, company, cycle_label).
-- UNIQUE KEY ensures idempotent upsert across worker ticks.
-- record_id NULL = pending materialization; non-NULL = disclosure record already created.
CREATE TABLE IF NOT EXISTS periodic_cycles (
  cycle_id        VARCHAR(64)   NOT NULL,
  type_id         VARCHAR(64)   NOT NULL,
  company_id      VARCHAR(64)   NOT NULL,
  cycle_label     VARCHAR(64)   NOT NULL,  -- "2026-05" | "2026-Q2" | "2026"
  due_date        DATE          NOT NULL,
  record_id       VARCHAR(64)   NULL,
  materialized_at DATETIME(3)   NULL,
  created_at      DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (cycle_id),
  UNIQUE KEY uq_pc_type_company_label (type_id, company_id, cycle_label),
  KEY idx_pc_pending (record_id, due_date),
  KEY idx_pc_company_type (company_id, type_id),
  CONSTRAINT fk_pc_type FOREIGN KEY (type_id)
    REFERENCES disclosure_types(type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
