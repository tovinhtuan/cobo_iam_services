SET NAMES utf8mb4;

DROP TABLE IF EXISTS email_verification_otps;

ALTER TABLE companies DROP COLUMN verification_status;
