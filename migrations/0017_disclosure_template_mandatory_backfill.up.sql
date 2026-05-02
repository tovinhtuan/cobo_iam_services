-- Backfill disclosure_template_blocks for existing disclosure_type_versions rows
-- that are missing mandatory block_key rows. Descriptions are taken from the
-- legacy flat columns (same mapping as ApplyTemplateFlatBlockSync in app code).
-- block_id uses prefix mb- + MD5(...) so down migration can remove these rows safely.

SET NAMES utf8mb4;

INSERT INTO disclosure_template_blocks (
    type_id, version_no, block_id, block_key, block_type, title, description,
    config_json, validation_json, display_order, enabled
)
SELECT
    v.type_id,
    v.version_no,
    CONCAT('mb-', MD5(CONCAT(v.type_id, ':', CAST(v.version_no AS CHAR), ':legal_basis'))),
    'legal_basis',
    'rich_text',
    'Cơ sở pháp lý',
    NULLIF(TRIM(COALESCE(v.legal_basis, '')), ''),
    JSON_OBJECT('max_length', 8000, 'allow_html', FALSE),
    CAST('{}' AS JSON),
    (SELECT COALESCE(MAX(b.display_order), 0) + 1
     FROM disclosure_template_blocks b
     WHERE b.type_id = v.type_id AND b.version_no = v.version_no),
    1
FROM disclosure_type_versions v
WHERE NOT EXISTS (
    SELECT 1 FROM disclosure_template_blocks x
    WHERE x.type_id = v.type_id AND x.version_no = v.version_no AND x.block_key = 'legal_basis'
);

INSERT INTO disclosure_template_blocks (
    type_id, version_no, block_id, block_key, block_type, title, description,
    config_json, validation_json, display_order, enabled
)
SELECT
    v.type_id,
    v.version_no,
    CONCAT('mb-', MD5(CONCAT(v.type_id, ':', CAST(v.version_no AS CHAR), ':disclosure_content'))),
    'disclosure_content',
    'rich_text',
    'Nội dung công bố/báo cáo',
    NULLIF(TRIM(COALESCE(v.report_content, '')), ''),
    JSON_OBJECT('max_length', 50000, 'allow_html', TRUE),
    CAST('{}' AS JSON),
    (SELECT COALESCE(MAX(b.display_order), 0) + 1
     FROM disclosure_template_blocks b
     WHERE b.type_id = v.type_id AND b.version_no = v.version_no),
    1
FROM disclosure_type_versions v
WHERE NOT EXISTS (
    SELECT 1 FROM disclosure_template_blocks x
    WHERE x.type_id = v.type_id AND x.version_no = v.version_no AND x.block_key = 'disclosure_content'
);

INSERT INTO disclosure_template_blocks (
    type_id, version_no, block_id, block_key, block_type, title, description,
    config_json, validation_json, display_order, enabled
)
SELECT
    v.type_id,
    v.version_no,
    CONCAT('mb-', MD5(CONCAT(v.type_id, ':', CAST(v.version_no AS CHAR), ':deadline'))),
    'deadline',
    'text',
    'Kỳ hạn công bố/báo cáo',
    NULLIF(TRIM(COALESCE(v.deadline_rule, '')), ''),
    JSON_OBJECT('max_length', 4000),
    CAST('{}' AS JSON),
    (SELECT COALESCE(MAX(b.display_order), 0) + 1
     FROM disclosure_template_blocks b
     WHERE b.type_id = v.type_id AND b.version_no = v.version_no),
    1
FROM disclosure_type_versions v
WHERE NOT EXISTS (
    SELECT 1 FROM disclosure_template_blocks x
    WHERE x.type_id = v.type_id AND x.version_no = v.version_no AND x.block_key = 'deadline'
);

INSERT INTO disclosure_template_blocks (
    type_id, version_no, block_id, block_key, block_type, title, description,
    config_json, validation_json, display_order, enabled
)
SELECT
    v.type_id,
    v.version_no,
    CONCAT('mb-', MD5(CONCAT(v.type_id, ':', CAST(v.version_no AS CHAR), ':channels_and_format'))),
    'channels_and_format',
    'rich_text',
    'Kênh và hình thức công bố/báo cáo',
    NULLIF(TRIM(CONCAT(
        IF(TRIM(COALESCE(v.channels_text, '')) = '', '', TRIM(v.channels_text)),
        IF(TRIM(COALESCE(v.channels_text, '')) <> '' AND TRIM(COALESCE(v.format, '')) <> '', CHAR(10), ''),
        IF(TRIM(COALESCE(v.format, '')) = '', '', CONCAT('Format: ', TRIM(v.format)))
    )), ''),
    JSON_OBJECT('max_length', 12000, 'allow_html', FALSE),
    CAST('{}' AS JSON),
    (SELECT COALESCE(MAX(b.display_order), 0) + 1
     FROM disclosure_template_blocks b
     WHERE b.type_id = v.type_id AND b.version_no = v.version_no),
    1
FROM disclosure_type_versions v
WHERE NOT EXISTS (
    SELECT 1 FROM disclosure_template_blocks x
    WHERE x.type_id = v.type_id AND x.version_no = v.version_no AND x.block_key = 'channels_and_format'
);

INSERT INTO disclosure_template_blocks (
    type_id, version_no, block_id, block_key, block_type, title, description,
    config_json, validation_json, display_order, enabled
)
SELECT
    v.type_id,
    v.version_no,
    CONCAT('mb-', MD5(CONCAT(v.type_id, ':', CAST(v.version_no AS CHAR), ':legal_risks'))),
    'legal_risks',
    'rich_text',
    'Rủi ro pháp lý nếu không thực hiện đúng',
    NULLIF(TRIM(COALESCE(v.legal_risks_text, '')), ''),
    JSON_OBJECT('max_length', 8000, 'allow_html', FALSE),
    CAST('{}' AS JSON),
    (SELECT COALESCE(MAX(b.display_order), 0) + 1
     FROM disclosure_template_blocks b
     WHERE b.type_id = v.type_id AND b.version_no = v.version_no),
    1
FROM disclosure_type_versions v
WHERE NOT EXISTS (
    SELECT 1 FROM disclosure_template_blocks x
    WHERE x.type_id = v.type_id AND x.version_no = v.version_no AND x.block_key = 'legal_risks'
);

INSERT INTO disclosure_template_blocks (
    type_id, version_no, block_id, block_key, block_type, title, description,
    config_json, validation_json, display_order, enabled
)
SELECT
    v.type_id,
    v.version_no,
    CONCAT('mb-', MD5(CONCAT(v.type_id, ':', CAST(v.version_no AS CHAR), ':enterprise_workflow'))),
    'enterprise_workflow',
    'rich_text',
    'Workflow của doanh nghiệp',
    NULLIF(TRIM(COALESCE(v.implementation_content, '')), ''),
    JSON_OBJECT('max_length', 12000, 'allow_html', TRUE),
    CAST('{}' AS JSON),
    (SELECT COALESCE(MAX(b.display_order), 0) + 1
     FROM disclosure_template_blocks b
     WHERE b.type_id = v.type_id AND b.version_no = v.version_no),
    1
FROM disclosure_type_versions v
WHERE NOT EXISTS (
    SELECT 1 FROM disclosure_template_blocks x
    WHERE x.type_id = v.type_id AND x.version_no = v.version_no AND x.block_key = 'enterprise_workflow'
);
