-- Phase: In-App Notification Feature
-- Stores per-user in-app notifications shown in the Bell icon dropdown.
-- Scoped by (user_id, company_id) for cross-tenant isolation.
CREATE TABLE user_in_app_notifications (
  id           VARCHAR(64)     NOT NULL,
  user_id      VARCHAR(64)     NOT NULL,
  company_id   VARCHAR(64)     NOT NULL,
  kind         VARCHAR(64)     NOT NULL,
  title        VARCHAR(255)    NOT NULL,
  body         TEXT,
  resource_type VARCHAR(64),
  resource_id  VARCHAR(64),
  is_read      TINYINT(1)      NOT NULL DEFAULT 0,
  created_at   TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_uin_user_company_read    (user_id, company_id, is_read),
  KEY idx_uin_user_company_created (user_id, company_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
