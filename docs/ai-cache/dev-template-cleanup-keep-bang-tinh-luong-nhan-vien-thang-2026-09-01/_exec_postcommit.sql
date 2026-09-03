SET NAMES utf8mb4;
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci;

SELECT 'roots' AS m, COUNT(*) AS c FROM disclosure_types;
SELECT dt.type_id, dt.status, dt.active_version_no, dtv.name
FROM disclosure_types dt
JOIN disclosure_type_versions dtv ON dtv.type_id=dt.type_id AND dtv.version_no=dt.active_version_no;

SELECT 'keep_versions' AS m, COUNT(*) AS c FROM disclosure_type_versions WHERE type_id COLLATE utf8mb4_unicode_ci=@keep_id;
SELECT 'keep_cycles' AS m, COUNT(*) AS c FROM periodic_cycles WHERE type_id COLLATE utf8mb4_unicode_ci=@keep_id;
SELECT 'keep_records' AS m, COUNT(*) AS c FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci=@keep_id;
SELECT 'keep_blocks' AS m, COUNT(*) AS c FROM disclosure_template_blocks WHERE type_id COLLATE utf8mb4_unicode_ci=@keep_id;
SELECT 'keep_dg' AS m, COUNT(*) AS c FROM template_display_groups WHERE template_id COLLATE utf8mb4_unicode_ci=@keep_id;

-- orphan check against frozen IDs (should be 0)
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

SELECT 'orphan_versions' AS m, COUNT(*) AS c FROM disclosure_type_versions v INNER JOIN frozen_delete_roots f ON f.type_id=v.type_id;
SELECT 'orphan_cycles' AS m, COUNT(*) AS c FROM periodic_cycles c INNER JOIN frozen_delete_roots f ON f.type_id=c.type_id;
SELECT 'orphan_records' AS m, COUNT(*) AS c FROM disclosure_records r INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id;
SELECT 'orphan_blocks' AS m, COUNT(*) AS c FROM disclosure_template_blocks b INNER JOIN frozen_delete_roots f ON f.type_id=b.type_id;
SELECT 'orphan_prefs' AS m, COUNT(*) AS c FROM company_type_preferences p INNER JOIN frozen_delete_roots f ON f.type_id=p.type_id;
SELECT 'orphan_overrides' AS m, COUNT(*) AS c FROM company_template_workflow_overrides o INNER JOIN frozen_delete_roots f ON f.type_id=o.type_id;
SELECT 'orphan_alert' AS m, COUNT(*) AS c FROM alert_template_configs a INNER JOIN frozen_delete_roots f ON f.type_id=a.type_id;
SELECT 'orphan_gwf' AS m, COUNT(*) AS c FROM global_workflows g INNER JOIN frozen_delete_roots f ON f.type_id=g.type_id;
SELECT 'orphan_gwv' AS m, COUNT(*) AS c FROM global_workflow_versions g INNER JOIN frozen_delete_roots f ON f.type_id=g.type_id;
SELECT 'orphan_dg' AS m, COUNT(*) AS c FROM template_display_groups d INNER JOIN frozen_delete_roots f ON f.type_id=d.template_id;
SELECT 'companies' AS m, COUNT(*) AS c FROM companies;
SELECT 'users' AS m, COUNT(*) AS c FROM users;
SELECT 'departments' AS m, COUNT(*) AS c FROM departments;
SELECT 'roles' AS m, COUNT(*) AS c FROM roles;

-- KEEP identity continuity
SELECT version_no FROM disclosure_type_versions WHERE type_id COLLATE utf8mb4_unicode_ci=@keep_id;
SELECT cycle_id FROM periodic_cycles WHERE type_id COLLATE utf8mb4_unicode_ci=@keep_id ORDER BY cycle_id;
SELECT record_id, status FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci=@keep_id ORDER BY record_id;
SELECT display_group_code FROM template_display_groups WHERE template_id COLLATE utf8mb4_unicode_ci=@keep_id ORDER BY display_group_code;
