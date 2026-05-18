SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS user_subscription_tiers (
  user_id            VARCHAR(36) PRIMARY KEY,
  subscription_tier  VARCHAR(32) NOT NULL,
  source             VARCHAR(64) NOT NULL DEFAULT 'seed',
  effective_from     TIMESTAMP NULL,
  effective_to       TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_subscription_tier_active_window (subscription_tier, effective_from, effective_to),
  CONSTRAINT fk_user_subscription_tier_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO user_subscription_tiers (user_id, subscription_tier, source, effective_from, effective_to)
SELECT u.user_id, 'Free', 'migration-0011-default', NULL, NULL
FROM users u
ON DUPLICATE KEY UPDATE
  subscription_tier = user_subscription_tiers.subscription_tier,
  source = user_subscription_tiers.source,
  effective_from = user_subscription_tiers.effective_from,
  effective_to = user_subscription_tiers.effective_to;

