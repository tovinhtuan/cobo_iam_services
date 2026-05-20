SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

DELETE FROM user_invitations WHERE invitation_id = 'inv_dev_invite_fixture';
DELETE FROM credentials WHERE user_id = 'u_dev_invite_fixture';
DELETE FROM user_subscription_tiers WHERE user_id = 'u_dev_invite_fixture';
DELETE FROM users WHERE user_id = 'u_dev_invite_fixture';

SET FOREIGN_KEY_CHECKS = 1;
