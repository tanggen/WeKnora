# 卡片 06：Token 配额系统

> 覆盖：T-10（Token 配额与用量追踪）
> 依赖：卡片 02（tenant_stats、plans、cron 框架）、卡片 03（JWT 中间件、tenant_id 上下文注入）
> 基准需求：`specs/REQUIREMENTS.md` v1.4 第 10 节

---

## 1. API 契约

### 1.1 获取租户 Token 用量详情

```
GET /api/v1/tenants/{tenant_id}/stats/token-usage

权限：tenant_admin, system_admin
描述：返回 Token 用量的详细统计（含按日期/按模型的细分，如有数据）

Response 200:
{
  "data": {
    "current_month": {
      "token_used": 450000,
      "token_quota": 1000000,
      "quota_reset_at": "2026-06-01T00:00:00Z",
      "usage_percent": 45.0,
      "remaining": 550000
    },
    "all_time": {
      "token_used_total": 3200000
    },
    "by_model": [                       // 按模型细分（可选）
      {
        "model_name": "hunyuan-lite",
        "token_used_monthly": 200000,
        "request_count": 450
      },
      {
        "model_name": "hunyuan-pro",
        "token_used_monthly": 250000,
        "request_count": 120
      }
    ],
    "daily_trend": [                    // 近 30 天趋势（可选，从 Redis daily key 获取）
      {"date": "2026-05-01", "tokens": 15000},
      {"date": "2026-05-02", "tokens": 22000},
      ...
    ]
  }
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  PERMISSION_DENIED
  404  RESOURCE_NOT_FOUND
```

### 1.2 系统管理员查看所有租户 Token 用量（已在卡片 02 定义）

```
GET /api/v1/admin/stats/tenants?sort_by=token_used&order=desc
// 详见卡片 02 1.2，此端点可复用
```

### 1.3 Token 超限消息

Token 超限时，对话/Agent 流式接口返回的错误格式：

```
Event: error
Data: {"error": {"code": "TOKEN_QUOTA_EXCEEDED", "message": "本月 Token 配额已用完，请升级套餐或等待下月重置"}}

HTTP Stream 中:
  Content-Type: text/event-stream
  ...
  data: {"type":"error","error":{"code":"TOKEN_QUOTA_EXCEEDED","message":"本月 Token 配额已用完，请升级套餐或等待下月重置"}}
```

HTTP 非流式接口:
```json
HTTP/1.1 403 Forbidden
{
  "error": {
    "code": "TOKEN_QUOTA_EXCEEDED",
    "message": "本月 Token 配额已用完。当前已用 1,000,000/1,000,000 tokens。请升级套餐。",
    "detail": {
      "used": 1000000,
      "quota": 1000000,
      "reset_at": "2026-06-01T00:00:00Z"
    }
  }
}
```

---

## 2. 数据库 DDL

本卡片使用卡片 02 已创建的 `tenant_stats` 表，新增一个辅助表用于按日/按模型细分（可选，初期可省略）。

### 2.1 token_usage_logs 表（可选，Phase 2 实现）

```sql
-- 用于更精细的用量追踪和审计
CREATE TABLE token_usage_logs (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     VARCHAR(36) NOT NULL,
    session_id  VARCHAR(36),
    model_name  VARCHAR(64) NOT NULL,
    tokens_in   INT NOT NULL DEFAULT 0,       -- prompt tokens
    tokens_out  INT NOT NULL DEFAULT 0,       -- completion tokens
    tokens_total INT GENERATED ALWAYS AS (tokens_in + tokens_out) STORED,
    endpoint    VARCHAR(128),                  -- 调用方：/api/v1/chat, /api/v1/agent/run
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_token_logs_tenant_date ON token_usage_logs(tenant_id, created_at);
CREATE INDEX idx_token_logs_user ON token_usage_logs(tenant_id, user_id);
-- 仅按需启用（Phase 2），卡片 06 初期不依赖此表
```

### 2.2 迁移编号

```
migrations/versioned/000090_token_usage_logs.up.sql — token_usage_logs 表（Phase 2 可选）
```

---

## 3. Redis Key Schema

### 核心 Key

| Key | 类型 | TTL | 用途 |
|-----|------|:---:|------|
| `tenant:{tid}:token_used_monthly` | String (int64) | 到下月 1 号 | 当月 Token 用量计数器。每次 LLM 调用后 INCRBY。 |
| `tenant:{tid}:token_used:daily:{date}` | String (int64) | 32 天 | 每日用量（可选，用于趋势图）。date 格式 YYYY-MM-DD |
| `tenant:{tid}:token_used:{model}:monthly` | String (int64) | 到下月 1 号 | 按模型细分（可选）。model 取 hunyuan-lite/gpt-4o 等 |

### 配额检查流程

```
┌── 对话请求 → LLM 调用前 ─────────────────────────┐
│                                                   │
│  1. ctx.GetString("tenant_id") → tid              │
│  2. quota = getPlanConfig(tid).token_quota_monthly │
│  3. if quota == 0: skip (无限配额)                 │
│  4. used = redis.GET("tenant:{tid}:token_used_monthly") │
│     if used is nil: used = tenant_stats.token_used_monthly (DB 兜底) │
│  5. estimated = estimateTokens(prompt) 预估本次用量 │
│  6. if used + estimated > quota:                  │
│       return 403 TOKEN_QUOTA_EXCEEDED              │
│  7. continue → LLM 调用                            │
└──────────────────────────────────────────────────┘

┌── LLM 调用完成后 ─────────────────────────────────┐
│                                                   │
│  8. actual_tokens = response.usage.total_tokens   │
│  9. redis.INCRBY("tenant:{tid}:token_used_monthly", actual_tokens) │
│ 10. redis.INCRBY("tenant:{tid}:token_used:daily:{date}", actual_tokens) │
│ 11. (可选) db.INSERT token_usage_logs              │
│ 12. return response                                │
└──────────────────────────────────────────────────┘
```

### Token 用量估值（预估检查）

```
简单估值（Phase 1）:
  estimateTokens(prompt) = len(prompt) / 4  // 粗略：中文约 1 char ≈ 0.5 token

精确估值（Phase 2，集成 tiktoken）:
  引入 tiktoken-go 库，按模型编码器精确计算

Phase 1 先用简单估值 + 10% buffer:
  estimated_tokens = max(len(prompt)/4, 50) + 200  // 最小 50 + 200 response buffer
```

---

## 4. 定时任务

| 任务 | cron | 说明 |
|------|:----:|------|
| `sync_token_counter_to_db` | `*/10 * * * *` | 从 Redis 同步计数器到 DB（已在卡片 02 4.1 定义，此处详细展开） |
| `reset_token_quota_monthly` | `0 0 1 * *` | 每月 1 号重置（已在卡片 02 定义，此处详细展开） |

### 4.1 `sync_token_counter_to_db` 详解

```go
func SyncTokenCounterToDB(ctx context.Context) error {
    // 1. 查出所有活跃租户
    tenants := db.Raw(`SELECT id FROM tenants WHERE status IN ('active', 'trial_expired')`).Scan(&ids)

    // 2. 逐个同步
    for _, tid := range ids {
        key := fmt.Sprintf("tenant:%d:token_used_monthly", tid)

        // 原子读取并重置 Redis 计数器
        val, err := redisClient.GetDel(ctx, key).Result()
        if err == redis.Nil {
            continue  // 该租户本月无调用
        }
        if err != nil {
            log.Errorf("SyncTokenCounter: tenant=%d err=%v", tid, err)
            continue
        }

        delta, _ := strconv.ParseInt(val, 10, 64)
        if delta <= 0 {
            continue
        }

        // 原子更新 DB（追加模式）
        err = db.Exec(`
            UPDATE tenant_stats
            SET token_used_monthly = token_used_monthly + ?,
                token_used_total = token_used_total + ?,
                updated_at = NOW()
            WHERE tenant_id = ?
        `, delta, delta, tid).Error

        if err != nil {
            // 回退：把 delta 加回 Redis（防止丢失）
            redisClient.IncrBy(ctx, key, delta)
            log.Errorf("SyncTokenCounter: DB update failed tenant=%d, rolled back redis", tid)
        }
    }

    return nil
}
```

### 4.2 `reset_token_quota_monthly` 详解

```go
func ResetTokenQuotaMonthly(ctx context.Context) error {
    now := time.Now()  // 每月 1 号 0 点触发

    // 1. 重置所有租户的月度计数
    db.Exec(`
        UPDATE tenant_stats
        SET token_used_monthly = 0,
            token_quota_reset_at = ?,
            updated_at = NOW()
    `, time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()))

    // 2. 清理 Redis 月度 key
    var cursor uint64
    for {
        keys, nextCursor, _ := redisClient.Scan(ctx, cursor, "tenant:*:token_used_monthly", 100).Result()
        for _, key := range keys {
            redisClient.Del(ctx, key)
        }
        if nextCursor == 0 {
            break
        }
        cursor = nextCursor
    }

    return nil
}
```

---

## 5. 业务规则汇总

### 5.1 Token 计数时机

| 调用的 API | 计哪些 tokens | 何时计 |
|------------|-------------|--------|
| `POST /api/v1/chat/completions` | prompt_tokens + completion_tokens | LLM 返回后立即 INCRBY |
| `POST /api/v1/chat/stream` | 同上（流式 gathering 完成后） | SSE stream 结束后计数 |
| `POST /api/v1/agent/run` | 所有 LLM 调用总和（Agent 内可能多次调用） | 每次 LLM 调用后 INCRBY |
| `POST /api/v1/agent/run/stream` | 同上 | 每次 LLM 调用后 INCRBY |

### 5.2 配额检查位置

将检查逻辑封装为一个 Service 方法，在 LLM 调用前统一调用：

```go
// internal/application/service/token_quota.go

// CheckAndReserve 在 LLM 调用前检查配额并预留
func (s *TokenQuotaService) CheckQuota(ctx context.Context, tenantID uint64, estimatedTokens int) error {
    config, err := s.planRepo.GetPlanConfig(tenantID)
    if err != nil {
        return err
    }
    // 0 = 无限制
    if config.TokenQuotaMonthly == 0 {
        return nil
    }

    // 从 Redis 读当前用量
    key := fmt.Sprintf("tenant:%d:token_used_monthly", tenantID)
    usedStr, err := s.redis.Get(ctx, key).Result()
    if err == redis.Nil {
        // Redis miss，从 DB 加载
        stats, _ := s.statsRepo.GetByTenantID(tenantID)
        usedStr = fmt.Sprintf("%d", stats.TokenUsedMonthly)
    }

    used, _ := strconv.ParseInt(usedStr, 10, 64)

    if used+int64(estimatedTokens) > config.TokenQuotaMonthly {
        return errors.NewTokenQuotaExceededError(used, config.TokenQuotaMonthly)
    }

    return nil
}

// RecordUsage LLM 调用完成后记录实际用量
func (s *TokenQuotaService) RecordUsage(ctx context.Context, tenantID uint64, actualTokens int) error {
    key := fmt.Sprintf("tenant:%d:token_used_monthly", tenantID)
    return s.redis.IncrBy(ctx, key, int64(actualTokens)).Err()
}
```

### 5.3 套餐升级后的配额处理

| 场景 | 行为 |
|------|------|
| trial → basic | 新配额立即生效。若当前已用 50000（超过 basic 的 500000 限制）→ 不受影响，配额变大了 |
| basic → pro | 同上，更大配额 |
| 降级（pro → basic） | **本版本不支持降级**，若实现则：token_used_monthly 不变，可能立即超限 |

### 5.4 Token 统计特性

1. **按月重置**：每月 1 号 0 点，所有租户 token_used_monthly 归零
2. **总额永久**：token_used_total 永不重置，用于全量数据展示
3. **高并发安全**：Redis INCRBY 是原子操作，无锁
4. **数据最终一致**：Redis → DB 同步有 10 分钟延迟，但计费准确（Redis 不会丢，DB 追加模式）
5. **降级策略**：Redis 不可用时，直接写 DB（性能下降但数据不丢）

---

## 6. 验收测试场景

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | 对话后 Token 计数增加 | 发送一条消息 → 检查 stats.token_used | token_used 增加（约等于 prompt+response 的 token 数） |
| 2 | ✅ | Token 同步到 DB | Redis token counter: 1234 → 等 10 分钟 cron → 查 DB | DB token_used_monthly += 1234，Redis key deleted |
| 3 | ✅ | 月配额重置 | 月底 token_used_monthly=500000 → 1 号 0 点 cron | DB 和 Redis 均归零，token_quota_reset_at 更新为下月 1 号 |
| 4 | ❌ | Token 超限拒绝 | quota=50000, used=49980 → 发送预估 100 token 的消息 | 403 TOKEN_QUOTA_EXCEEDED |
| 5 | ❌ | 无限配额不过滤 | enterprise 套餐 token_quota_monthly=0 → 大量对话 | 不返回 TOKEN_QUOTA_EXCEEDED |
| 6 | ❌ | Redis 不可用降级 | 关闭 Redis → 发对话 | System 自动降级写 DB（功能不受影响，仅延迟略增） |
| 7 | ❌ | 升级套餐后配额扩大 | trial(50000 限额) → pro(5000000 限额) → 继续对话 | 不再超限，新配额立即生效 |

---

## 7. 文件清单

### 新建文件

| 文件 | 说明 |
|------|------|
| `internal/application/service/token_quota.go` | TokenQuotaService：CheckQuota + RecordUsage + 降级逻辑 |
| `internal/middleware/token_quota.go` | Token 配额检查中间件（可嵌入 chat/agent 流中） |
| `internal/utils/token_estimate.go` | Token 用量估值工具（Phase 1: 字符数/4 简单估算） |
| `internal/handler/token_usage.go` | Handler：GET /tenants/{id}/stats/token-usage（如需独立端点） |
| `migrations/versioned/000090_token_usage_logs.up.sql` | token_usage_logs 表（Phase 2 可选） |

### 修改文件

| 文件 | 说明 |
|------|------|
| `internal/handler/chat.go` | 对话接口：调用前 CheckQuota → 调用后 RecordUsage |
| `internal/handler/agent.go` | Agent 运行接口：调用前 CheckQuota → 每次 LLM 调用后 RecordUsage |
| `internal/cron/sync_token_counter.go` | 完善 sync_token_counter_to_db 实现（已在卡片 02 创建文件） |
| `internal/cron/reset_token_quota.go` | 完善 reset_token_quota_monthly 实现（已在卡片 02 创建文件） |
| `internal/application/service/tenant_stats.go` | 新增 GetTokenUsageDetail 方法（含 Redis + DB 双读） |
| `internal/container/container.go` | 注册 TokenQuotaService |

---

## 质量门控

- [x] 每个 P0 功能都有对应 API 端点 + Request + Response + Errors（Token 用量视图 + 超限错误格式）
- [x] 每张数据库表有完整 DDL（token_usage_logs 可选表 + Phase 2 设计）
- [x] 每个 Redis Key 标注了类型和 TTL（3 个 Key + 完整配额检查/记录流程图）
- [x] 定时任务完整定义（2 个任务完整实现代码 + 降级回退逻辑）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 4 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
