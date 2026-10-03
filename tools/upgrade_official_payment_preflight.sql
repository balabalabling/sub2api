-- Sub2API official-payment upgrade: READ-ONLY preflight.
-- Run against an isolated restore of the production database before release.
-- Do not assume the absence of payment traffic implies no historical custom orders.
SELECT order_type, status, count(*) AS orders
FROM payment_orders
GROUP BY order_type, status
ORDER BY order_type, status;

-- Any rows here need a decision before removing the custom fulfillment implementation.
SELECT id, order_type, status, created_at
FROM payment_orders
WHERE order_type NOT IN ('balance', 'subscription')
  AND status IN ('PENDING', 'PAID', 'RECHARGING', 'FAILED')
ORDER BY created_at
LIMIT 100;

SELECT filename, checksum, applied_at
FROM schema_migrations
WHERE filename IN ('145_storefront.sql', '151_subscription_plan_key_quota.sql',
                   '152_payment_order_api_key_target.sql',
                   '241_add_payment_order_bonus_amount.sql', '241_add_typesafe_platform.sql')
ORDER BY filename;

SELECT platform, count(*) AS rows FROM user_platform_quotas GROUP BY platform;
SELECT target_platform, count(*) AS rows FROM composite_model_routes GROUP BY target_platform;

-- Historical key-targeted subscription orders also need review even if order_type is official.
SELECT order_type, status, count(*) AS key_targeted_orders
FROM payment_orders
WHERE api_key_id IS NOT NULL
GROUP BY order_type, status
ORDER BY order_type, status;

-- Retired fulfillment/refund paths: include all non-terminal refund states.
SELECT order_type, status, count(*) AS unresolved_custom_orders
FROM payment_orders
WHERE (order_type NOT IN ('balance', 'subscription') OR api_key_id IS NOT NULL)
  AND status NOT IN ('COMPLETED', 'CANCELLED', 'EXPIRED', 'REFUNDED')
GROUP BY order_type, status
ORDER BY order_type, status;

SELECT delivery_status, count(*) AS store_orders
FROM store_orders GROUP BY delivery_status ORDER BY delivery_status;
SELECT count(*) AS plans_with_custom_key_quota
FROM subscription_plans WHERE key_quota_usd <> 0;

-- A zero-row result is required before adding the official platform constraints.
SELECT platform, count(*) AS incompatible_rows
FROM user_platform_quotas
WHERE platform NOT IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok',
                      'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe')
GROUP BY platform;
SELECT target_platform, count(*) AS incompatible_rows
FROM composite_model_routes
WHERE target_platform NOT IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok',
                             'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe')
GROUP BY target_platform;
