# 卡片 01：Tenant 用户与角色体系

> 覆盖：T-01 (Tenant语义变更) + T-05 (租户用户管理) + T-06 (角色体系)
> 依赖：无（基础设施卡片，其他卡片依赖本卡片）
> 基准需求：`specs/REQUIREMENTS.md` v1.4

---

## 1. API 契约

### 公共约定

| 约定项 | 规范 |
|--------|------|
| 前缀 | `/api/v1` |
| 认证 | Bearer Token（Header: `Authorization: Bearer <jwt>`） |
| Content-Type | `application/json` |
| 分页 | Query: `?page=1&page_size=20`，Response 含 `total`、`page`、`page_size` |
| 时间格式 | ISO 8601 (`2026-05-20T10:30:00Z`) |
| 错误格式 | `{"error": {"code": "ERROR_CODE", "message": "人类可读消息"}}` |
| 角色上下文 | 中间件从 JWT 解析后注入 `c.Set("user_id", ...)` `c.Set("tenant_id", ...)` `c.Set("role", ...)`，Handler 通过 `c.GetString(...)` 获取 |

### 端点清单

#### 1.1 列出租户内用户

```
GET /api/v1/tenants/{tenant_id}/users

权限：tenant_admin, system_admin
描述：返回租户内所有有效用户（分页）

Request:
  Path:   tenant_id (uint64, required)
  Query:  page       (int, default=1, min=1)
          page_size  (int, default=20, min=1, max=100)
          keyword    (string, optional — 按 username / email 模糊搜索)
          role       (string, optional — 按角色筛选: tenant_admin | editor | viewer)
          status     (string, optional — active | disabled)

Response 200:
{
  "data": [
    {
      "user_id": "uuid-xxx",
      "username": "zhangsan",
      "email": "zhangsan@example.com",
      "phone": "13800138000",
      "role": "editor",
      "is_active": true,
      "permissions": ["kb:create", "kb:edit:own", "agent:view", "chat:use"],
      "joined_at": "2026-05-15T08:00:00Z"
    }
  ],
  "total": 45,
  "page": 1,
  "page_size": 20
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  PERMISSION_DENIED  — 非 tenant_admin / system_admin
  403  USER_NOT_IN_TENANT — 用户不属于该租户
  404  RESOURCE_NOT_FOUND — tenant_id 不存在
```

#### 1.2 新增用户（租户管理员创建）

```
POST /api/v1/tenants/{tenant_id}/users

权限：tenant_admin, system_admin
描述：在租户下创建新用户。系统生成随机8位初始密码，首次登录时强制修改。

Request Body:
{
  "username": "zhangsan",              // string, required, 2-64 chars, 字母数字下划线
  "email": "zhangsan@example.com",     // string, required, ≤255 chars, email格式
  "phone": "13800138000",              // string, optional, 匹配正则 ^1[3-9]\d{9}$
  "role": "editor"                     // string, required
                                       //   tenant_admin: 租户管理员
                                       //   editor:       编辑者
                                       //   viewer:       查看者
                                       //   system_admin: 不允许通过此接口设置
}

Response 201:
{
  "user_id": "uuid-xxx",
  "username": "zhangsan",
  "email": "zhangsan@example.com",
  "phone": "13800138000",
  "role": "editor",
  "initial_password": "aB3xK9mQ",     // 仅创建时返回一次，后续不暴露
  "is_active": true,
  "joined_at": "2026-05-20T10:30:00Z"
}

Errors:
  400  VALIDATION_ERROR       — 字段校验失败（详情见 message）
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  PERMISSION_DENIED      — 非 tenant_admin / system_admin
  403  PLAN_USER_LIMIT        — 租户套餐用户数已达上限（检查 tenant_stats.user_count >= plans.config.max_users）
  409  USERNAME_EXISTS        — 用户名已被使用（同租户）
  409  EMAIL_EXISTS           — 邮箱已被使用（全局）
  409  PHONE_EXISTS           — 手机号已被使用（全局，仅当 phone 非空时检查）
  422  INVALID_ROLE           — role 不在 [tenant_admin, editor, viewer] 内
```

#### 1.3 编辑用户信息

```
PUT /api/v1/tenants/{tenant_id}/users/{user_id}

权限：tenant_admin, system_admin
描述：编辑用户的基本信息（用户名、邮箱、手机号）

Request Body:
{
  "username": "new_username",          // string, optional, 2-64 chars
  "email": "new_email@example.com",    // string, optional, email格式
  "phone": "13900139000"              // string, optional, 匹配正则
}
// 至少传一个字段

Response 200:
{
  "user_id": "uuid-xxx",
  "username": "new_username",
  "email": "new_email@example.com",
  "phone": "13900139000",
  "role": "editor",
  "is_active": true,
  "updated_at": "2026-05-20T11:00:00Z"
}

Errors:
  400  VALIDATION_ERROR
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  PERMISSION_DENIED
  404  RESOURCE_NOT_FOUND    — user_id 不存在
  409  USERNAME_EXISTS / EMAIL_EXISTS / PHONE_EXISTS
```

#### 1.4 禁用/启用用户

```
PATCH /api/v1/tenants/{tenant_id}/users/{user_id}/status

权限：tenant_admin, system_admin
描述：切换用户激活状态。禁用后用户 Token 立即失效（加入黑名单）。

Request Body:
{
  "is_active": false                   // bool, required — true=启用, false=禁用
}

Response 200:
{
  "user_id": "uuid-xxx",
  "is_active": false,
  "updated_at": "2026-05-20T11:10:00Z"
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  PERMISSION_DENIED
  404  RESOURCE_NOT_FOUND
  422  LAST_ADMIN            — 禁用的是租户最后一个 active 的 tenant_admin

副作用:
  - 禁用: tenant_stats.user_count -= 1
  - 启用: tenant_stats.user_count += 1
  - 禁用: 将该用户所有未过期 JWT 的 jti 加入 Redis blacklist:token:{jti}
```

#### 1.5 逻辑删除用户

```
DELETE /api/v1/tenants/{tenant_id}/users/{user_id}

权限：tenant_admin, system_admin
描述：逻辑删除用户（设置 is_active=false, deleted_at=now）。
      用户创建的 Agent/知识库保留在租户内（数据归属租户）。
      用户创建的 Session 保留但不可访问（归属个人）。

Response 200:
{
  "user_id": "uuid-xxx",
  "is_active": false,
  "deleted_at": "2026-05-20T11:20:00Z",
  "message": "用户已删除"
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  PERMISSION_DENIED
  404  RESOURCE_NOT_FOUND
  422  LAST_ADMIN            — 删除的是租户最后一个 active 的 tenant_admin

副作用:
  - tenant_stats.user_count -= 1
  - 将该用户所有未过期 JWT 的 jti 加入 Redis blacklist
```

#### 1.6 设置用户角色

```
PUT /api/v1/tenants/{tenant_id}/users/{user_id}/role

权限：tenant_admin, system_admin
描述：修改用户在租户内的角色。不能将最后一个 tenant_admin 降级。

Request Body:
{
  "role": "editor"                     // string, required
                                       // 允许: tenant_admin | editor | viewer
                                       // 不允许: system_admin（仅系统管理员可设）
}

Response 200:
{
  "user_id": "uuid-xxx",
  "role": "editor",
  "permissions": ["kb:create", "kb:edit:own", "agent:view", "chat:use"],
  "updated_at": "2026-05-20T11:30:00Z"
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  PERMISSION_DENIED
  404  RESOURCE_NOT_FOUND
  422  INVALID_ROLE          — role 不在允许范围内
  422  LAST_ADMIN            — 降级的是租户最后一个 tenant_admin
```

---

## 2. 数据库 DDL

### 2.1 新建表：`tenant_users`

```sql
CREATE TABLE tenant_users (
    tenant_id    BIGINT NOT NULL,
    user_id      VARCHAR(36) NOT NULL,
    role         VARCHAR(32) NOT NULL DEFAULT 'viewer'
                 CHECK (role IN ('system_admin', 'tenant_admin', 'editor', 'viewer')),
    permissions  JSONB NOT NULL DEFAULT '[]',
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (tenant_id, user_id),
    CONSTRAINT fk_tenant_users_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT fk_tenant_users_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_tenant_users_user_id ON tenant_users(user_id);
-- 确保一个用户只属于一个租户
```

### 2.2 修改表：`users`

```sql
-- 新增手机号
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
-- 部分唯一索引：仅非空手机号参与唯一约束
CREATE UNIQUE INDEX idx_users_phone
    ON users(phone) WHERE phone IS NOT NULL AND phone != '';

-- 废弃字段（保留兼容，后续版本删除）
-- tenant_id 字段已存在于 users 表，保留但不作为用户-租户关联依据
-- 数据迁移时需将此字段值设为 NULL
```

### 2.3 修改表：`sessions`

```sql
-- 新增 user_id 实现个人会话隔离
ALTER TABLE sessions ADD COLUMN user_id VARCHAR(36) NOT NULL DEFAULT '';
CREATE INDEX idx_sessions_tenant_user ON sessions(tenant_id, user_id);

-- 数据迁移：
--   旧会话（user_id=''）：按 tenant_id 找到该租户唯一用户并填入
--   若该租户有多个用户（理论上不会），默认归属到 tenant_admin
--   无 tenant_admin 的租户 → 创建一个 system 虚拟用户
```

### 2.4 修改表：`knowledges`

```sql
ALTER TABLE knowledges ADD COLUMN created_by VARCHAR(36);
CREATE INDEX idx_knowledges_created_by ON knowledges(tenant_id, created_by);

-- 数据迁移：
--   现有文档的 created_by 设为该租户的 tenant_admin 或 first active user
```

### 2.5 修改表：`agents`

```sql
ALTER TABLE agents ADD COLUMN created_by VARCHAR(36);
CREATE INDEX idx_agents_created_by ON agents(tenant_id, created_by);

-- 数据迁移：同 knowledges
```

### 2.6 迁移编号

```
migrations/versioned/000050_tenant_users.up.sql    — tenant_users 表 + 索引
migrations/versioned/000051_users_phone.up.sql      — users.phone
migrations/versioned/000052_sessions_userid.up.sql  — sessions.user_id
migrations/versioned/000053_created_by.up.sql       — knowledges + agents created_by
migrations/versioned/000054_migrate_to_tenant_users.up.sql — 数据迁移脚本
```

---

## 3. Redis Key Schema

| Key | 类型 | TTL | 用途 |
|-----|------|:---:|------|
| `blacklist:token:{jti}` | String | =JWT剩余有效期 | Token 黑名单。用户被禁用/删除/登出时，将其 JWT 的 `jti` 写入。中间件校验时先查此 Key |

### 操作流程

```
用户禁用时:
  jti = JWT 中解析的 jti claim
  ttl = JWT exp - now()
  SET blacklist:token:{jti} "1" EX {ttl}

中间件校验时:
  jti = 从 Authorization header 解析 JWT 的 jti
  EXISTS blacklist:token:{jti}  → 若存在返回 401 TOKEN_REVOKED
```

---

## 4. 定时任务

本卡片无独立定时任务。`clean_expired_tokens`（清理过期的 Redis blacklist key）见卡片 02。

---

## 5. 业务规则汇总

### 5.1 字段验证

| 字段 | 规则 |
|------|------|
| `username` | 2-64 字符，正则 `^[a-zA-Z0-9_]+$`，租户内唯一 |
| `email` | ≤255 字符，合法 email 格式（标准 RFC 5322 简化版），全局唯一 |
| `phone` | 可选，匹配 `^1[3-9]\d{9}$`，全局唯一（仅非空时校验） |
| `password` | 8-128 字符，必须含至少 1 大写 + 1 小写 + 1 数字 |
| `role` | 枚举：`system_admin` / `tenant_admin` / `editor` / `viewer` |
| `initial_password` | 系统生成，8 位随机 (大小写字母+数字混合)，首次登录强制修改 |

### 5.2 角色定义

| 角色 | 级别 | 说明 |
|------|:----:|------|
| `system_admin` | 系统 | 管理所有租户、所有用户、全局配置。不能通过租户用户管理 API 分配 |
| `tenant_admin` | 租户 | 集中管控：管理租户信息、用户、所有人的知识库/Agent/模型 |
| `editor` | 租户 | 创建/编辑/删除自己的知识库和 Agent，查看租户内全部资源 |
| `viewer` | 租户 | 仅查看和搜索 |

### 5.3 权限矩阵

| 权限 | system_admin | tenant_admin | editor | viewer |
|------|:---:|:---:|:---:|:---:|
| 管理所有租户（CRUD + 套餐） | ✅ | ❌ | ❌ | ❌ |
| 管理租户信息 | ✅ | ✅ | ❌ | ❌ |
| 管理租户内用户 | ✅ | ✅ | ❌ | ❌ |
| 管理模型配置 | ✅ | ✅ | ❌ | ❌ |
| 创建知识库 | ✅ | ✅ | ✅ | ❌ |
| 编辑/删除任意知识库 | ✅ | ✅ | ❌ | ❌ |
| 编辑/删除自己的知识库 | — | — | ✅ | ❌ |
| 创建 Agent | ✅ | ✅ | ✅ | ❌ |
| 编辑/删除任意 Agent | ✅ | ✅ | ❌ | ❌ |
| 编辑/删除自己的 Agent | — | — | ✅ | ❌ |
| 对话/搜索 | ✅ | ✅ | ✅ | ✅ |
| 查看 Token 用量 | ✅ | ✅ | ❌ | ❌ |

### 5.4 权限字符串

| 权限标识 | 含义 |
|----------|------|
| `kb:view` | 查看知识库 |
| `kb:create` | 创建知识库 |
| `kb:edit` | 编辑任意知识库 |
| `kb:edit:own` | 编辑自己创建的知识库 |
| `kb:delete` | 删除任意知识库 |
| `kb:delete:own` | 删除自己创建的知识库 |
| `agent:view` | 查看 Agent |
| `agent:create` | 创建 Agent |
| `agent:edit` | 编辑任意 Agent |
| `agent:edit:own` | 编辑自己创建的 Agent |
| `agent:delete` | 删除任意 Agent |
| `agent:delete:own` | 删除自己创建的 Agent |
| `chat:use` | 对话/搜索 |
| `tenant:manage` | 管理租户信息 |
| `user:view` | 查看用户列表 |
| `user:manage` | 管理用户（增删改禁用） |
| `model:manage` | 管理模型配置 |
| `system:admin` | 系统管理（跨租户操作） |

### 5.5 `tenant_users.permissions` 默认值

| 角色 | 默认 permissions JSONB |
|------|--------|
| `system_admin` | `["system:admin", "kb:*", "agent:*", "chat:use", "tenant:manage", "user:manage", "model:manage"]` |
| `tenant_admin` | `["kb:create", "kb:edit", "kb:delete", "agent:create", "agent:edit", "agent:delete", "chat:use", "tenant:manage", "user:manage", "model:manage"]` |
| `editor` | `["kb:create", "kb:edit:own", "kb:delete:own", "agent:create", "agent:edit:own", "agent:delete:own", "chat:use"]` |
| `viewer` | `["kb:view", "agent:view", "chat:use"]` |

### 5.6 核心业务规则

1. **一个用户只能属于一个租户**（`tenant_users.user_id` UNIQUE）
2. **至少保留 1 个 tenant_admin**：禁用/删除/降级最后一个 active 的 tenant_admin 时返回 `LAST_ADMIN`
3. **禁用用户**：立即失效所有 Token（写入 blacklist），`tenant_stats.user_count -= 1`
4. **逻辑删除**：`is_active=false`，`deleted_at=now()`，数据保留在租户内
5. **新建用户默认状态**：`is_active=true`，首次登录返回 `423 FIRST_LOGIN_RESET`
6. **system_admin 角色**：不能通过租户用户管理 API 分配，只能由种子脚本预置
7. **数据归属**：用户创建的 Agent/KB 数据保留在租户内，创建者的 Session 保留但不可访问
8. **套餐限制**：创建用户前检查 `tenant_stats.user_count >= plans.config.max_users`（max_users>0 时）

### 5.7 权限执行层（三层分工）✅ 已确认

| 层 | 检查内容 | 失败返回 | 实现位置 |
|----|----------|:------:|----------|
| Gin 中间件 | `c.GetString("role")` 匹配路由元数据 `required_roles` | 403 | `internal/middleware/permission.go` |
| Service 层 | 对 editor: `resource.CreatedBy == ctx.UserID`；对 admin: 跳过 | 403 | 各 Service 方法内部 |
| Handler 层 | 复杂业务规则（如 COUNT tenant_admin） | 422 | 各 Handler 函数开头 |

---

## 6. 验收测试场景

### T-01 Tenant 语义 + 数据隔离

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | 同租户 KB 共享 | A(editor) 创建 KB → B(viewer) 查询 KB 列表 | B 能看到该 KB |
| 2 | ❌ | Session 个人隔离 | A 创建 3 个 Session → B 查询 Session 列表 | B 只看到自己的 Session，看不到 A 的 |
| 3 | ❌ | 同一用户加入两个租户 | A 已在 tenant_10001 → 尝试 INSERT tenant_users(20000, A_id) | 返回 409 USER_EXISTS（UNIQUE 约束冲突） |

### T-05 用户管理

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 4 | ✅ | 管理员创建用户 | tenant_admin POST /users {username, email, role=editor} | 201，返回 initial_password，user_count+1 |
| 5 | ❌ | 移除最后一个 admin | 租户仅 1 个 tenant_admin → PUT role=viewer | 422 LAST_ADMIN |
| 6 | ❌ | 禁用用户后 Token 失效 | tenant_admin 禁用用户 B → B 用旧 Token 访问 API | 401 TOKEN_REVOKED |
| 7 | ❌ | 套餐用户数达上限 | trial 套餐 max_users=1，已有 1 用户 → 新增 | 403 PLAN_USER_LIMIT |
| 8 | ❌ | 非 admin 访问用户管理 | editor 调用 GET /users | 403 PERMISSION_DENIED |

### T-06 角色权限

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 9 | ✅ | editor 删除自己创建的 KB | editor 创建 KB → 删除该 KB | 200 成功 |
| 10 | ❌ | editor 删除别人创建的 KB | editor_A 创建的 KB → editor_B 删除 | 403 PERMISSION_DENIED |
| 11 | ❌ | viewer 创建 KB | viewer POST /knowledge-bases | 403 PERMISSION_DENIED |

---

## 7. 文件清单

### 新建文件

| 文件 | 说明 |
|------|------|
| `internal/types/tenant_user.go` | TenantUser 结构体定义（GORM model） |
| `internal/types/interfaces/tenant_user.go` | TenantUserRepository + TenantUserService 接口 |
| `internal/application/repository/tenant_user.go` | TenantUserRepository 实现 |
| `internal/application/service/tenant_user.go` | TenantUserService 实现（含业务逻辑 + 权限校验） |
| `internal/handler/tenant_user.go` | HTTP Handler（6 个端点） |
| `internal/middleware/permission.go` | 三层权限中间件（Layer 1: role 检查 + Layer 3 辅助函数 `RequireRole(...)` |
| `migrations/versioned/000050_tenant_users.up.sql` | tenant_users 建表 |
| `migrations/versioned/000050_tenant_users.down.sql` | tenant_users 删表 |
| `migrations/versioned/000051_users_phone.up.sql` | users.phone 加列 |
| `migrations/versioned/000051_users_phone.down.sql` | users.phone 删列 |
| `migrations/versioned/000052_sessions_userid.up.sql` | sessions.user_id 加列+索引 |
| `migrations/versioned/000052_sessions_userid.down.sql` | sessions.user_id 删列 |
| `migrations/versioned/000053_created_by.up.sql` | knowledges + agents created_by |
| `migrations/versioned/000053_created_by.down.sql` | 回滚 |
| `migrations/versioned/000054_migrate_to_tenant_users.up.sql` | 数据迁移脚本 |
| `migrations/versioned/000054_migrate_to_tenant_users.down.sql` | 回滚 |

### 修改文件

| 文件 | 说明 |
|------|------|
| `internal/router/router.go` | 注册 6 个用户管理路由（加 RequireRole 中间件） |
| `internal/middleware/auth.go` | JWT 解析增加 `role`、`permissions` 注入 context |
| `internal/types/user.go` | User 结构体增加 `Phone` 字段，废弃 `TenantID` 的文档注释 |
| `internal/types/session.go` | Session 结构体增加 `UserID` 字段 |
| `internal/types/knowledge.go` | Knowledge 结构体增加 `CreatedBy` 字段 |
| `internal/types/agent.go` | Agent 结构体增加 `CreatedBy` 字段 |
| `internal/application/repository/session.go` | `ListSessions` / `GetSession` 查询增加 `user_id` 过滤条件 |
| `internal/application/service/session.go` | 创建 Session 时写入 `user_id = ctx.UserID` |
| `internal/container/container.go` | 注册 TenantUserRepository、TenantUserService、TenantUserHandler 到 DI 容器 |

---

## 质量门控

- [x] 每个 P0 功能都有对应 API 端点 + Request + Response + Errors（6 个端点完整）
- [x] 每张数据库表有完整 DDL（5 张表/迁移，含约束+索引+数据迁移脚本）
- [x] 每个 Redis Key 标注了类型和 TTL（1 个 Key）
- [x] 定时任务已标注（本卡片无独立任务，关联卡片已覆盖）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 8 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
