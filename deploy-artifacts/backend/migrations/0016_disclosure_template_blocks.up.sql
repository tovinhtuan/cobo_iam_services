CREATE TABLE IF NOT EXISTS disclosure_template_blocks (
    type_id VARCHAR(100) NOT NULL,
    version_no INT NOT NULL,
    block_id VARCHAR(64) NOT NULL,
    block_key VARCHAR(100) NOT NULL,
    block_type VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NULL,
    config_json JSON NOT NULL,
    validation_json JSON NOT NULL,
    display_order INT NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (type_id, version_no, block_id),
    CONSTRAINT fk_disclosure_template_blocks_version
        FOREIGN KEY (type_id, version_no)
        REFERENCES disclosure_type_versions(type_id, version_no)
        ON DELETE CASCADE,
    CONSTRAINT uq_disclosure_template_blocks_key UNIQUE (type_id, version_no, block_key),
    CONSTRAINT uq_disclosure_template_blocks_order UNIQUE (type_id, version_no, display_order),
    INDEX idx_disclosure_template_blocks_lookup (type_id, version_no, display_order)
);
