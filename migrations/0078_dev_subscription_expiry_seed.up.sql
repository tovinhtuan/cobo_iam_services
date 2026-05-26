-- 0078: Dev QA ACC-QA-10 — show subscription_expires_at for admin.dn on Account tab.
UPDATE user_subscription_tiers
SET effective_to = '2027-12-31 00:00:00'
WHERE user_id = 'u_admin_dn'
  AND subscription_tier = 'Enterprise';
