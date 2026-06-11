SET NAMES utf8mb4;

-- Backfill global active template versions: all 3 company classes + all 3 sectors.
-- Periodic templates also get default structure deadlines (30/30/20).
UPDATE disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
SET v.applicability_rules_json = CASE
  WHEN LOWER(TRIM(v.template_category)) = 'periodic' THEN JSON_OBJECT(
    'applicable_company_classes', JSON_ARRAY('listed', 'large_public', 'non_large_public'),
    'applicable_sectors', JSON_ARRAY('commercial', 'service', 'manufacturing'),
    'deadline_by_structure', JSON_OBJECT(
      'has_subsidiaries', JSON_OBJECT('days', 30),
      'has_subordinate_units', JSON_OBJECT('days', 30),
      'simple_structure', JSON_OBJECT('days', 20)
    )
  )
  ELSE JSON_OBJECT(
    'applicable_company_classes', JSON_ARRAY('listed', 'large_public', 'non_large_public'),
    'applicable_sectors', JSON_ARRAY('commercial', 'service', 'manufacturing')
  )
END
WHERE t.company_id IS NULL
  AND t.active_version_no > 0
  AND v.applicability_rules_json IS NULL;
