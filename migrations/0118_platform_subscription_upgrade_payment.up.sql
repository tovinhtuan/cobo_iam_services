SET NAMES utf8mb4;

-- Singleton platform config for Phase-1 subscription upgrade QR / hotline instruction.
CREATE TABLE IF NOT EXISTS platform_subscription_upgrade_payment (
  id TINYINT NOT NULL PRIMARY KEY DEFAULT 1,
  description_vi TEXT NULL,
  description_en TEXT NULL,
  hotline VARCHAR(64) NULL,
  bank_name VARCHAR(128) NULL,
  account_name VARCHAR(255) NULL,
  account_number VARCHAR(64) NULL,
  transfer_note_template VARCHAR(255) NULL,
  is_active TINYINT(1) NOT NULL DEFAULT 0,
  qr_object_key VARCHAR(512) NULL,
  qr_content_type VARCHAR(128) NULL,
  qr_file_name VARCHAR(255) NULL,
  updated_by VARCHAR(64) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  CONSTRAINT chk_platform_subscription_upgrade_payment_singleton CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO platform_subscription_upgrade_payment (id, transfer_note_template, is_active)
VALUES (1, 'COBO {{company_code}} NANGCAPGOI', 0)
ON DUPLICATE KEY UPDATE id = id;
