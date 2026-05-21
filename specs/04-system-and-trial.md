# 卡片 04：系统租户 + 体验租户初始化

> 覆盖：T-02（系统租户初始化）+ T-04（体验租户注册引导）
> 依赖：卡片 01（tenant_users 表）、卡片 02（tenant_stats、plans 表）、卡片 03（注册接口）
> 基准需求：`specs/REQUIREMENTS.md` v1.4 第 2、4 节

---

## 1. API 契约

本卡片无独立 API 端点。相关 API 已由其他卡片定义：

| API | 定义卡片 | 关系 |
|-----|:------:|------|
| `POST /api/v1/auth/register` | 卡片 03 | 注册时自动创建体验租户，相关逻辑在本卡片中 |
| `POST /api/v1/admin/tenants/{id}/plan` | 卡片 02 | 系统管理员可为体验租户升级套餐 |
| `GET /api/v1/admin/stats/overview` | 卡片 02 | Dashboard 展示体验租户统计数据 |

---

## 2. 数据库 DDL

### 2.1 种子脚本：系统租户 + 系统管理员

```sql
-- ======================================================
-- 种子迁移：系统租户 + 系统管理员
-- 文件：migrations/versioned/000080_seed_system_tenant.up.sql
-- ======================================================

-- 1. 创建系统租户
INSERT INTO tenants (id, name, description, status, is_system, plan_id)
VALUES (1, '系统管理', '系统默认管理租户', 'active', true, 'trial')
ON CONFLICT (id) DO NOTHING;

-- 2. 创建系统管理员用户（默认密码 admin_123456，首次登录强制修改）
-- bcrypt 哈希值对应明文 "admin_123456"（cost=12）
INSERT INTO users (id, username, email, password_hash, is_active, password_expired)
VALUES (
    'sys-admin-001',
    'admin',
    'admin@system.local',
    '$2a$12$LJ3m4ys3GZfnYM5LJh0TKO0bXL5fLDR6Kv6VkG7mK8w0b5d4m5p5q',  -- 占位，需替换实际 bcrypt hash
    true,
    true       -- password_expired = TRUE → 首次登录 423 强制改密
)
ON CONFLICT (id) DO NOTHING;

-- 3. 建立租户-用户关联（角色 = system_admin）
INSERT INTO tenant_users (tenant_id, user_id, role, permissions)
VALUES (
    1,
    'sys-admin-001',
    'system_admin',
    '["system:admin", "kb:*", "agent:*", "chat:use", "tenant:manage", "user:manage", "model:manage"]'::jsonb
)
ON CONFLICT (tenant_id, user_id) DO NOTHING;

-- 4. 初始化系统租户统计
INSERT INTO tenant_stats (tenant_id, user_count, plan_id, storage_quota_bytes)
VALUES (1, 1, 'trial', 10737418240)
ON CONFLICT (tenant_id) DO NOTHING;
```

### 2.2 种子脚本回滚

```sql
-- migrations/versioned/000080_seed_system_tenant.down.sql
DELETE FROM tenant_stats WHERE tenant_id = 1;
DELETE FROM tenant_users WHERE tenant_id = 1 AND user_id = 'sys-admin-001';
DELETE FROM users WHERE id = 'sys-admin-001';
DELETE FROM tenants WHERE id = 1;
```

### 2.3 种子脚本：套餐数据

```sql
-- ======================================================
-- 种子迁移：套餐种子数据
-- 文件：migrations/versioned/000081_seed_plan_data.up.sql
-- ======================================================

-- 体验版
INSERT INTO plans (id, name, description, is_trial, trial_days, is_active, sort_order)
VALUES ('trial', '体验版', '新用户免费体验 3 天', true, 3, true, 1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO plan_configs (plan_id, max_users, max_knowledge_bases, max_knowledge_per_kb,
    max_chunks_per_knowledge, storage_quota_bytes, token_quota_monthly, max_agents,
    features, marketing_features, price_monthly_cny, price_yearly_cny)
VALUES (
    'trial',
    1,              -- 最多 1 个用户
    5,              -- 最多 5 个知识库
    100,            -- 每个 KB 最多 100 条文档
    1000,           -- 每条文档最多 1000 个分块
    524288000,      -- 0.5 GB 存储
    50000,          -- 每月 5 万 token
    1,              -- 最多 1 个 Agent
    '["chat", "search"]'::jsonb,
    '[]'::jsonb,
    0, 0
) ON CONFLICT (plan_id) DO NOTHING;

-- 基础版
INSERT INTO plans (id, name, description, is_trial, trial_days, is_active, sort_order)
VALUES ('basic', '基础版', '适合个人知识管理者', false, 0, true, 2)
ON CONFLICT (id) DO NOTHING;

INSERT INTO plan_configs (plan_id, max_users, max_knowledge_bases, max_knowledge_per_kb,
    max_chunks_per_knowledge, storage_quota_bytes, token_quota_monthly, max_agents,
    features, marketing_features, price_monthly_cny, price_yearly_cny)
VALUES (
    'basic',
    5, 20, 1000, 10000,
    5368709120,     -- 5 GB
    500000,         -- 50 万/月
    5,
    '["chat", "search", "web_search"]'::jsonb,
    '[]'::jsonb,
    29900, 286800   -- ¥299/月, ¥2868/年（分）
) ON CONFLICT (plan_id) DO NOTHING;

-- 专业版
INSERT INTO plans (id, name, description, is_trial, trial_days, is_active, sort_order)
VALUES ('pro', '专业版', '适合专业团队与知识密集型场景', false, 0, true, 3)
ON CONFLICT (id) DO NOTHING;

INSERT INTO plan_configs (plan_id, max_users, max_knowledge_bases, max_knowledge_per_kb,
    max_chunks_per_knowledge, storage_quota_bytes, token_quota_monthly, max_agents,
    features, marketing_features, price_monthly_cny, price_yearly_cny)
VALUES (
    'pro',
    20, 100, 10000, 0,   -- 0 = 无限制
    10737418240,    -- 10 GB
    5000000,        -- 500 万/月
    20,
    '["chat", "search", "web_search", "api_access"]'::jsonb,
    '[]'::jsonb,
    99900, 958800   -- ¥999/月, ¥9588/年
) ON CONFLICT (plan_id) DO NOTHING;
```

### 2.4 迁移编号

```
migrations/versioned/000080_seed_system_tenant.up.sql   — 系统租户 + admin 种子
migrations/versioned/000080_seed_system_tenant.down.sql — 回滚
migrations/versioned/000081_seed_plan_data.up.sql        — 套餐种子数据
migrations/versioned/000081_seed_plan_data.down.sql      — 回滚
```

---

## 3. Redis Key Schema

本卡片无新增 Redis Key。涉及的 Key：

| Key | 定义卡片 | 说明 |
|-----|:------:|------|
| `plan:{plan_id}:config` | 卡片 02 | 启动时预热：加载种子套餐到 Redis |
| `plan:list:active` | 卡片 02 | 启动时预热 |

### 启动预热

```
// cmd/server/main.go 中的初始化逻辑
func preloadPlanCache() {
    for _, plan := range db.FindAllActivePlans() {
        redis.HSet("plan:"+plan.ID+":config", plan.ToConfigMap())
        redis.Expire("plan:"+plan.ID+":config", 1*time.Hour)
    }
    redis.Set("plan:list:active", json.Marshal(db.FindAllActivePlans()), 30*time.Minute)
}
```

---

## 4. 定时任务

本卡片无独立定时任务。涉及的任务：

| 任务 | 定义卡片 | 说明 |
|------|:------:|------|
| `check_trial_expiry` | 卡片 02 | 检查体验租户到期 |
| `reset_token_quota_monthly` | 卡片 02 | 每月重置 |

### 体验租户到期检查逻辑（`check_trial_expiry` 详解）

```go
func CheckTrialExpiry(ctx context.Context) error {
    // 1. 查出已过期的体验租户
    expired := db.Raw(`
        SELECT id, name FROM tenants
        WHERE plan_id = 'trial'
          AND trial_expires_at < NOW()
          AND status = 'active'
    `).Scan(&tenants)

    // 2. 标记状态
    for _, t := range expired {
        // 方案 B 已确认：当前对话继续，下次对话时阻断
        // → 不立即中断 API，而是标记状态
        db.Exec(`UPDATE tenants SET status = 'trial_expired' WHERE id = ?`, t.ID)

        // 可选：通知用户
        // notifyService.SendTrialExpiredNotice(t.ID)
    }
    return nil
}
```

### Token 每月重置逻辑

```
// 每月 1 号凌晨 0 点
func ResetTokenQuotaMonthly(ctx context.Context) error {
    db.Exec(`
        UPDATE tenant_stats
        SET token_used_monthly = 0,
            token_quota_reset_at = DATE_TRUNC('month', NOW()) + INTERVAL '1 month'
    `)
    return nil
}
```

---

## 5. 业务规则汇总

### 5.1 系统租户规则

| 规则 | 说明 |
|------|------|
| ID=1 | 系统租户固定使用 tenant_id=1 |
| 标志位 | `tenants.is_system = TRUE` 标识系统租户 |
| 唯一管理员 | 仅 sys-admin-001 属于该租户 |
| 角色 | 该用户在 tenant_users 中 role=system_admin |
| 默认密码 | `admin_123456`，首次登录 423 强制改密 |
| 不可删除 | 系统租户不能被任何方式删除 |
| 不可禁用 | 系统租户 status='active' 不可变更 |
| 套餐 | 系统租户默认关联 trial 套餐（无实际限制意义，system_admin 操作不受套餐约束） |

### 5.2 体验租户规则

| 规则 | 说明 |
|------|------|
| 触发方式 | 用户自助注册时自动创建，或 API `POST /auth/register` |
| 套餐 | plan_id='trial' |
| 有效期 | trial_expires_at = created_at + 3天 |
| 用户角色 | 注册者为该租户的 tenant_admin |
| 状态 | 创建时 status='active'，到期后 cron 标记为 'trial_expired' |
| 升级 | 系统管理员可通过 `POST /admin/tenants/{id}/plan` 切换套餐 |
| 过期行为 | 方案B：当前对话继续，下次对话返回 403 TRIAL_EXPIRED |

### 5.3 应用启动初始化（`cmd/server/main.go`）

```
1. 自动执行数据库迁移（golang-migrate）
2. 种子数据检查（若 tenant ID=1 不存在 → 执行种子脚本插入）
3. 预热 Redis 套餐缓存
4. 启动 cron scheduler
5. 启动 HTTP server
```

---

## 6. 验收测试场景

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | 首次部署初始化 | 空数据库 → 启动 service | tenants ID=1 存在，users sys-admin-001 存在，plans trial/basic/pro 存在 |
| 2 | ✅ | 系统管理员登录 | POST /auth/login {account:"admin", password:"admin_123456"} | 423 FIRST_LOGIN_RESET → 改密 → 200 |
| 3 | ✅ | 自助注册创建体验租户 | POST /auth/register → 检查 DB | 新租户 plan_id='trial', trial_expires_at≈now+3天, role='tenant_admin' |
| 4 | ❌ | 体验租户到期 | 手动改 trial_expires_at=now-1小时 → 等 cron 触发 → 该租户用户发对话 | 403 TRIAL_EXPIRED |
| 5 | ❌ | 系统租户不可删除 | DELETE /api/v1/admin/tenants/1 | 403 FORBIDDEN 或 422 SYSTEM_TENANT_PROTECTED |
| 6 | ❌ | 重复注册同名租户 | A 注册 tenant_name="测试空间" → B 也注册 "测试空间" | 409 TENANT_NAME_EXISTS |

---

## 7. 文件清单

### 新建文件

| 文件 | 说明 |
|------|------|
| `migrations/versioned/000080_seed_system_tenant.up.sql` | 系统租户 + admin 种子数据 |
| `migrations/versioned/000080_seed_system_tenant.down.sql` | 回滚 |
| `migrations/versioned/000081_seed_plan_data.up.sql` | 套餐种子数据（trial/basic/pro） |
| `migrations/versioned/000081_seed_plan_data.down.sql` | 回滚 |
| `internal/initialization/seed.go` | 种子数据初始化器（启动时检查并插入、预热 Redis） |

### 修改文件

| 文件 | 说明 |
|------|------|
| `cmd/server/main.go` | 启动流程：迁移 → 种子检查 → 预热缓存 → cron → HTTP server |

---

## 质量门控

- [x] 每个 P0 功能都有对应 API 端点 + Request + Response + Errors（依赖其他卡片，本卡片无独立 API）
- [x] 每张数据库表有完整 DDL（2 个种子 SQL 迁移文件）
- [x] 每个 Redis Key 标注了类型和 TTL（启动预热逻辑已定义）
- [x] 定时任务已定义（2个任务，含详细逻辑）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 3 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
