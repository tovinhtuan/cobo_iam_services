-- DEV template hard-delete — single-session transactional cleanup
-- KEEP: bang-tinh-luong-nhan-vien-ban-sao-2
SET NAMES utf8mb4;
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci;

-- Master data baselines (for post-check)
SELECT COUNT(*) INTO @companies_before FROM companies;
SELECT COUNT(*) INTO @users_before FROM users;
SELECT COUNT(*) INTO @departments_before FROM departments;
SELECT COUNT(*) INTO @roles_before FROM roles;

-- KEEP baselines
SELECT COUNT(*) INTO @keep_versions_before FROM disclosure_type_versions WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_cycles_before FROM periodic_cycles WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_records_before FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_blocks_before FROM disclosure_template_blocks WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_dg_before FROM template_display_groups WHERE template_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT status, active_version_no INTO @keep_status_before, @keep_av_before
FROM disclosure_types WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;

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

SELECT COUNT(*) INTO @frozen_cnt FROM frozen_delete_roots;
SELECT COUNT(*) INTO @keep_in_set FROM frozen_delete_roots WHERE type_id = @keep_id;

-- Expected business deletes
SELECT COUNT(*) INTO @exp_non_draft FROM disclosure_records r INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id WHERE r.status <> 'draft';
SELECT COUNT(*) INTO @exp_submitted FROM disclosure_records r INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id WHERE r.submitted_at IS NOT NULL;

START TRANSACTION;

-- Gate: KEEP not in delete set; frozen size 41; KEEP baselines match audit
SET @gate_pre = IF(@frozen_cnt = 41 AND @keep_in_set = 0
  AND @keep_versions_before = 1 AND @keep_cycles_before = 8 AND @keep_records_before = 8
  AND @keep_blocks_before = 6 AND @keep_dg_before = 2
  AND @keep_status_before = 'active' AND @keep_av_before = 1
  AND @exp_non_draft = 6 AND @exp_submitted = 1, 1, 0);

-- Child-first deletes (scoped to frozen roots only)
DELETE wt FROM workflow_tasks wt
INNER JOIN workflow_instances wi ON wi.workflow_instance_id = wt.workflow_instance_id
INNER JOIN disclosure_records r ON r.record_id = wi.record_id
INNER JOIN frozen_delete_roots f ON f.type_id = r.type_id;
SET @del_wf_tasks = ROW_COUNT();

DELETE wi FROM workflow_instances wi
INNER JOIN disclosure_records r ON r.record_id = wi.record_id
INNER JOIN frozen_delete_roots f ON f.type_id = r.type_id;
SET @del_wf_inst = ROW_COUNT();

DELETE dac FROM deadline_alert_confirmations dac
INNER JOIN disclosure_records r ON r.record_id = dac.record_id
INNER JOIN frozen_delete_roots f ON f.type_id = r.type_id;
SET @del_dac = ROW_COUNT();

DELETE r FROM disclosure_records r
INNER JOIN frozen_delete_roots f ON f.type_id = r.type_id;
SET @del_records = ROW_COUNT();

DELETE c FROM periodic_cycles c
INNER JOIN frozen_delete_roots f ON f.type_id = c.type_id;
SET @del_cycles = ROW_COUNT();

DELETE ov FROM company_template_workflow_override_versions ov
INNER JOIN company_template_workflow_overrides o ON o.override_id = ov.override_id
INNER JOIN frozen_delete_roots f ON f.type_id = o.type_id;
SET @del_ov = ROW_COUNT();

DELETE o FROM company_template_workflow_overrides o
INNER JOIN frozen_delete_roots f ON f.type_id = o.type_id;
SET @del_overrides = ROW_COUNT();

DELETE p FROM company_type_preferences p
INNER JOIN frozen_delete_roots f ON f.type_id = p.type_id;
SET @del_prefs = ROW_COUNT();

DELETE a FROM alert_template_configs a
INNER JOIN frozen_delete_roots f ON f.type_id = a.type_id;
SET @del_alert = ROW_COUNT();

DELETE b FROM disclosure_template_blocks b
INNER JOIN frozen_delete_roots f ON f.type_id = b.type_id;
SET @del_blocks = ROW_COUNT();

DELETE gws FROM global_workflow_steps gws
INNER JOIN global_workflows gw ON gw.workflow_id = gws.workflow_id
INNER JOIN frozen_delete_roots f ON f.type_id = gw.type_id;
SET @del_gws = ROW_COUNT();

DELETE gwv FROM global_workflow_versions gwv
INNER JOIN frozen_delete_roots f ON f.type_id = gwv.type_id;
SET @del_gwv = ROW_COUNT();

DELETE gw FROM global_workflows gw
INNER JOIN frozen_delete_roots f ON f.type_id = gw.type_id;
SET @del_gwf = ROW_COUNT();

DELETE d FROM template_display_groups d
INNER JOIN frozen_delete_roots f ON f.type_id = d.template_id;
SET @del_dg = ROW_COUNT();

DELETE a FROM ad_hoc_proposals a
INNER JOIN frozen_delete_roots f ON f.type_id = a.type_id;
SET @del_adhoc = ROW_COUNT();

DELETE w FROM workflow_override_conflicts w
INNER JOIN frozen_delete_roots f ON f.type_id = w.type_id;
SET @del_woc = ROW_COUNT();

DELETE v FROM disclosure_type_versions v
INNER JOIN frozen_delete_roots f ON f.type_id = v.type_id;
SET @del_versions = ROW_COUNT();

DELETE dt FROM disclosure_types dt
INNER JOIN frozen_delete_roots f ON f.type_id = dt.type_id;
SET @del_roots = ROW_COUNT();

-- PRECOMMIT verification
SELECT COUNT(*) INTO @root_cnt FROM disclosure_types;
SELECT COUNT(*) INTO @keep_present FROM disclosure_types WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT status, active_version_no INTO @keep_status_after, @keep_av_after
FROM disclosure_types WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_versions_after FROM disclosure_type_versions WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_cycles_after FROM periodic_cycles WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_records_after FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_blocks_after FROM disclosure_template_blocks WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @keep_dg_after FROM template_display_groups WHERE template_id COLLATE utf8mb4_unicode_ci = @keep_id;
SELECT COUNT(*) INTO @delete_root_remaining FROM disclosure_types dt INNER JOIN frozen_delete_roots f ON f.type_id = dt.type_id;

-- Orphans for deleted roots
SELECT COUNT(*) INTO @orphan_versions FROM disclosure_type_versions v INNER JOIN frozen_delete_roots f ON f.type_id=v.type_id;
SELECT COUNT(*) INTO @orphan_cycles FROM periodic_cycles c INNER JOIN frozen_delete_roots f ON f.type_id=c.type_id;
SELECT COUNT(*) INTO @orphan_records FROM disclosure_records r INNER JOIN frozen_delete_roots f ON f.type_id=r.type_id;
SELECT COUNT(*) INTO @orphan_blocks FROM disclosure_template_blocks b INNER JOIN frozen_delete_roots f ON f.type_id=b.type_id;
SELECT COUNT(*) INTO @orphan_prefs FROM company_type_preferences p INNER JOIN frozen_delete_roots f ON f.type_id=p.type_id;
SELECT COUNT(*) INTO @orphan_overrides FROM company_template_workflow_overrides o INNER JOIN frozen_delete_roots f ON f.type_id=o.type_id;
SELECT COUNT(*) INTO @orphan_alert FROM alert_template_configs a INNER JOIN frozen_delete_roots f ON f.type_id=a.type_id;
SELECT COUNT(*) INTO @orphan_gwf FROM global_workflows g INNER JOIN frozen_delete_roots f ON f.type_id=g.type_id;
SELECT COUNT(*) INTO @orphan_gwv FROM global_workflow_versions g INNER JOIN frozen_delete_roots f ON f.type_id=g.type_id;
SELECT COUNT(*) INTO @orphan_dg FROM template_display_groups d INNER JOIN frozen_delete_roots f ON f.type_id=d.template_id;
SELECT COUNT(*) INTO @orphan_wf_inst FROM workflow_instances wi
  INNER JOIN disclosure_records r ON r.record_id = wi.record_id
  INNER JOIN frozen_delete_roots f ON f.type_id = r.type_id;

SELECT COUNT(*) INTO @companies_after FROM companies;
SELECT COUNT(*) INTO @users_after FROM users;
SELECT COUNT(*) INTO @departments_after FROM departments;
SELECT COUNT(*) INTO @roles_after FROM roles;

SELECT COUNT(*) INTO @keep_name_ok FROM disclosure_types dt
JOIN disclosure_type_versions dtv ON dtv.type_id=dt.type_id AND dtv.version_no=dt.active_version_no
WHERE dt.type_id COLLATE utf8mb4_unicode_ci = @keep_id
  AND TRIM(dtv.name) = 'Bảng tính lương nhân viên tháng';

SET @orphan_total = @orphan_versions + @orphan_cycles + @orphan_records + @orphan_blocks
  + @orphan_prefs + @orphan_overrides + @orphan_alert + @orphan_gwf + @orphan_gwv + @orphan_dg + @orphan_wf_inst;

SET @count_delta_ok = IF(
  @del_roots = 41
  AND @del_versions = 106
  AND @del_cycles = 195
  AND @del_records = 195
  AND @del_wf_inst = 184
  AND @del_wf_tasks = 184
  AND @del_blocks = 636
  AND @del_alert = 30
  AND @del_dg = 69
  AND @del_prefs = 7
  AND @del_overrides = 4
  AND @del_ov = 19
  AND @del_gwv = 4
  AND @del_gwf = 0
  AND @del_gws = 0
  AND @del_dac = 0
  AND @del_adhoc = 0
  AND @del_woc = 0
, 1, 0);

SET @ok = IF(
  @gate_pre = 1
  AND @root_cnt = 1
  AND @keep_present = 1
  AND @keep_name_ok = 1
  AND @keep_status_after = 'active'
  AND @keep_av_after = 1
  AND @keep_versions_after = 1
  AND @keep_cycles_after = 8
  AND @keep_records_after = 8
  AND @keep_blocks_after = 6
  AND @keep_dg_after = 2
  AND @delete_root_remaining = 0
  AND @orphan_total = 0
  AND @companies_after = @companies_before
  AND @users_after = @users_before
  AND @departments_after = @departments_before
  AND @roles_after = @roles_before
  AND @count_delta_ok = 1
, 1, 0);

SET @action = IF(@ok = 1, 'COMMIT', 'ROLLBACK');
PREPARE stmt FROM @action;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SELECT @action AS txn_result,
       @ok AS all_gates_pass,
       @gate_pre AS gate_pre,
       @root_cnt AS root_cnt,
       @keep_present AS keep_present,
       @keep_name_ok AS keep_name_ok,
       @keep_status_after AS keep_status,
       @keep_av_after AS keep_av,
       @keep_versions_after AS keep_versions,
       @keep_cycles_after AS keep_cycles,
       @keep_records_after AS keep_records,
       @keep_blocks_after AS keep_blocks,
       @keep_dg_after AS keep_dg,
       @delete_root_remaining AS delete_root_remaining,
       @orphan_total AS orphan_total,
       @count_delta_ok AS count_delta_ok,
       @del_roots AS del_roots,
       @del_versions AS del_versions,
       @del_cycles AS del_cycles,
       @del_records AS del_records,
       @del_wf_inst AS del_wf_inst,
       @del_wf_tasks AS del_wf_tasks,
       @del_blocks AS del_blocks,
       @del_alert AS del_alert,
       @del_dg AS del_dg,
       @del_prefs AS del_prefs,
       @del_overrides AS del_overrides,
       @del_ov AS del_ov,
       @del_gwv AS del_gwv,
       @companies_after AS companies_after,
       @users_after AS users_after;
