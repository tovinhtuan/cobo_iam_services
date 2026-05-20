-- Direct per-membership permission grants, independent of role assignments.
-- Enables Enterprise Admin to grant/revoke specific capabilities (e.g. ad_hoc_alert.propose)
-- to individual members without changing their role.
-- source_type='direct' in membership_effective_permissions projection references rows here.

CREATE TABLE membership_direct_permissions (
  id              BIGINT PRIMARY KEY AUTO_INCREMENT,
  membership_id   VARCHAR(36)  NOT NULL,
  company_id      VARCHAR(36)  NOT NULL,
  permission_code VARCHAR(191) NOT NULL,
  granted_by      VARCHAR(36)  NOT NULL,
  granted_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  revoked_by      VARCHAR(36)  NULL,
  revoked_at      TIMESTAMP    NULL,
  UNIQUE KEY uk_mdp_active (membership_id, permission_code, revoked_at),
  KEY idx_mdp_lookup (company_id, membership_id),
  CONSTRAINT fk_mdp_membership FOREIGN KEY (membership_id) REFERENCES memberships(membership_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
