-- Optional bilingual display labels persisted alongside legacy `title` (canonical UI storage).
ALTER TABLE disclosure_template_blocks
  ADD COLUMN name_en VARCHAR(255) NULL DEFAULT NULL AFTER title,
  ADD COLUMN name_vi VARCHAR(255) NULL DEFAULT NULL AFTER name_en;

-- Backfill Vietnamese label from existing title.
UPDATE disclosure_template_blocks
SET name_vi = title
WHERE (name_vi IS NULL OR TRIM(name_vi) = '')
  AND title IS NOT NULL
  AND TRIM(title) <> '';

-- Backfill English labels for known mandatory keys (non-mandatory / extras stay NULL until written).
UPDATE disclosure_template_blocks
SET name_en = CASE block_key
    WHEN 'legal_basis' THEN 'Legal basis'
    WHEN 'disclosure_content' THEN 'Disclosure and reporting content'
    WHEN 'deadline' THEN 'Disclosure/reporting deadline'
    WHEN 'channels_and_format' THEN 'Channels and format'
    WHEN 'legal_risks' THEN 'Legal risks if requirements are not met'
    WHEN 'enterprise_workflow' THEN 'Enterprise workflow'
    ELSE name_en
  END
WHERE block_key IN (
    'legal_basis',
    'disclosure_content',
    'deadline',
    'channels_and_format',
    'legal_risks',
    'enterprise_workflow'
)
  AND (name_en IS NULL OR TRIM(name_en) = '');
