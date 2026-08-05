SET NAMES utf8mb4;

ALTER TABLE companies
    DROP CHECK chk_companies_status_valid,
    DROP CHECK chk_companies_verification_status_valid;
