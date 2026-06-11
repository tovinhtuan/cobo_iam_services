SET NAMES utf8mb4;

ALTER TABLE companies
  ADD COLUMN is_listed TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Công ty niêm yết',
  ADD COLUMN is_large_public TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'CT đại chúng quy mô lớn',
  ADD COLUMN is_non_large_public TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'CT đại chúng không QML',
  ADD COLUMN has_subsidiaries TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Có công ty con',
  ADD COLUMN has_subordinate_accounting_units TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Có ĐVKT trực thuộc',
  ADD COLUMN business_sector VARCHAR(32) NULL COMMENT 'commercial|service|manufacturing';
