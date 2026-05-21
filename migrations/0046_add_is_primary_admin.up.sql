SET NAMES utf8mb4;

ALTER TABLE memberships
  ADD COLUMN is_primary_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill: per company, mark the membership that (a) holds the company_admin role
-- and (b) was created earliest as the primary admin.
-- Uses created_at as a proxy for "who founded the company".
-- Safe to run on empty systems — UPDATE affects 0 rows if no company_admin roles exist.
UPDATE memberships m
JOIN (
  SELECT m2.membership_id
  FROM memberships m2
  JOIN membership_roles mr ON mr.membership_id = m2.membership_id
                          AND mr.status = 'active'
  JOIN roles r             ON r.role_id   = mr.role_id
                          AND r.role_code = 'company_admin'
  JOIN (
    SELECT m3.company_id, MIN(m3.created_at) AS earliest
    FROM memberships m3
    JOIN membership_roles mr3 ON mr3.membership_id = m3.membership_id
                             AND mr3.status = 'active'
    JOIN roles r3             ON r3.role_id   = mr3.role_id
                             AND r3.role_code = 'company_admin'
    GROUP BY m3.company_id
  ) first_per_company ON first_per_company.company_id = m2.company_id
                     AND first_per_company.earliest   = m2.created_at
) candidates ON candidates.membership_id = m.membership_id
SET m.is_primary_admin = TRUE;
