# 卡片 07：系统管理员后台

> 覆盖：T-11（系统管理员后台 API — 租户管理 + 后台端点的完整收口）
> 依赖：卡片 01-06 全部定义完毕
> 基准需求：`specs/REQUIREMENTS.md` v1.4 第 11 节

---

## 1. API 契约

所有端点前缀 `/api/v1/admin`，全部需要 `system_admin` 角色。
中间件链：`AuthMiddleware` → `RequireRole("system_admin")`

### 1.1 租户列表

```
GET /api/v1/admin/tenants

描述：系统管理员查看所有租户列表（分页 + 搜索 + 筛选）

Query:
  page       (int, default=1)
  page_size  (int, default=20, max=100)
  keyword    (string, optional — 按名称模糊搜索)
  status     (string, optional — active | disabled | trial_expired)
  plan_id    (string, optional — trial | basic | pro)

Response 200:
{
  "data": [
    {
      "tenant_id": 10042,
      "name": "张三的知识空间",
      "plan_id": "trial",
      "plan_name": "体验版",
      "status": "active",
      "is_system": false,
      "trial_expires_at": "2026-05-23T10:30:00Z",
      "user_count": 1,
      "storage_used_bytes": 104857600,
      "token_used_monthly": 12000,
      "created_at": "2026-05-20T10:30:00Z"
    },
    ...
  ],
  "total": 42,
  "page": 1,
  "page_size": 20
}
```

### 1.2 租户详情

```
GET /api/v1/admin/tenants/{tenant_id}

描述：获取单个租户详细信息

Response 200:
{
  "data": {
    "tenant_id": 10042,
    "name": "张三的知识空间",
    "description": "",
    "plan_id": "trial",
    "plan_name": "体验版",
    "status": "active",
    "is_system": false,
    "trial_expires_at": "2026-05-23T10:30:00Z",
    "storage_quota_bytes": 524288000,
    "created_at": "2026-05-20T10:30:00Z",
    "users": [                               // 租户内用户列表
      {
        "user_id": "uuid-xxx",
        "username": "zhangsan",
        "email": "zhangsan@example.com",
        "role": "tenant_admin",
        "is_active": true,
        "joined_at": "2026-05-20T10:30:00Z"
      }
    ],
    "stats": {                               // 统计数据快照
      "knowledge_base_count": 2,
      "knowledge_count": 15,
      "chunk_count": 340,
      "storage_used_bytes": 104857600,
      "token_used_monthly": 12000,
      "token_used_total": 12000,
      "agent_count": 0,
      "session_count": 8
    }
  }
}

Errors:
  404  RESOURCE_NOT_FOUND
```

### 1.3 编辑租户信息

```
PUT /api/v1/admin/tenants/{tenant_id}

描述：编辑租户名称、描述等基本信息

Request Body:
{
  "name": "XX科技有限公司",                // string, optional, ≤128 chars
  "description": "企业知识管理平台"          // string, optional
}

Response 200:
{
  "data": {
    "tenant_id": 10042,
    "name": "XX科技有限公司",
    "description": "企业知识管理平台",
    "updated_at": "2026-05-20T15:00:00Z"
  }
}

Errors:
  400  VALIDATION_ERROR
  404  RESOURCE_NOT_FOUND
  409  TENANT_NAME_EXISTS   — 租户名全局唯一
  422  SYSTEM_TENANT_PROTECTED — 不允许编辑系统租户
```

### 1.4 禁用/启用租户

```
PATCH /api/v1/admin/tenants/{tenant_id}/status

描述：禁用或启用一个租户。
      禁用 = status='disabled'（所有 API 返回 403 TENANT_DISABLED）
      启用 = status='active'

Request Body:
{
  "status": "disabled"                   // string, required — active | disabled
                                          // 不允许设为 trial_expired（由 cron 自动设置）
}

Response 200:
{
  "data": {
    "tenant_id": 10042,
    "status": "disabled",
    "updated_at": "2026-05-20T15:10:00Z"
  }
}

Errors:
  404  RESOURCE_NOT_FOUND
  422  SYSTEM_TENANT_PROTECTED — 不允许禁用系统租户(tenant_id=1)
  422  INVALID_STATUS        — 不允许设为 trial_expired

副作用:
  - 禁用租户时：该租户所有活跃 Token 加入黑名单
```

### 1.5 系统管理员查看任意租户的用户列表

```
GET /api/v1/admin/tenants/{tenant_id}/users

描述：系统管理员查看指定租户下的用户列表（复用卡片 01 1.1 的逻辑，但视角是 system_admin）

Query:
  page / page_size / keyword / role / status 同卡片01 1.1

Response 200: 同卡片 01 1.1
```

### 1.6 全局 Dashboard 统计

```
GET /api/v1/admin/stats/overview

描述：全局运营 Dashboard 汇总数据

Response 200:
{
  "data": {
    "total_tenants": 42,
    "active_tenants": 38,
    "disabled_tenants": 2,
    "trial_expired_tenants": 2,
    "total_users": 560,
    "total_knowledge_bases": 180,
    "total_storage_used_bytes": 107374182400,
    "total_token_used_all_time": 125000000,
    "total_token_used_this_month": 42000000,
    "trials_active": 12,
    "trials_converted": 8,
    "plans_distribution": {
      "trial": 25,
      "basic": 12,
      "pro": 5
    },
    "daily_new_tenants": [              // 近 30 天新租户数（用于趋势图）
      {"date": "2026-04-21", "count": 3},
      {"date": "2026-04-22", "count": 1},
      ...
    ]
  }
}
```

### 1.7 已有端点引用（不再重复定义，仅列清单）

| 端点 | 定义处 | 说明 |
|------|:------:|------|
| `GET /api/v1/admin/stats/tenants` | 卡片 02 1.2 | 所有租户统计列表 |
| `POST /api/v1/admin/plans` | 卡片 02 1.6.1 | 创建套餐 |
| `PUT /api/v1/admin/plans/{id}` | 卡片 02 1.6 | 编辑套餐 |
| `DELETE /api/v1/admin/plans/{id}` | 卡片 02 1.6 | 删除套餐 |
| `PATCH /api/v1/admin/plans/{id}/status` | 卡片 02 1.6.2 | 启用/下架套餐 |
| `POST /api/v1/admin/tenants/{id}/plan` | 卡片 02 1.7 | 分配/切换租户套餐 |

---

## 2. 数据库 DDL

本卡片无新建表。所有操作复用已有表：

| 表 | 涉及的操作 |
|----|-----------|
| `tenants` | 列表查询、详情查询、名称编辑、状态变更 |
| `tenant_users` | 列出租户用户（JOIN users） |
| `tenant_stats` | 统计 Dashboard 汇总（COUNT + SUM + GROUP BY） |
| `plans` / `plan_configs` | 套餐管理（已在卡片 02） |

---

## 3. Redis Key Schema

本卡片不新增 Redis Key。涉及的 Key：

| Key | 定义处 | 管理员操作时的清除逻辑 |
|-----|:----:|------|
| `menu:{tid}:{uid}` | 卡片 05 | 禁用租户时 → 清理该租户所有用户的菜单缓存 |

---

## 4. 定时任务

本卡片无独立定时任务。

---

## 5. 业务规则汇总

### 5.1 系统租户保护

| 受保护的资源 | 保护规则 |
|-------------|----------|
| 系统租户 (tenant_id=1) | 不可编辑名称、不可禁用、不可删除、不可切换套餐 |
| 系统管理员用户 (sys-admin-001) | 不可从 system_admin 降级（角色的保护在卡片 01 中处理） |
| system_admin 角色 | 不可通过租户用户管理 API 分配（仅种子脚本预置） |

### 5.2 禁用租户的连锁反应

```
系统管理员 PATCH /admin/tenants/{id}/status → disabled
  ├─ tenants.status = 'disabled'
  ├─ 该租户所有在线用户的 Token 加入黑名单
  ├─ 清除该租户所有用户的菜单缓存 (menu:{tid}:*)
  ├─ 租户内所有用户下次 API 请求 → 403 TENANT_DISABLED
  └─ 不删除数据（KB、文档、Session 保留）
```

### 5.3 Dashboard 统计查询

```sql
-- total_tenants
SELECT COUNT(*) FROM tenants;

-- active_tenants
SELECT COUNT(*) FROM tenants WHERE status = 'active';

-- plans_distribution
SELECT plan_id, COUNT(*) as cnt FROM tenants GROUP BY plan_id;

-- total_users (active)
SELECT COUNT(*) FROM tenant_users tu
JOIN tenants t ON tu.tenant_id = t.id
WHERE t.status != 'deleted';

-- daily_new_tenants (近 30 天)
SELECT DATE(created_at) as date, COUNT(*) as count
FROM tenants
WHERE created_at >= NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at)
ORDER BY date;

-- total_storage / total_token
SELECT
  SUM(storage_used_bytes) as total_storage,
  SUM(token_used_total) as total_token_all,
  SUM(token_used_monthly) as total_token_month
FROM tenant_stats;

-- trials_active (体验套餐且在试用期内)
SELECT COUNT(*) FROM tenants
WHERE plan_id = 'trial'
  AND trial_expires_at > NOW()
  AND status = 'active';

-- trials_converted (从 trial 转为付费套餐)
SELECT COUNT(*) FROM tenants
WHERE plan_id != 'trial'
  AND EXISTS (
    SELECT 1 FROM tenant_plan_history WHERE tenant_id = tenants.id AND from_plan = 'trial'
  );
```

### 5.4 错误码扩展

| 错误码 | HTTP | 含义 |
|--------|:----:|------|
| `SYSTEM_TENANT_PROTECTED` | 422 | 不允许操作系统租户 |
| `TENANT_DISABLED` | 403 | 租户已被禁用，所有 API 不可用 |
| `INVALID_STATUS` | 422 | 非法状态值 |

---

## 6. 验收测试场景

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | system_admin 查看全局 Dashboard | GET /admin/stats/overview | 返回 total_tenants、plans_distribution、daily_new_tenants 等 |
| 2 | ✅ | system_admin 禁用违规租户 | PATCH /admin/tenants/10042/status disabled | 200，该租户用户无法继续使用 API |
| 3 | ✅ | system_admin 编辑租户名称 | PUT /admin/tenants/10042 {name:"新名称"} | 200 更新成功 |
| 4 | ❌ | 普通 tenant_admin 访问系统管理 | tenant_admin GET /admin/stats/overview | 403 PERMISSION_DENIED |
| 5 | ❌ | 尝试禁用系统租户 | PATCH /admin/tenants/1/status disabled | 422 SYSTEM_TENANT_PROTECTED |
| 6 | ❌ | 尝试设置非法租户状态 | PATCH /admin/tenants/10042/status trial_expired | 422 INVALID_STATUS |
| 7 | ❌ | 重复租户名 | PUT /admin/tenants/10042 {name:"已存在的租户名"} | 409 TENANT_NAME_EXISTS |

---

## 7. 文件清单

### 新建文件

| 文件 | 说明 |
|------|------|
| `internal/handler/admin_tenant.go` | AdminTenantHandler：ListTenants、GetTenant、UpdateTenant、SetTenantStatus、GetTenantUsers |
| `internal/handler/admin_stats.go` | AdminStatsHandler：GetOverview（已在卡片 02 创建文件，此处补充 overview 端点） |
| `internal/application/service/admin.go` | AdminService：租户管理业务逻辑（禁用连锁操作、系统租户保护校验） |

### 修改文件

| 文件 | 说明 |
|------|------|
| `internal/router/router.go` | 注册 `/api/v1/admin/tenants/*` 路由组（含所有管理端点），`RequireRole("system_admin")` |
| `internal/application/service/plan.go` | （已在卡片 02）可能需补充系统租户保护：切换套餐时禁止操作 ID=1 |
| `internal/application/repository/tenant.go` | 新增 `FindAll(query)`（分页+搜索+筛选） |
| `internal/application/repository/tenant_stats.go` | 新增 `GetOverview()`（Dashboard 汇总查询） |
| `internal/container/container.go` | 注册 AdminService、AdminTenantHandler |

---

## 质量门控

- [x] 每个 P0 功能都有对应 API 端点 + Request + Response + Errors（6 个新端点 + 7 个引用端点）
- [x] 每张数据库表有完整 DDL（复用已有表，无新建）
- [x] 每个 Redis Key 标注了类型和 TTL（缓存清除逻辑已覆盖）
- [x] 定时任务已标注（无独立任务）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 4 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
