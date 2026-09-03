SET NAMES utf8mb4;
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci;

-- Frozen delete set membership check via temp list of audited IDs
CREATE TEMPORARY TABLE frozen_delete_roots (
  type_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci PRIMARY KEY
);
INSERT INTO frozen_delete_roots (type_id) VALUES
('bang-tinh-luong-nhan-vien'),
('bang-tinh-luong-nhan-vien-ban-sao'),
('bang-tinh-luong-nhan-vien-thang-ban-sao'),
('bang-tinh-luong-nhan-vien-thang-ban-sao-2'),
('bao-cao-tai-chinh-nam'),
('bao-cao-tai-chinh-quy'),
('bao-cao-tai-chinh-test'),
('bao-cao-tai-chinh-thang'),
('bao-cao-tcq1'),
('bao-cao-test-final'),
('bao-cao-test-final-ban-sao'),
('bao-cao-tuan-test'),
('bao-cao-tuan-test-ban-sao'),
('qa-applicable-to-v1-1788015610625-g1-open'),
('qa-applicable-to-v1-1788015610625-g10-up'),
('qa-applicable-to-v1-1788015610625-g12-end'),
('qa-applicable-to-v1-1788015610625-g15-ext'),
('qa-applicable-to-v1-1788015610625-g2-eq'),
('qa-applicable-to-v1-1788015610625-g3-range'),
('qa-applicable-to-v1-1788015610625-g4-past'),
('qa-applicable-to-v1-1788015610625-g5-bad'),
('qa-applicable-to-v1-1788015610625-g6-split'),
('qa-applicable-to-v1-1788015610625-g7-ver'),
('qa-applicable-to-v1-1788015610625-g9-pres'),
('qa-applicable-to-v1-1788015751009-g1-open'),
('qa-applicable-to-v1-1788015751009-g10-up'),
('qa-applicable-to-v1-1788015751009-g12-end'),
('qa-applicable-to-v1-1788015751009-g15-ext'),
('qa-applicable-to-v1-1788015751009-g2-eq'),
('qa-applicable-to-v1-1788015751009-g3-range'),
('qa-applicable-to-v1-1788015751009-g4-past'),
('qa-applicable-to-v1-1788015751009-g5-bad'),
('qa-applicable-to-v1-1788015751009-g6-split'),
('qa-applicable-to-v1-1788015751009-g7-ver'),
('qa-applicable-to-v1-1788015751009-g9-pres'),
('qa-clone-from-existing-20260821-1108'),
('qa-model-a-financial-20260820-1509'),
('qa-p4-af-clone-1787584187749'),
('qa-p4-af-clone-1787584216798'),
('qa-template-real-user-smoke-20260821-0941'),
('qa-ui-clone-20260821-1115');

SELECT 'frozen_count' AS m, COUNT(*) AS c FROM frozen_delete_roots;
SELECT 'keep_in_delete_set' AS m, COUNT(*) AS c FROM frozen_delete_roots WHERE type_id = @keep_id;
SELECT 'missing_from_db' AS m, COUNT(*) AS c FROM frozen_delete_roots f LEFT JOIN disclosure_types dt ON dt.type_id = f.type_id WHERE dt.type_id IS NULL;
SELECT 'extra_roots_not_in_frozen_or_keep' AS m, COUNT(*) AS c
FROM disclosure_types dt
LEFT JOIN frozen_delete_roots f ON f.type_id = dt.type_id
WHERE dt.type_id <> @keep_id AND f.type_id IS NULL;

SELECT 'would_versions' AS m, COUNT(*) AS c FROM disclosure_type_versions v JOIN frozen_delete_roots f ON f.type_id=v.type_id
UNION ALL SELECT 'would_cycles', COUNT(*) FROM periodic_cycles c JOIN frozen_delete_roots f ON f.type_id=c.type_id
UNION ALL SELECT 'would_records', COUNT(*) FROM disclosure_records r JOIN frozen_delete_roots f ON f.type_id=r.type_id
UNION ALL SELECT 'would_blocks', COUNT(*) FROM disclosure_template_blocks b JOIN frozen_delete_roots f ON f.type_id=b.type_id
UNION ALL SELECT 'would_display_groups', COUNT(*) FROM template_display_groups d JOIN frozen_delete_roots f ON f.type_id=d.template_id
UNION ALL SELECT 'would_prefs', COUNT(*) FROM company_type_preferences p JOIN frozen_delete_roots f ON f.type_id=p.type_id
UNION ALL SELECT 'would_overrides', COUNT(*) FROM company_template_workflow_overrides o JOIN frozen_delete_roots f ON f.type_id=o.type_id
UNION ALL SELECT 'would_alert_cfgs', COUNT(*) FROM alert_template_configs a JOIN frozen_delete_roots f ON f.type_id=a.type_id
UNION ALL SELECT 'would_gwf', COUNT(*) FROM global_workflows g JOIN frozen_delete_roots f ON f.type_id=g.type_id
UNION ALL SELECT 'would_gwv', COUNT(*) FROM global_workflow_versions g JOIN frozen_delete_roots f ON f.type_id=g.type_id
UNION ALL SELECT 'would_adhoc', COUNT(*) FROM ad_hoc_proposals a JOIN frozen_delete_roots f ON f.type_id=a.type_id
UNION ALL SELECT 'would_woc', COUNT(*) FROM workflow_override_conflicts w JOIN frozen_delete_roots f ON f.type_id=w.type_id
UNION ALL SELECT 'would_wf_inst', COUNT(DISTINCT wi.workflow_instance_id)
FROM workflow_instances wi JOIN disclosure_records r ON r.record_id=wi.record_id JOIN frozen_delete_roots f ON f.type_id=r.type_id
UNION ALL SELECT 'would_wf_tasks', COUNT(*)
FROM workflow_tasks wt JOIN workflow_instances wi ON wi.workflow_instance_id=wt.workflow_instance_id
JOIN disclosure_records r ON r.record_id=wi.record_id JOIN frozen_delete_roots f ON f.type_id=r.type_id
UNION ALL SELECT 'would_dac', COUNT(*)
FROM deadline_alert_confirmations dac JOIN disclosure_records r ON r.record_id=dac.record_id JOIN frozen_delete_roots f ON f.type_id=r.type_id
UNION ALL SELECT 'non_draft', COUNT(*) FROM disclosure_records r JOIN frozen_delete_roots f ON f.type_id=r.type_id WHERE r.status <> 'draft'
UNION ALL SELECT 'submitted', COUNT(*) FROM disclosure_records r JOIN frozen_delete_roots f ON f.type_id=r.type_id WHERE r.submitted_at IS NOT NULL;

-- FK list to disclosure_types
SELECT TABLE_NAME, CONSTRAINT_NAME, DELETE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA='cobo_iam' AND REFERENCED_TABLE_NAME='disclosure_types'
ORDER BY TABLE_NAME;

SELECT TABLE_NAME, CONSTRAINT_NAME, DELETE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA='cobo_iam' AND REFERENCED_TABLE_NAME='disclosure_type_versions'
ORDER BY TABLE_NAME;

-- companies/users baseline counts (global master)
SELECT 'companies' AS m, COUNT(*) AS c FROM companies
UNION ALL SELECT 'users', COUNT(*) FROM users
UNION ALL SELECT 'departments', COUNT(*) FROM departments
UNION ALL SELECT 'roles', COUNT(*) FROM roles;
