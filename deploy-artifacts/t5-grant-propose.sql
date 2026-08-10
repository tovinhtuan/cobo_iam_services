-- QA propose-only role for T5 (idempotent)
INSERT INTO roles (role_id, company_id, role_code, role_name, status)
VALUES ('r_qa_propose_only_t5', 'c_001', 'qa_propose_only_t5', 'QA Propose Only T5', 'active')
ON DUPLICATE KEY UPDATE role_name=VALUES(role_name), status='active';

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'r_qa_propose_only_t5', permission_id FROM permissions WHERE permission_code='ad_hoc_alert.propose'
ON DUPLICATE KEY UPDATE role_id=role_id;

INSERT INTO membership_roles (membership_id, role_id, status)
VALUES ('m_105', 'r_qa_propose_only_t5', 'active')
ON DUPLICATE KEY UPDATE status='active';

SELECT r.role_code, p.permission_code
FROM membership_roles mr
JOIN roles r ON r.role_id=mr.role_id
JOIN role_permissions rp ON rp.role_id=r.role_id
JOIN permissions p ON p.permission_id=rp.permission_id
WHERE mr.membership_id='m_105' AND mr.status='active' AND p.permission_code LIKE 'ad_hoc%';

SELECT company_id, company_name FROM companies LIMIT 10;
