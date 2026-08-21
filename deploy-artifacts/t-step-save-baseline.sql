SELECT type_id, active_version_no, status FROM disclosure_types WHERE type_id='bao-cao-tuan-test';
SELECT version_no, is_released, COALESCE(workflow_authority_mode,'') AS mode,
       LEFT(COALESCE(workflow_semantic_hash,''), 16) AS manifest_hash16,
       LEFT(COALESCE(publication_candidate_hash,''), 16) AS candidate_hash16,
       CHAR_LENGTH(COALESCE(workflow_manifest_json,'')) AS manifest_len,
       JSON_LENGTH(JSON_EXTRACT(workflow_manifest_json, '$.steps')) AS step_count
FROM disclosure_type_versions WHERE type_id='bao-cao-tuan-test' ORDER BY version_no;
SELECT COUNT(*) AS global_wf FROM global_workflows WHERE type_id='bao-cao-tuan-test';
SELECT COUNT(*) AS global_hist FROM global_workflow_versions WHERE type_id='bao-cao-tuan-test';
