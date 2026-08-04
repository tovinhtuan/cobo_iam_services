SET NAMES utf8mb4;

-- Commercial company paid-plan source-of-truth (Case C).
-- Not derived from user_subscription_tiers / CompanyTierResolver.
CREATE TABLE company_subscriptions (
  id               VARCHAR(36)  NOT NULL,
  company_id       VARCHAR(36)  NOT NULL,
  plan_code        VARCHAR(32)  NOT NULL,
  status           VARCHAR(32)  NOT NULL,
  effective_from   TIMESTAMP    NOT NULL,
  expires_at       TIMESTAMP    NULL,
  origin           VARCHAR(64)  NOT NULL DEFAULT 'manual',
  created_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_company_subscriptions_lookup (company_id, status, effective_from, expires_at),
  KEY idx_company_subscriptions_origin (origin),
  CONSTRAINT fk_company_subscriptions_company
    FOREIGN KEY (company_id) REFERENCES companies (company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
