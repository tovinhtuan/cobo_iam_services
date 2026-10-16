-- verify_0155_adhoc_multi_reviewer_tables_backfill.sql
-- READ-ONLY (SELECT only). Safe on any env: no DDL/DML.
-- Run after 0155. Every row's `status` MUST be 'PASS'.

-- 1. both multi-reviewer tables exist
SELECT '1.tables_exist' AS check_name,
       IF(COUNT(*)=2,'PASS','FAIL') AS status,
       CONCAT('tables_found=',COUNT(*)) AS detail
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_name IN ('ad_hoc_proposal_reviewers','ad_hoc_proposal_approvals')

UNION ALL
-- 2. no legacy proposal is left without a reviewer
SELECT '2.legacy_controller_has_reviewer',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('legacy_without_reviewer=',COUNT(*))
FROM ad_hoc_proposals p
WHERE p.process_controller_id IS NOT NULL AND p.process_controller_id <> ''
  AND NOT EXISTS (SELECT 1 FROM ad_hoc_proposal_reviewers r WHERE r.proposal_id = p.proposal_id)

UNION ALL
-- 3. every reviewer row belongs to the company of its proposal
SELECT '3.reviewer_company_matches_proposal',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('mismatched_rows=',COUNT(*))
FROM ad_hoc_proposal_reviewers r
JOIN ad_hoc_proposals p ON p.proposal_id = r.proposal_id
WHERE r.company_id <> p.company_id

UNION ALL
-- 4. informational: reviewer membership missing or in another company (legacy data quality)
SELECT '4.info_reviewer_membership_outside_company',
       'PASS', CONCAT('rows=',COUNT(*))
FROM ad_hoc_proposal_reviewers r
LEFT JOIN memberships m ON m.membership_id = r.membership_id AND m.company_id = r.company_id
WHERE m.membership_id IS NULL

UNION ALL
-- 5. 0155 is recorded
SELECT '5.ledger_has_0155',
       IF(COUNT(*)=1,'PASS','FAIL'), CONCAT('rows=',COUNT(*))
FROM schema_migrations
WHERE file_name = '0155_adhoc_multi_reviewer_tables_backfill.up.sql';
