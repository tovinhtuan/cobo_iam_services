SET NAMES utf8mb4;

-- "Tạm ngưng" (suspended, e.g. unpaid): members keep read-only access (view and export) because
-- disclosure deadlines still apply; "inactive" ("Ngừng hoạt động") shuts them out.
ALTER TABLE companies
    DROP CHECK chk_companies_status_valid,
    ADD CONSTRAINT chk_companies_status_valid
        CHECK (status COLLATE utf8mb4_bin IN ('active', 'inactive', 'suspended'));
