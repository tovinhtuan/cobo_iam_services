SET NAMES utf8mb4;

-- Organization units tree (department/team) for subtree-based authorization scope.
CREATE TABLE org_units (
  org_unit_id        VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NOT NULL,
  parent_org_unit_id VARCHAR(36) NULL,
  unit_code          VARCHAR(128) NOT NULL,
  unit_name          VARCHAR(255) NOT NULL,
  unit_type          VARCHAR(32) NOT NULL, -- department|team|division
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_org_units_company_code (company_id, unit_code),
  KEY idx_org_units_company_parent (company_id, parent_org_unit_id),
  KEY idx_org_units_company_status (company_id, status),
  CONSTRAINT fk_org_units_company FOREIGN KEY (company_id) REFERENCES companies(company_id),
  CONSTRAINT fk_org_units_parent FOREIGN KEY (parent_org_unit_id) REFERENCES org_units(org_unit_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Closure table for fast subtree traversal.
CREATE TABLE org_unit_closure (
  ancestor_org_unit_id   VARCHAR(36) NOT NULL,
  descendant_org_unit_id VARCHAR(36) NOT NULL,
  depth                  INT NOT NULL,
  created_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (ancestor_org_unit_id, descendant_org_unit_id),
  KEY idx_org_unit_closure_desc (descendant_org_unit_id),
  CONSTRAINT fk_org_unit_closure_ancestor FOREIGN KEY (ancestor_org_unit_id) REFERENCES org_units(org_unit_id),
  CONSTRAINT fk_org_unit_closure_descendant FOREIGN KEY (descendant_org_unit_id) REFERENCES org_units(org_unit_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Membership <-> org unit relation with position role in unit.
CREATE TABLE org_unit_memberships (
  org_unit_membership_id BIGINT PRIMARY KEY AUTO_INCREMENT,
  company_id             VARCHAR(36) NOT NULL,
  membership_id          VARCHAR(36) NOT NULL,
  org_unit_id            VARCHAR(36) NOT NULL,
  position_code          VARCHAR(64) NOT NULL, -- admin_dn|truong_phong|pho_phong|truong_nhom|pho_nhom|nhan_vien
  status                 VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_org_unit_membership_unique (membership_id, org_unit_id),
  KEY idx_org_unit_memberships_company_membership (company_id, membership_id, status),
  KEY idx_org_unit_memberships_company_org_unit (company_id, org_unit_id, status),
  KEY idx_org_unit_memberships_position (company_id, position_code, status),
  CONSTRAINT fk_org_unit_memberships_company FOREIGN KEY (company_id) REFERENCES companies(company_id),
  CONSTRAINT fk_org_unit_memberships_membership FOREIGN KEY (membership_id) REFERENCES memberships(membership_id),
  CONSTRAINT fk_org_unit_memberships_org_unit FOREIGN KEY (org_unit_id) REFERENCES org_units(org_unit_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
