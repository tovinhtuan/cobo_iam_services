SET NAMES utf8mb4;

ALTER TABLE companies
    ADD CONSTRAINT chk_companies_status_valid
        CHECK (status COLLATE utf8mb4_bin IN ('active', 'inactive')),
    ADD CONSTRAINT chk_companies_verification_status_valid
        CHECK (verification_status COLLATE utf8mb4_bin IN ('verified', 'unverified'));
