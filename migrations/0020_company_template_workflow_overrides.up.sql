CREATE TABLE IF NOT EXISTS company_template_workflow_overrides (
    override_id VARCHAR(64) PRIMARY KEY,
    company_id VARCHAR(64) NOT NULL,
    type_id VARCHAR(64) NOT NULL,
    active_version_no INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_by VARCHAR(64) NOT NULL,
    updated_by VARCHAR(64) NOT NULL,
    approved_by VARCHAR(64) NULL,
    approved_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uq_company_template_workflow_override UNIQUE (company_id, type_id),
    INDEX idx_ctwo_company_status (company_id, status),
    INDEX idx_ctwo_type (type_id),
    CONSTRAINT fk_ctwo_type FOREIGN KEY (type_id) REFERENCES disclosure_types(type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS company_template_workflow_override_versions (
    override_id VARCHAR(64) NOT NULL,
    version_no INT NOT NULL,
    workflow_json JSON NOT NULL,
    change_note TEXT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_by VARCHAR(64) NOT NULL,
    approved_by VARCHAR(64) NULL,
    approved_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (override_id, version_no),
    INDEX idx_ctwov_override_state (override_id, state, version_no),
    CONSTRAINT fk_ctwov_override FOREIGN KEY (override_id)
        REFERENCES company_template_workflow_overrides(override_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
