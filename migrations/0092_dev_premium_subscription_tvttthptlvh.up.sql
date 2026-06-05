-- Nâng subscription tier của tvttthptlvh@gmail.com lên Premium.
-- Premium: max 3 self-provisioned companies (xem admin_service_provision_quota.go).
-- User hiện có 2 công ty (Free chỉ cho 1) — nâng Premium để tạo thêm được công ty thứ 3+.
UPDATE user_subscription_tiers
SET
    subscription_tier = 'Premium',
    source            = 'admin_grant',
    updated_at        = NOW()
WHERE user_id = 'd938e010-1064-440c-8c7b-aba6e1924402';
