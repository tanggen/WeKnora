# 卡片 03：认证与注册

> 覆盖：T-08（注册 + 登录 + JWT 扩展 + Token 黑名单 + 首次登录改密）
> 依赖：卡片 01（tenant_users 表）、卡片 02（tenant_stats、plans）
> 基准需求：`specs/REQUIREMENTS.md` v1.4 第 8 节

---

## 1. API 契约

### 端点清单

#### 1.1 用户注册

```
POST /api/v1/auth/register

描述：用户自助注册。注册后自动创建体验租户，用户为该租户的 tenant_admin。
      体验套餐 trial，3 天有效期。
      ⚠️ 不同于租户管理员创建用户（卡片01 1.2），自助注册者直接获得 tenant_admin

Request Body:
{
  "username": "zhangsan",              // string, required, 2-64 chars, ^[a-zA-Z0-9_]+$
  "email": "zhangsan@example.com",     // string, required, email格式, ≤255 chars
  "password": "Abc12345",              // string, required, 8-128 chars, 必须含大写+小写+数字
  "phone": "13800138000",              // string, optional, ^1[3-9]\d{9}$
  "tenant_name": "张三的知识空间"       // string, optional, ≤128 chars, 默认取 username+"的空间"
}

Response 201:
{
  "data": {
    "user_id": "uuid-xxx",
    "tenant_id": 10042,
    "tenant_name": "张三的知识空间",
    "role": "tenant_admin",
    "plan_id": "trial",
    "trial_expires_at": "2026-05-23T10:30:00Z",
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 86400,               // access_token 有效期（秒），默认 24h
    "token_type": "Bearer"
  }
}

Errors:
  400  VALIDATION_ERROR       — 字段校验失败
  409  USERNAME_EXISTS        — 用户名已被使用（全局）
  409  EMAIL_EXISTS           — 邮箱已被使用（全局）
  409  PHONE_EXISTS           — 手机号已被使用（全局）
  409  TENANT_NAME_EXISTS     — 租户名称已被使用（可选检查）
  422  WEAK_PASSWORD          — 密码不满足复杂度要求
  429  RATE_LIMIT_EXCEEDED    — 同 IP 注册超过限制（Redis ratelimit:register:{ip}）

副作用:
  - 创建 users 行（password_expired=false，注册者密码凭据自选，无需首次强制修改）
  - 创建 tenants 行（plan_id="trial", trial_expires_at=now+3天）
  - 创建 tenant_users 行（role="tenant_admin"）
  - 初始化 tenant_stats 行
  - tenant_stats.user_count = 1
```

#### 1.2 用户登录

```
POST /api/v1/auth/login

描述：用户登录。JWT payload 扩展含 role + permissions + must_change_password 标记。

Request Body:
{
  "account": "zhangsan@example.com",   // string, required — username 或 email 或 phone
  "password": "Abc12345"              // string, required
}
// account 字段智能识别：含 @ → email，全数字且11位 → phone，其他 → username

Response 200（正常）:
{
  "data": {
    "user_id": "uuid-xxx",
    "tenant_id": 10042,
    "username": "zhangsan",
    "role": "tenant_admin",
    "permissions": ["kb:create", "kb:edit", ...],
    "plan_id": "trial",
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 86400,
    "token_type": "Bearer"
  }
}

Response 423（首次登录/密码过期）:
{
  "error": {
    "code": "FIRST_LOGIN_RESET",
    "message": "首次登录或密码已过期，请修改密码"
  },
  "data": {
    "user_id": "uuid-xxx",
    "reset_token": "eyJhbGciOi..."         // 临时 Token，仅用于改密，5 分钟有效
  }
}

Errors:
  400  VALIDATION_ERROR
  401  INVALID_CREDENTIALS   — 账号或密码错误
  403  ACCOUNT_DISABLED      — 用户已被禁用
  403  TENANT_DISABLED       — 所属租户已被禁用
  429  RATE_LIMIT_EXCEEDED   — 同 IP 登录失败超过限制（Redis ratelimit:login:{ip}）
```

#### 1.3 修改密码（登录后）

```
POST /api/v1/auth/change-password

描述：用户主动修改密码，或首次登录强制修改。

Request Body:
{
  "old_password": "Abc12345",          // string, required（首次登录时可不传，用 reset_token）
  "new_password": "NewPass456"         // string, required, 8-128 chars
}
// Header: Authorization: Bearer <access_token>
// 首次登录场景: Authorization: Bearer <reset_token>（无 old_password 字段）

Response 200:
{
  "data": {
    "message": "密码修改成功"
  }
}

Errors:
  400  VALIDATION_ERROR
  401  INVALID_CREDENTIALS   — 旧密码不正确
  422  WEAK_PASSWORD         — 新密码不满足复杂度要求
  422  SAME_PASSWORD         — 新旧密码相同

副作用:
  - users.password_hash = bcrypt(new_password)
  - users.password_expired = false
```

#### 1.4 登出

```
POST /api/v1/auth/logout

权限：任何已登录用户
描述：登出，将当前 Token 的 jti 加入黑名单

Request:
  Header: Authorization: Bearer <access_token>

Response 200:
{
  "data": {
    "message": "已登出"
  }
}

副作用:
  - 解析当前 JWT 的 jti
  - SET blacklist:token:{jti} "1" EX {remaining_ttl}
```

#### 1.5 刷新 Token

```
POST /api/v1/auth/refresh

描述：使用 refresh_token 换取新的 access_token

Request Body:
{
  "refresh_token": "eyJhbGciOi..."
}

Response 200:
{
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",      // 同时刷新 refresh_token
    "expires_in": 86400,
    "token_type": "Bearer"
  }
}

Errors:
  401  INVALID_TOKEN          — refresh_token 无效或已过期
  403  ACCOUNT_DISABLED       — 用户已被禁用
```

---

## 2. 数据库 DDL

### 2.1 修改表：`users`

```sql
-- 新增字段
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
ALTER TABLE users ADD COLUMN password_expired BOOLEAN NOT NULL DEFAULT FALSE;
-- password_expired = TRUE 意味着该用户需要强制修改密码

-- 部分唯一索引
CREATE UNIQUE INDEX idx_users_phone
    ON users(phone) WHERE phone IS NOT NULL AND phone != '';

-- 将现有种子 admin 用户标记为 password_expired = TRUE（种子脚本创建时也标记）
-- 系统管理员的默认密码 admin_123456，未改过密码的需强制修改

-- 注意：自助注册用户的 password_expired = FALSE（他们自己设的密码）
-- 管理员创建的用户的 password_expired = TRUE（系统随机密码，首次登录强制修改）
```

### 2.2 迁移编号

```
migrations/versioned/000070_users_auth.up.sql — phone + password_expired
```

---

## 3. Redis Key Schema

### 本卡片管理的 Key

| Key | 类型 | TTL | 用途 |
|-----|------|:---:|------|
| `blacklist:token:{jti}` | String | =JWT exp - now() | Token 黑名单。登出/禁用/删除用户时写入 |
| `ratelimit:register:{ip}` | String | 窗口期 | IP 注册频率限制。格式: `计数器:窗口起始时间戳` |
| `ratelimit:login:{ip}` | String | 窗口期 | IP 登录频率限制（失败次数计数） |
| `reset:token:{reset_jti}` | String | 5 分钟 | 首次登录改密临时 Token 黑名单（用完即加入） |

### 频率限制实现

```
# 注册限流（Window: 1小时，Max: 3次/IP）
key = "ratelimit:register:192.168.1.1"
方案: 滑动窗口计数器
  每次注册前: INCR key, 若首次 → EXPIRE key 3600
  若 count > 3 → 返回 429

# 登录限流（Window: 15分钟，Max: 10次失败/IP）
key = "ratelimit:login:192.168.1.1"
方案: 同上
  登录成功 → DEL key（重置计数器）
  登录失败 → INCR key, 若 count > 10 → 返回 429
```

### Token 黑名单完整流程

```
┌── 正常登录 ──────────────────────────────────────┐
│  POST /auth/login                                │
│    → jwt = Generate(user_id, tenant_id, role,     │
│             permissions, jti=uuid, exp=now+24h)   │
│    → 返回 access_token + refresh_token            │
└──────────────────────────────────────────────────┘

┌── 登出 ──────────────────────────────────────────┐
│  POST /auth/logout                               │
│    → jti = parseJWT(token).jti                   │
│    → ttl = parseJWT(token).exp - now()           │
│    → SET blacklist:token:{jti} "1" EX {ttl}      │
└──────────────────────────────────────────────────┘

┌── 禁用用户 ──────────────────────────────────────┐
│  PATCH /users/{id}/status is_active=false        │
│    → 查询该用户的活跃 Token（需额外管理活跃Token表│
│      或 JWT 中存 user_id 方便匹配）               │
│    方案 A: 简单方案 — 不逐条失效，依赖中间件检查   │
│             user.is_active → 若 false 返回 401     │
│    方案 B: 严格方案 — 查活跃Token表 → 逐个加黑名单 │
│    ⚠️ 先采用方案 A + user 登录检查 is_active      │
│       配合方案 B（Token 加黑名单）双保险           │
└──────────────────────────────────────────────────┘

┌── 中间件校验 ────────────────────────────────────┐
│  func AuthMiddleware(c *gin.Context) {           │
│    token = extractBearer(c)                      │
│    claims = parseJWT(token)                      │
│    if EXISTS blacklist:token:{claims.jti} {      │
│      return 401 TOKEN_REVOKED                    │
│    }                                             │
│    // 额外检查（可选，用于禁用用户的简单方案）       │
│    user = db.FindUser(claims.user_id)            │
│    if !user.is_active OR !user.tenant.is_active { │
│      return 403 ACCOUNT_DISABLED                 │
│    }                                             │
│    c.Set("user_id", claims.user_id)              │
│    c.Set("tenant_id", claims.tenant_id)          │
│    c.Set("role", claims.role)                    │
│    c.Set("permissions", claims.permissions)      │
│    c.Next()                                      │
│  }                                               │
└──────────────────────────────────────────────────┘
```

---

## 4. 定时任务

本卡片无独立定时任务。`clean_expired_redis_keys` 已由卡片 02 定义。

---

## 5. 业务规则汇总

### 5.1 字段验证

| 字段 | 规则 |
|------|------|
| `account` (login) | 智能识别：含 @ → 按 email 查询；全数字 11 位 → 按 phone 查询；其他 → 按 username 查询 |
| `password` | 8-128 chars，必须含大写字母×1 + 小写字母×1 + 数字×1 |
| `new_password` | 同 password 规则，且不能与 old_password 相同 |
| `tenant_name` (register) | ≤128 chars，可选，默认 `{username}的知识空间` |

### 5.2 JWT Payload 结构

```json
{
  "sub": "uuid-xxx",            // user_id
  "tid": 10042,                 // tenant_id
  "role": "tenant_admin",       // 角色
  "perms": ["kb:create", ...],  // 权限列表
  "jti": "jti-uuid",           // Token 唯一标识（用于黑名单）
  "type": "access",             // access / refresh / reset
  "iat": 1747735200,
  "exp": 1747821300
}
```

### 5.3 Token 有效期

| Token 类型 | 有效期 | 说明 |
|------------|:------:|------|
| access_token | 24 小时 | 正常 API 访问 |
| refresh_token | 7 天 | 刷新 access_token |
| reset_token | 5 分钟 | 首次登录改密专用 |

### 5.4 注册流程

```
1. 校验输入字段
2. 检查限流 Redis ratelimit:register:{ip}
3. 检查用户名/邮箱/手机号唯一性（全局）
4. tx.Begin()
5.   INSERT users (username, email, phone, password_hash, password_expired=FALSE)
6.   INSERT tenants (name, plan_id="trial", trial_expires_at=now+3天, status="active")
7.   INSERT tenant_users (tenant_id, user_id, role="tenant_admin")
8.   INSERT tenant_stats (tenant_id, user_count=1)
9.   tenant_stats.user_count = 1 (同步更新)
10. tx.Commit()
11. 生成 JWT (含 role + permissions)
12. 返回 201 + JWT
```

### 5.5 首次登录强制改密

- **触发条件**：`users.password_expired = TRUE`
- **管理员创建的初始密码**：管理员 POST /users → 系统生成随机密码，`password_expired=TRUE` → 传给用户
  - 管理员默认密码：`admin_123456`，种子脚本标记 `password_expired=TRUE`
- **登录流程**：`password_expired=TRUE` → 返回 423 FIRST_LOGIN_RESET + `reset_token`
- **改密流程**：POST /auth/change-password（用 reset_token 认证） → 设置新密码 → `password_expired=FALSE`

### 5.6 密码复杂度校验

```
function IsStrongPassword(pwd string) bool:
    if len(pwd) < 8 or len(pwd) > 128: return false
    hasUpper = regexp.Match("[A-Z]", pwd)
    hasLower = regexp.Match("[a-z]", pwd)
    hasDigit = regexp.Match("[0-9]", pwd)
    return hasUpper && hasLower && hasDigit
```

---

## 6. 验收测试场景

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | 正常注册+登录 | POST /auth/register → POST /auth/login | 201 + 200，JWT 含 role+tenant_id+permissions |
| 2 | ✅ | 首次登录改密 | 管理员创建的账户(初始密码) → login → 423 → change-password → login | 200，新密码有效 |
| 3 | ✅ | 登出后 Token 失效 | login → 拿到 token → logout → 用旧 token 调 API | 401 TOKEN_REVOKED |
| 4 | ❌ | 注册重复邮箱 | POST /auth/register {email="exists@example.com"} | 409 EMAIL_EXISTS |
| 5 | ❌ | 弱密码注册 | POST /auth/register {password="12345678"} | 422 WEAK_PASSWORD |
| 6 | ❌ | 禁用用户登录 | admin 禁用用户 → 该用户 POST /auth/login | 403 ACCOUNT_DISABLED |
| 7 | ❌ | 注册频率限制 | 同 IP 1 小时内第 4 次 register | 429 RATE_LIMIT_EXCEEDED |
| 8 | ❌ | 登录失败频率限制 | 同 IP 15 分钟内第 11 次 login 失败 | 429 RATE_LIMIT_EXCEEDED |
| 9 | ❌ | 错误密码 | POST /auth/login {password="wrong"} | 401 INVALID_CREDENTIALS |

---

## 7. 文件清单

### 新建文件

| 文件 | 说明 |
|------|------|
| `internal/handler/auth.go` | 认证 Handler：register、login、logout、changePassword、refreshToken |
| `internal/application/service/auth.go` | AuthService：注册流程编排、登录校验、JWT 生成、Token 黑名单 |
| `internal/middleware/auth.go` | **主认证中间件**：JWT 解析 → 黑名单检查 → user 状态检查 → 注入 context（user_id/tenant_id/role/permissions） |
| `internal/utils/jwt.go` | JWT 工具函数：GenerateAccessToken、GenerateRefreshToken、GenerateResetToken、ParseToken |
| `internal/utils/password.go` | 密码工具：HashPassword(bcrypt)、VerifyPassword、IsStrongPassword、GenerateRandomPassword |
| `internal/utils/ratelimit.go` | Redis 滑动窗口限流工具 |
| `internal/utils/account.go` | account 字段智能识别（email/phone/username 路由查询） |
| `migrations/versioned/000070_users_auth.up.sql` | users.phone + password_expired |
| `migrations/versioned/000070_users_auth.down.sql` | 回滚 |

### 修改文件

| 文件 | 说明 |
|------|------|
| `internal/router/router.go` | 注册认证路由（/auth/* 组，不加 RequireRole） |
| `internal/types/user.go` | User 结构体增加 `Phone`、`PasswordExpired` 字段 |
| `internal/application/repository/user.go` | 增加 `FindByAccount(account)`（智能匹配 email/phone/username） |
| `internal/middleware/permission.go` | `RequireRole(...)` 中间件（已在卡片 01 定义，此处做 JWT 注入后的读值适配） |
| `internal/config/config.go` | jwt_secret、jwt_expiry、refresh_expiry、rate_limit 配置项 |
| `internal/container/container.go` | 注册 AuthService、AuthHandler |
| `cmd/server/main.go` | 初始化 JWT secret（从配置或 env 读取） |

---

## 质量门控

- [x] 每个 P0 功能都有对应 API 端点 + Request + Response + Errors（5 个端点完整）
- [x] 每张数据库表有完整 DDL（1 迁移文件，新增 2 列 + 1 索引）
- [x] 每个 Redis Key 标注了类型和 TTL（4 个 Key + 限流算法详细说明）
- [x] 定时任务已标注（依赖卡片 02 的 clean_expired_redis_keys）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 6 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
