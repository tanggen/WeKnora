-- 000063: Seed Plan Data
-- Insert default plans (trial, basic, pro)

INSERT INTO plans (id, name, description, is_trial, trial_days, is_active, sort_order) VALUES
('trial',  '体验版', '免费体验3天，感受AI知识库的完整功能',  true,  3,  true, 1),
('basic',  '基础版', '适合小团队，管理少量知识库',         false, 0,  true, 2),
('pro',    '专业版', '适合中型团队，大容量存储与更多Token',  false, 0,  true, 3)
ON CONFLICT (id) DO NOTHING;

INSERT INTO plan_configs (
    plan_id, max_users, max_knowledge_bases, max_knowledge_per_kb,
    max_chunks_per_knowledge, storage_quota_bytes, token_quota_monthly,
    max_agents, price_monthly_cny, price_yearly_cny, features, marketing_features
) VALUES
(
    'trial',
    1,                              -- max 1 user
    5,                              -- max 5 knowledge bases
    100,                            -- max 100 documents per KB
    1000,                           -- max 1000 chunks per document
    524288000,                      -- 0.5 GB storage
    50000,                          -- 50K tokens/month
    1,                              -- max 1 agent
    0, 0,                           -- free
    '["chat","search","web_search"]',
    '["体验3天完整功能","支持1个用户","5个知识库","50MB存储空间","每月5万Token"]'
),
(
    'basic',
    5,                              -- max 5 users
    20,                             -- max 20 knowledge bases
    1000,                           -- max 1000 documents per KB
    10000,                          -- max 10000 chunks per document
    5368709120,                     -- 5 GB storage
    500000,                         -- 500K tokens/month
    5,                              -- max 5 agents
    9900, 95040,                    -- ¥99/month, ¥950.40/year
    '["chat","search","web_search"]',
    '["支持5个用户","20个知识库","5GB存储空间","每月50万Token","5个Agent"]'
),
(
    'pro',
    20,                             -- max 20 users
    100,                            -- max 100 knowledge bases
    10000,                          -- max 10000 documents per KB
    50000,                          -- max 50000 chunks per document
    10737418240,                    -- 10 GB storage
    5000000,                        -- 5M tokens/month
    20,                             -- max 20 agents
    29900, 287040,                  -- ¥299/month, ¥2870.40/year
    '["chat","search","web_search","api_access","advanced_analytics"]',
    '["支持20个用户","100个知识库","10GB存储空间","每月500万Token","20个Agent","API访问","高级分析"]'
)
ON CONFLICT (plan_id) DO NOTHING;
