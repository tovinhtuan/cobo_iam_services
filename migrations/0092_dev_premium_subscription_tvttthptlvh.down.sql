-- Rollback: hoàn về Free cho tvttthptlvh@gmail.com.
UPDATE user_subscription_tiers
SET
    subscription_tier = 'Free',
    source            = 'public_registration',
    updated_at        = NOW()
WHERE user_id = 'd938e010-1064-440c-8c7b-aba6e1924402';
