-- Dev-only fixture: invited user + one unused invitation with a stable raw token.
-- Apply after: 0025_user_invitations, seed_dev_identity_authorization (needs u_cms_operator for created_by_user_id).
--
-- Browser smoke URL (PUBLIC_WEB_BASE_URL + path):
--   /accept-invitation?token=dev-invite-fixed-token-001
-- token_hash matches Go internal/platform/refreshtoken.Hash (SHA-256 of UTF-8 raw token, lowercase hex).
--
-- This migration runs once per database. After you complete accept-invitation in the browser, to test
-- again on the same volume either wipe the volume (docker compose down -v) or manually:
--   UPDATE user_invitations SET used_at = NULL, revoked_at = NULL WHERE invitation_id = 'inv_dev_invite_fixture';
--   DELETE FROM credentials WHERE user_id = 'u_dev_invite_fixture';
--   UPDATE users SET account_status = 'invited' WHERE user_id = 'u_dev_invite_fixture';

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

INSERT INTO users (user_id, login_id, full_name, email, account_status) VALUES
  ('u_dev_invite_fixture', 'dev.invite.fixture@cobo.local', 'Dev Invite Fixture', 'dev.invite.fixture@cobo.local', 'invited')
ON DUPLICATE KEY UPDATE
  full_name = VALUES(full_name),
  email = VALUES(email),
  account_status = 'invited',
  updated_at = CURRENT_TIMESTAMP;

INSERT INTO user_subscription_tiers (user_id, subscription_tier, source, effective_from, effective_to) VALUES
  ('u_dev_invite_fixture', 'Free', 'seed_dev_invite_fixture', NULL, NULL)
ON DUPLICATE KEY UPDATE
  subscription_tier = VALUES(subscription_tier),
  source = VALUES(source),
  effective_from = VALUES(effective_from),
  effective_to = VALUES(effective_to);

DELETE FROM credentials WHERE user_id = 'u_dev_invite_fixture';

INSERT INTO user_invitations (
  invitation_id,
  user_id,
  token_hash,
  expires_at,
  created_by_user_id,
  send_count,
  last_sent_at
) VALUES (
  'inv_dev_invite_fixture',
  'u_dev_invite_fixture',
  LOWER(SHA2('dev-invite-fixed-token-001', 256)),
  DATE_ADD(UTC_TIMESTAMP(), INTERVAL 365 DAY),
  'u_cms_operator',
  1,
  UTC_TIMESTAMP()
)
ON DUPLICATE KEY UPDATE
  token_hash = VALUES(token_hash),
  expires_at = VALUES(expires_at),
  used_at = NULL,
  revoked_at = NULL,
  created_by_user_id = VALUES(created_by_user_id),
  send_count = VALUES(send_count),
  last_sent_at = VALUES(last_sent_at);

SET FOREIGN_KEY_CHECKS = 1;
