SET NAMES utf8mb4;

-- Suspended companies have no equivalent in the previous allowlist: they become inactive (no
-- access) rather than active, so rolling back never grants more than was granted.
UPDATE companies SET status = 'inactive' WHERE status = 'suspended';

ALTER TABLE companies
    DROP CHECK chk_companies_status_valid,
    ADD CONSTRAINT chk_companies_status_valid
        CHECK (status COLLATE utf8mb4_bin IN ('active', 'inactive'));
