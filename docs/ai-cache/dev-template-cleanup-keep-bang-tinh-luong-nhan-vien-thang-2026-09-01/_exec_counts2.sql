SET NAMES utf8mb4;
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci;

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

SELECT 'would_versions' AS m, COUNT(*) AS c FROM disclosure_type_versions v INNER JOIN frozen_delete_roots f ON f.type_id=v.type_id;
SELECT 'would_cycles' AS m, COUNT(*) AS c FROM periodic_cycles c INNER JOIN frozen_delete_roots f ON f.type_id=c.type_id;
SELECT 'would_records' AS m, COUNT(*) AS c FROM disclosure_records r INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id;
SELECT 'would_blocks' AS m, COUNT(*) AS c FROM disclosure_template_blocks b INNER JOIN frozen_delete_roots f ON f.type_id=b.type_id;
SELECT 'would_display_groups' AS m, COUNT(*) AS c FROM template_display_groups d INNER JOIN frozen_delete_roots f ON f.type_id=d.template_id;
SELECT 'would_prefs' AS m, COUNT(*) AS c FROM company_type_preferences p INNER JOIN frozen_delete_roots f ON f.type_id=p.type_id;
SELECT 'would_overrides' AS m, COUNT(*) AS c FROM company_template_workflow_overrides o INNER JOIN frozen_delete_roots f ON f.type_id=o.type_id;
SELECT 'would_override_versions' AS m, COUNT(*) AS c FROM company_template_workflow_override_versions ov INNER JOIN company_template_workflow_overrides o ON o.override_id=ov.override_id INNER JOIN frozen_delete_roots f ON f.type_id=o.type_id;
SELECT 'would_alert_cfgs' AS m, COUNT(*) AS c FROM alert_template_configs a INNER JOIN frozen_delete_roots f ON f.type_id=a.type_id;
SELECT 'would_gwf' AS m, COUNT(*) AS c FROM global_workflows g INNER JOIN frozen_delete_roots f ON f.type_id=g.type_id;
SELECT 'would_gwv' AS m, COUNT(*) AS c FROM global_workflow_versions g INNER JOIN frozen_delete_roots f ON f.type_id=g.type_id;
SELECT 'would_adhoc' AS m, COUNT(*) AS c FROM ad_hoc_proposals a INNER JOIN frozen_delete_roots f ON f.type_id=a.type_id;
SELECT 'would_woc' AS m, COUNT(*) AS c FROM workflow_override_conflicts w INNER JOIN frozen_delete_roots f ON f.type_id=w.type_id;
SELECT 'would_wf_inst' AS m, COUNT(DISTINCT wi.workflow_instance_id) AS c FROM workflow_instances wi INNER JOIN disclosure_records r ON r.record_id=wi.record_id INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id;
SELECT 'would_wf_tasks' AS m, COUNT(*) AS c FROM workflow_tasks wt INNER JOIN workflow_instances wi ON wi.workflow_instance_id=wt.workflow_instance_id INNER JOIN disclosure_records r ON r.record_id=wi.record_id INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id;
SELECT 'would_dac' AS m, COUNT(*) AS c FROM deadline_alert_confirmations dac INNER JOIN disclosure_records r ON r.record_id=dac.record_id INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id;
SELECT 'non_draft' AS m, COUNT(*) AS c FROM disclosure_records r INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id WHERE r.status <> 'draft';
SELECT 'submitted' AS m, COUNT(*) AS c FROM disclosure_records r INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id WHERE r.submitted_at IS NOT NULL;

SELECT TABLE_NAME, CONSTRAINT_NAME, DELETE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA='cobo_iam' AND REFERENCED_TABLE_NAME='disclosure_types'
ORDER BY TABLE_NAME;

SELECT TABLE_NAME, CONSTRAINT_NAME, DELETE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA='cobo_iam' AND REFERENCED_TABLE_NAME='disclosure_type_versions'
ORDER BY TABLE_NAME;

SELECT 'companies' AS m, COUNT(*) AS c FROM companies;
SELECT 'users' AS m, COUNT(*) AS c FROM users;
SELECT 'departments' AS m, COUNT(*) AS c FROM departments;
SELECT 'roles' AS m, COUNT(*) AS c FROM roles;
