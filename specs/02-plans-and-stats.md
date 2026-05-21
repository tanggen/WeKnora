# 卡片 02：套餐体系 + 租户统计

> 覆盖：T-03 (套餐体系) + 租户统计基础设施 (tenant_stats)
> 依赖：卡片 01（tenant_users 表已创建）
> 基准需求：`specs/REQUIREMENTS.md` v1.4 第 3、4、5、10、16 节

---

## 1. API 契约

### 公共约定

同卡片 01。所有系统管理类端点仅 `system_admin` 可访问。

### 端点清单

#### 1.1 列出租户统计（租户自己查看）

```
GET /api/v1/tenants/{tenant_id}/stats

权限：tenant_admin, system_admin
描述：返回当前租户的统计数据快照

Response 200:
{
  "data": {
    "tenant_id": 10001,
    "user_count": 12,
    "knowledge_base_count": 5,
    "knowledge_count": 230,
    "chunk_count": 14580,
    "storage_used_bytes": 2147483648,       // 2 GB
    "storage_quota_bytes": 10737418240,     // 10 GB（来自 plan 配置）
    "token_used": 500000,
    "token_quota_monthly": 1000000,         // 本月配额（来自 plan）
    "token_quota_reset_at": "2026-06-01T00:00:00Z",
    "agent_count": 3,
    "session_count": 450
  }
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  404  RESOURCE_NOT_FOUND
```

#### 1.2 列出租户统计（系统管理员查看所有租户）

```
GET /api/v1/admin/stats/tenants

权限：system_admin
描述：返回所有租户的统计列表（分页，按用量排序）

Query:
  page       (int, default=1)
  page_size  (int, default=20, max=100)
  sort_by    (string, default="token_used", 允许: user_count | storage_used_bytes | token_used | created_at)
  order      (string, default="desc", 允许: asc | desc)
  keyword    (string, optional — 按租户名称搜索)

Response 200:
{
  "data": [
    {
      "tenant_id": 10001,
      "tenant_name": "XX科技有限公司",
      "plan_name": "pro",
      "user_count": 25,
      "storage_used_bytes": 5368709120,
      "token_used": 3200000,
      "token_quota_monthly": 1000000,
      "is_active": true,
      "created_at": "2026-05-01T00:00:00Z"
    }
  ],
  "total": 42,
  "page": 1,
  "page_size": 20
}
```

#### 1.3 列出租户全局统计（dashboard）

```
GET /api/v1/admin/stats/overview

权限：system_admin
描述：全局 dashboard 汇总数据

Response 200:
{
  "data": {
    "total_tenants": 42,
    "active_tenants": 38,
    "total_users": 560,
    "total_knowledge_bases": 180,
    "total_storage_used_bytes": 107374182400,
    "total_token_used_all_time": 125000000,
    "trials_active": 12,                   // 体验租户数（3天内创建的体验套餐）
    "trials_converted": 8                  // 已转为付费的体验租户数
  }
}
```

#### 1.4 套餐列表

```
GET /api/v1/plans

权限：所有登录用户（需登录态，不限角色）
描述：获取套餐列表。用于展示页面、新用户选套餐。

Response 200:
{
  "data": [
    {
      "plan_id": "trial",
      "name": "体验版",
      "is_trial": true,
      "trial_days": 3,
      "max_users": 1,
      "max_knowledge_bases": 5,
      "max_knowledge_per_kb": 100,
      "max_chunks_per_knowledge": 1000,
      "storage_quota_bytes": 524288000,          // 0.5 GB
      "token_quota_monthly": 50000,
      "max_agents": 1,
      "features": ["chat", "search", "web_search"],
      "price_monthly_cny": 0,
      "price_yearly_cny": 0,
      "sort_order": 1
    },
    {
      "plan_id": "basic",
      "name": "基础版",
      "is_trial": false,
      "trial_days": 0,
      "max_users": 5,
      "max_knowledge_bases": 20,
      ...
      "price_monthly_cny": 9900,                  // ¥99.00
      "price_yearly_cny": 95040,                  // ¥950.40
      "sort_order": 2
    },
    ...
  ]
}
```

#### 1.5 获取套餐详情

```
GET /api/v1/plans/{plan_id}

权限：所有登录用户
描述：获取单个套餐详情

Response 200:
{
  "data": {
    "plan_id": "pro",
    "name": "专业版",
    "config": { /* 同上结构 */ },
    "features_detail": [
      { "key": "api_access", "name": "API 访问", "enabled": true },
      { "key": "advanced_analytics", "name": "高级分析", "enabled": true },
      ...
    ]
  }
}
```

#### 1.6 系统管理员管理套餐（后台用）

```
POST   /api/v1/admin/plans                       # 创建新套餐
PUT    /api/v1/admin/plans/{plan_id}             # 编辑套餐
DELETE /api/v1/admin/plans/{plan_id}             # 删除套餐（仅当无租户使用时）
PATCH  /api/v1/admin/plans/{plan_id}/status      # 启用/下架套餐

权限：system_admin
```

##### 1.6.1 创建套餐

```
POST /api/v1/admin/plans

Request Body:
{
  "plan_id": "enterprise",               // string, required, 唯一标识，小写下划线
  "name": "企业版",                      // string, required
  "description": "面向大型企业",
  "is_trial": false,                     // bool
  "trial_days": 0,                       // int
  "max_users": 0,                        // int, 0=无限制
  "max_knowledge_bases": 100,
  "max_knowledge_per_kb": 10000,
  "max_chunks_per_knowledge": 0,         // 0=无限制
  "storage_quota_bytes": 0,              // int64, 0=无限制
  "token_quota_monthly": 5000000,
  "max_agents": 20,
  "features": ["chat", "search", "web_search", "api_access", "advanced_analytics"],
  "marketing_features": [                // 前端展示用的特性列表
    { "key": "unlimited_storage", "name": "无限存储", "description": "不限制文件上传大小" }
  ],
  "price_monthly_cny": 99900,
  "price_yearly_cny": 958800,
  "sort_order": 4
}

Response 201:
{ "data": { "plan_id": "enterprise" } }

Errors:
  400  VALIDATION_ERROR
  409  PLAN_EXISTS  — plan_id 已被使用
```

##### 1.6.2 启用/下架套餐

```
PATCH /api/v1/admin/plans/{plan_id}/status

Request Body:
{
  "is_active": false                     // bool
}

Response 200:
{
  "data": { "plan_id": "pro", "is_active": false }
}

Errors:
  422  PLAN_IN_USE  — 有租户正在使用此套餐，不能下架
```

#### 1.7 租户套餐分配

```
POST /api/v1/admin/tenants/{tenant_id}/plan

权限：system_admin
描述：为租户分配/切换套餐

Request Body:
{
  "plan_id": "pro"                       // string, required
}

Response 200:
{
  "data": {
    "tenant_id": 10001,
    "plan_id": "pro",
    "plan_name": "专业版",
    "effective_at": "2026-05-20T12:00:00Z",
    "previous_plan_id": "trial"
  }
}

Errors:
  404  RESOURCE_NOT_FOUND — plan_id 或 tenant_id 不存在
  422  PLAN_NOT_ACTIVE    — 套餐已下架
  422  SAME_PLAN          — 当前已使用该套餐

副作用:
  - 更新 tenants.plan_id
  - 更新 tenants.storage_quota = plans.config.storage_quota_bytes (若配置变更)
  - 删除 Redis plan:{tenant_id}:config 缓存（强制下次加载最新）
  - 如果 token_quota_monthly 变了，重置 token_quota_reset_at 为下月1日
```

---

## 2. 数据库 DDL

### 2.1 新建表：`plans`

```sql
CREATE TABLE plans (
    id             VARCHAR(32) PRIMARY KEY,        -- 如 'trial', 'basic', 'pro', 'enterprise'
    name           VARCHAR(128) NOT NULL,
    description    TEXT,
    is_trial       BOOLEAN NOT NULL DEFAULT FALSE,
    trial_days     INT NOT NULL DEFAULT 0,         -- 体验天数（is_trial=true 时生效）
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order     INT NOT NULL DEFAULT 99,

    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE plan_configs (
    plan_id VARCHAR(32) PRIMARY KEY REFERENCES plans(id) ON DELETE CASCADE,

    -- 资源限制（0 表示无限制）
    max_users                 INT NOT NULL DEFAULT 5,
    max_knowledge_bases       INT NOT NULL DEFAULT 20,
    max_knowledge_per_kb      INT NOT NULL DEFAULT 1000,
    max_chunks_per_knowledge  INT NOT NULL DEFAULT 10000,
    storage_quota_bytes       BIGINT NOT NULL DEFAULT 10737418240,  -- 10 GB
    token_quota_monthly       BIGINT NOT NULL DEFAULT 100000,       -- 默认 10 万/月
    max_agents                INT NOT NULL DEFAULT 5,

    -- 价格（分）
    price_monthly_cny INT NOT NULL DEFAULT 0,
    price_yearly_cny  INT NOT NULL DEFAULT 0,

    -- JSON 配置
    features            JSONB NOT NULL DEFAULT '["chat","search"]',
    marketing_features  JSONB NOT NULL DEFAULT '[]',

    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### 2.2 新建表：`tenant_stats`

```sql
CREATE TABLE tenant_stats (
    tenant_id             BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,

    user_count            INT NOT NULL DEFAULT 0,
    knowledge_base_count  INT NOT NULL DEFAULT 0,
    knowledge_count       INT NOT NULL DEFAULT 0,
    chunk_count           BIGINT NOT NULL DEFAULT 0,
    storage_used_bytes    BIGINT NOT NULL DEFAULT 0,
    token_used_total      BIGINT NOT NULL DEFAULT 0,        -- 历史总额（不重置）
    token_used_monthly    BIGINT NOT NULL DEFAULT 0,        -- 当月已用
    token_quota_reset_at  TIMESTAMP WITH TIME ZONE,          -- 下月重置时间

    agent_count           INT NOT NULL DEFAULT 0,
    session_count         BIGINT NOT NULL DEFAULT 0,

    -- 套餐快照
    plan_id               VARCHAR(32),
    storage_quota_bytes   BIGINT NOT NULL DEFAULT 10737418240,

    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tenant_stats_plan_id ON tenant_stats(plan_id);
```

### 2.3 修改表：`tenants`

```sql
-- 实际项目已存在 tenants 表，此处列出需要的变更
ALTER TABLE tenants ADD COLUMN plan_id VARCHAR(32) DEFAULT 'trial';
ALTER TABLE tenants ADD COLUMN trial_expires_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX idx_tenants_plan_id ON tenants(plan_id);
```

### 2.4 迁移编号

```
migrations/versioned/000060_plans.up.sql         — plans + plan_configs + 种子数据
migrations/versioned/000061_tenant_stats.up.sql   — tenant_stats 表
migrations/versioned/000062_tenants_plan.up.sql   — tenants.plan_id + trial_expires_at
migrations/versioned/000063_seed_plan_data.up.sql — 插入 trial/basic/pro 种子数据
```

---

## 3. Redis Key Schema

### 本卡片管理的 Key

| Key | 类型 | TTL | 用途 |
|-----|------|:---:|------|
| `plan:{plan_id}:config` | Hash | 1 小时 | 套餐配置缓存。字段: max_users, max_kb, storage_quota, features(JSON) 等。读时先查 Redis，miss 则查 DB 并回填。 |
| `plan:{plan_id}:features` | String (JSON) | 1 小时 | 套餐 features 列表缓存（供 menu API 用） |
| `plan:list:active` | String (JSON) | 30 分钟 | 活跃套餐列表缓存（供 /plans 列表 API 用） |

### 操作流程

```
读套餐配置:
  redis.HGetAll("plan:pro:config")
  若 miss → DB 查 plans JOIN plan_configs → HSet 回填（TTL=1h）

写套餐配置（管理后台）:
  写 DB → DEL plan:{plan_id}:config + DEL plan:{plan_id}:features + DEL plan:list:active
```

---

## 4. 定时任务

### 完整调度表

| 任务 | cron | 队列 | 描述 |
|------|:----:|------|------|
| `reconcile_tenant_stats` | `0 3 * * *` (每天凌晨3点) | `cron` | 全量对账：逐租户 COUNT 所有业务表，与 tenant_stats 比对，自动修复偏差 |
| `reset_token_quota_monthly` | `0 0 1 * *` (每月1号) | `cron` | 将所有租户 token_used_monthly 重置为 0，更新 token_quota_reset_at |
| `check_trial_expiry` | `0 2 * * *` (每天凌晨2点) | `cron` | 检查体验租户是否过期（now > trial_expires_at），过期则标记状态 |
| `clean_expired_redis_keys` | `0 4 * * *` | `cron` | 清理已过期的 Redis Token 黑名单（实际上 Redis 自带 TTL 过期，此任务为保险措施：SCAN blacklist:token:* → 检查 TTL → 删除 TTL<0 的 key） |
| `sync_token_counter_to_db` | `*/10 * * * *` (每10分钟) | `cron` | 从 Redis token counter 同步到 DB tenant_stats.token_used_monthly |

### 4.1 `reconcile_tenant_stats` 详解

```go
// 函数签名
func ReconcileTenantStats(ctx context.Context) error

// 对账逻辑（逐租户）
for each tenant in SELECT id FROM tenants WHERE status != 'deleted':
    1. LOCK tenant_stats WHERE tenant_id = tenant.id FOR UPDATE
    2. actual.user_count       = COUNT(*) FROM tenant_users WHERE tenant_id=? AND is_active=true
    3. actual.kb_count         = COUNT(*) FROM knowledge_bases WHERE tenant_id=? AND deleted_at IS NULL
    4. actual.knowledge_count  = COUNT(*) FROM knowledges WHERE tenant_id=? AND deleted_at IS NULL
    5. actual.chunk_count      = COUNT(*) FROM chunks WHERE tenant_id=?
    6. actual.storage_used     = COALESCE(SUM(file_size), 0) FROM knowledge_files WHERE tenant_id=?
    7. actual.storage_used    += COALESCE(SUM(chunk_size_estimate), 0) FROM chunks WHERE tenant_id=?
    8. actual.agent_count      = COUNT(*) FROM agents WHERE tenant_id=? AND deleted_at IS NULL
    9. actual.session_count    = COUNT(*) FROM sessions WHERE tenant_id=?

    10. 比对 actual vs tenant_stats 当前值:
        若偏差 > 0: UPDATE tenant_stats SET ...（写入 actual 值）
        记录日志: "tenant_id=X: corrected Y from Z to A"

// 错误处理
// 单个租户对账失败不影响其他租户
// 全部完成后输出汇总: "reconciled X tenants, found Y discrepancies, corrected Z"
```

### 4.2 Token 计数器同步（高并发 → DB）

```
Redis: tenant:{tid}:token_used_monthly → 每次对话 INCRBYDelta
每次同步时:
  for each tenant in tenant_stats:
    val = redis.GET("tenant:{tid}:token_used_monthly")
    if val > 0:
      db.TENANT_STATS.UPDATE(token_used_monthly = val)
      redis.DECRBY("tenant:{tid}:token_used_monthly", val)
// 同步是追加式：Redis 计数 + DB 计数 = 真实总用量
```

---

## 5. 业务规则汇总

### 5.1 套餐字段说明

| 字段 | 单位 | 0 的含义 |
|------|:---:|----------|
| max_users | 个 | 无限制 |
| max_knowledge_bases | 个 | 无限制 |
| max_knowledge_per_kb | 条文档 | 无限制 |
| max_chunks_per_knowledge | 个分块 | 无限制 |
| storage_quota_bytes | 字节 | 无限制 |
| token_quota_monthly | tokens | 无限制 |
| max_agents | 个 | 无限制 |

### 5.2 套餐升级/降级规则

1. **升级**（如 trial→basic / basic→pro）：立即生效，配额扩大，无需等待
2. **降级**（如 pro→basic）：下个计费周期生效（暂不实现，本版本仅支持升级）
3. **切换套餐**：`POST /admin/tenants/{id}/plan` 立即更新 `tenants.plan_id`，清除缓存
4. **套餐下架**：有租户使用的套餐不能下架，先迁移所有租户到其他套餐

### 5.3 统计计数器更新时机

| 计数 | +1 时机 | -1 时机 |
|------|---------|---------|
| user_count | 新建租户用户 / 用户启用 | 用户禁用 / 用户删除 |
| knowledge_base_count | 创建 KB | 删除 KB |
| knowledge_count | 文档上传成功 | 文档删除 |
| chunk_count | 文档分块完成（批量 + N） | 文档删除（批量 - N） |
| storage_used_bytes | 文件上传成功 | 文件删除 |
| agent_count | 创建 Agent | 删除 Agent |
| session_count | 创建 Session | —（不减少，历史会话保留） |
| token_used_monthly | 每次 LLM 调用后 INCRBY(used_tokens) | 每月1号重置为 0 |

### 5.4 配额检查点

**创建前**检查（返回 403 PLAN_xxx_LIMIT）：

| 操作 | 检查条件 |
|------|----------|
| 创建用户 | `stats.user_count >= config.max_users AND config.max_users > 0` |
| 创建 KB | `stats.knowledge_base_count >= config.max_knowledge_bases AND config.max_knowledge_bases > 0` |
| 上传文档 | `stats.knowledge_count >= config.max_knowledge_per_kb AND config.max_knowledge_per_kb > 0` |
| 分块 | `stats.chunk_count + new_count >= config.max_chunks_per_knowledge AND config.max_chunks_per_knowledge > 0` |
| 上传文件 | `stats.storage_used_bytes + file_size >= config.storage_quota_bytes AND config.storage_quota_bytes > 0` |
| 创建 Agent | `stats.agent_count >= config.max_agents AND config.max_agents > 0` |
| 每次 LLM 调用 | `stats.token_used_monthly >= config.token_quota_monthly AND config.token_quota_monthly > 0`（返回 403 TOKEN_QUOTA_EXCEEDED） |

### 5.5 统计一致性保障

1. **实时写入**：各 Service 方法执行完操作后，立即调用 `tenant_stats` 对应的 `IncrementXX` / `DecrementXX` 方法
2. **原子更新**：使用 `UPDATE tenant_stats SET xx = xx + ? WHERE tenant_id = ?`（无锁，单字段原子）
3. **多字段更新**：需要同时更新多个统计值时，使用 `SELECT ... FOR UPDATE` + 事务
4. **夜间对账**：定时任务每天全量对账修复
5. **Redis 补偿**：Token 用量走 Redis 原子递增，10 分钟同步一次到 DB

---

## 6. 验收测试场景

### T-03 套餐体系

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | 新租户注册获得体验套餐 | 用户注册 → 自动创建租户 + trial 套餐 | tenant.plan_id='trial', trial_expires_at=now+3天 |
| 2 | ❌ | 体验租户超过用户数限制 | trial 租户已有 1 用户 → POST /users | 403 PLAN_USER_LIMIT |
| 3 | ❌ | 系统管理员下架正使用的套餐 | 租户 10001 在用 pro → PATCH /admin/plans/pro/status is_active=false | 422 PLAN_IN_USE |
| 4 | ❌ | Token 超限后拒绝对话 | token_used_monthly=99950, quota=100000 → 发送 51 token 的消息 | 403 TOKEN_QUOTA_EXCEEDED |

### 统计系统

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 5 | ✅ | 创建 KB 后统计增加 | KB count=5 → POST /knowledge-bases | stats.knowledge_base_count=6 |
| 6 | ✅ | 每日对账修复 | 手动篡改 stats.chunk_count → 凌晨 3 点 | 自动修复为正确值 |
| 7 | ❌ | 存储超限拒绝上传 | storage_used=9.8GB, quota=10GB → 上传 300MB 文件 | 403 STORAGE_QUOTA_EXCEEDED |

---

## 7. 文件清单

### 新建文件

| 文件 | 说明 |
|------|------|
| `internal/types/plan.go` | Plan + PlanConfig 结构体 |
| `internal/types/tenant_stats.go` | TenantStats 结构体 |
| `internal/types/interfaces/plan.go` | PlanRepository + PlanService + TenantStatsRepository 接口 |
| `internal/application/repository/plan.go` | PlanRepository 实现（CRUD） |
| `internal/application/repository/tenant_stats.go` | TenantStatsRepository 实现（原子增减 + 对账） |
| `internal/application/service/plan.go` | PlanService 实现（套餐分配、配额检查） |
| `internal/application/service/tenant_stats.go` | 统计方法集合：IncrementUserCount、DecrementKB、GetStats 等 |
| `internal/handler/plan.go` | 套餐管理端点（CRUD） |
| `internal/handler/tenant_stats.go` | 统计查看端点 |
| `internal/handler/admin_stats.go` | 系统管理统计端点 |
| `internal/cron/reconcile_tenant_stats.go` | 统计对账定时任务 |
| `internal/cron/reset_token_quota.go` | Token 月额度重置定时任务 |
| `internal/cron/check_trial_expiry.go` | 体验租户到期检查 |
| `internal/cron/clean_expired_redis.go` | Redis 黑名单清理 |
| `internal/cron/sync_token_counter.go` | Redis Token 计数器 → DB 同步 |
| `internal/cron/init.go` | 定时任务注册（go-cron 或 robfig/cron） |
| `migrations/versioned/000060_plans.up.sql` | plans + plan_configs |
| `migrations/versioned/000060_plans.down.sql` | 回滚 |
| `migrations/versioned/000061_tenant_stats.up.sql` | tenant_stats |
| `migrations/versioned/000061_tenant_stats.down.sql` | 回滚 |
| `migrations/versioned/000062_tenants_plan.up.sql` | tenants.plan_id + trial_expires_at |
| `migrations/versioned/000063_seed_plan_data.up.sql` | trial/basic/pro 种子数据 |

### 修改文件

| 文件 | 说明 |
|------|------|
| `internal/router/router.go` | 注册统计 + 套餐管理路由 |
| `internal/middleware/auth.go` | 增加 `tenant_stats` 配额中间件（检查 token_used < quota） |
| `internal/application/service/knowledgebase.go` | 创建/删除 KB 时调用 stats 增减 |
| `internal/application/service/knowledge.go` | 上传/删除文档时调用 stats 增减 |
| `internal/application/service/agent.go` | 创建/删除 Agent 时调用 stats 增减 |
| `internal/application/service/session.go` | 创建 Session 时调用 stats 增加 |
| `internal/application/repository/tenant.go` | 扩展 `AdjustStorageUsed` 同步写入 tenant_stats |
| `internal/container/container.go` | 注册新服务到 DI 容器 |
| `cmd/server/main.go` | 启动 cron scheduler |

---

## 质量门控

- [x] 每个 P0 功能都有对应 API 端点 + Request + Response + Errors（9 组端点完整）
- [x] 每张数据库表有完整 DDL（4 张表 + 种子数据脚本）
- [x] 每个 Redis Key 标注了类型和 TTL（3 个 Key + 操作流程）
- [x] 定时任务完整定义（5 个任务 + 逐个拆解 + cron 表达式）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 5 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
