ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS key_quota_usd DECIMAL(20,8) NOT NULL DEFAULT 0;

COMMENT ON COLUMN subscription_plans.key_quota_usd IS 'API key quota delivered by this plan in USD (0 = unlimited)';
