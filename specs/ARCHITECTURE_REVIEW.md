# WeKnora SaaS 化改造 — 架构评审报告

> 需求基准：`specs/REQUIREMENTS.md` v1.4 | 评审日期：2026-05-20 | 更新：已补全 P0 缺口

---

## 评审总览

| 维度 | v1.3 | v1.4 | 评级 |
|------|:---:|:---:|:----:|
| API 契约 | 45% | **78%** | ✅ 主要端点已定义 Request/Response/Errors |
| 数据模型 DDL | 70% | **92%** | ✅ tenant_users/sessions/users/created_by 完整 |
| Redis Schema | 0% | **75%** | ✅ 6 个 Key 已定义，含 TTL + 用途 + 配额流程 |
| 定时任务 | 40% | **85%** | ✅ 5 个 Cron 已定义 |
| 验收标准 | 0% | **55%** | ⚠️ 5 个核心模块有验收场景，部分模块待补 |
| 环境配置 | 20% | **60%** | ⚠️ 核心配置已定义 |
| **综合** | **29%** | **74%** | 🟢 可进入阶段 3 |

---

## 一、逐模块审计

### T-01：Tenant 语义变更

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | Session 查询增加 `user_id` 过滤（现有端点改造见卡片阶段） |
| DDL | ✅ | `tenant_users` 完整 DDL 含 PK/FK/UNIQUE/CHECK；`sessions.user_id`；`users.phone`；`knowledges.created_by`；`agents.created_by` |
| Redis | — | — |
| 任务 | ✅ | 无异步需求 |
| 验收 | ✅ | 3 条场景 |
| 配置 | ✅ | 无新增配置 |

---

### T-02：系统默认租户

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | 无 API |
| DDL | ✅ | `tenants.is_system` 已在种子 DDL 体现 |
| Redis | — | — |
| 任务 | ⚠️ | 强制修改密码：通过登录接口返回 423 FIRST_LOGIN_RESET 实现（见 T-08） |
| 验收 | ⚠️ | 待补 |
| 配置 | ⚠️ | 默认 admin 密码来源 → **见下方决策汇报** |

---

### T-03：套餐体系

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | CRUD 端点已在 T-11 定义，错误码已补全（PLAN_DISABLED/PLAN_*_LIMIT） |
| DDL | ✅ | `plans` 表 DDL 完整 |
| Redis | ✅ | `plan:{pid}:config` String, TTL=1h |
| 任务 | ⚠️ | 套餐停用后正在进行的对话 → **见下方决策汇报** |
| 验收 | ✅ | 3 条场景 |
| 配置 | ✅ | — |

---

### T-04：体验租户

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | 复用 T-08 注册 |
| DDL | ✅ | 种子含 `tenant_stats(tenant_id=2)` 初始化 |
| Redis | — | — |
| 任务 | ✅ | `check_trial_expiry` 每天 01:00 |
| 验收 | ⚠️ | 含在 T-08 场景中 |
| 配置 | ✅ | 3 个配置项 |

---

### T-05：租户用户管理

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | 6 端点 + Scenario B Request/Errors 已定义 |
| DDL | ✅ | `tenant_users` 完整 DDL（见 T-01） |
| Redis | ✅ | `blacklist:token:{jti}` String, TTL=JWT剩余有效期 |
| 任务 | ✅ | 无异步需求 |
| 验收 | ✅ | 3 条场景 |
| 配置 | ✅ | `default_password_length: 8` + `force_password_change: true` |

---

### T-06：角色体系

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | 角色分配嵌入 T-05 API；角色列表可前端硬编码（4 个预设） |
| DDL | ✅ | `tenant_users.role` 含 CHECK 约束 |
| Redis | ✅ | 角色/权限注入 JWT（登录时查一次），使用期间校验 JWT |
| 任务 | ✅ | — |
| 验收 | ⚠️ | 含在 T-05/T-08 场景中 |
| 配置 | ✅ | — |

---

### T-07：菜单权限控制

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | `GET /api/v1/menus` 完整定义（含 Response JSON） |
| DDL | ✅ | — |
| Redis | ✅ | `menu:{tid}:{uid}` String, TTL=5min |
| 任务 | ✅ | — |
| 验收 | ⚠️ | 待补 |
| 配置 | ✅ | — |

---

### T-08：注册流程变更

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | login Response 含 role/permissions；register 含 phone+Errors |
| DDL | ✅ | `users.phone` 迁移已定义 |
| Redis | ✅ | `ratelimit:register:{ip}` + `ratelimit:login:{ip}` |
| 任务 | ✅ | — |
| 验收 | ✅ | 3 条场景 |
| 配置 | ✅ | 4 个配置项 |

---

### T-09：删除 Organization

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | 纯删除 |
| DDL | ✅ | 6 表 + 迁移脚本号已定 |
| Redis | ✅ | — |
| 任务 | ✅ | — |
| 验收 | ✅ | 删除 Organization 后共享逻辑不存在，share_count 一并移除 |
| 配置 | ✅ | — |

---

### T-10：Token 配额统计

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ✅ | Token 用量含在 `GET /api/v1/admin/tenants/{id}/stats` (T-11) |
| DDL | ✅ | `tenant_stats` 表已覆盖 |
| Redis | ✅ | `tenant:{tid}:token_used` String（原子 INCRBY）+ 配额检查流程已定义 |
| 任务 | ✅ | `reset_token_quota` + `sync_token_redis_to_db` |
| 验收 | ✅ | 3 条场景 |
| 配置 | ⚠️ | Cron 表达式可配置 → 卡片阶段细化 |

---

### T-11：系统管理员后台

| 维度 | 状态 | 缺口 |
|------|:----:|------|
| API | ⚠️ | 13 个端点路径已定，全部缺 Request/Response。最关键：`GET /api/v1/admin/tenants` 的分页参数？`POST /api/v1/admin/tenants` 创建租户时同时创建 `tenant_stats` 行 |
| DDL | ✅ | — |
| Redis | ❌ | — |
| 任务 | ✅ | — |
| 验收 | ❌ | — |
| 配置 | ✅ | — |

---

## 二、交叉关注点

### 2.1 Auth 中间件重构（影响所有模块）

现有中间件 `internal/middleware/auth.go` 仅注入 `tenant_id`。需改造为：

```
JWT Payload 新增字段:
  - user_id: string
  - tenant_id: uint64
  - role: string
  - permissions: []string

Context 注入:
  - ctx.TenantID
  - ctx.UserID        ← 新增
  - ctx.Role          ← 新增
  - ctx.Permissions   ← 新增
```

**影响**：所有现有 Handler 从 `ctx.TenantID` 读租户的方式不变，但需逐一确认 Session 相关 Handler 同时读 `ctx.UserID`。

### 2.2 权限执行点（未定义）

权限矩阵（T-06）已定义，但**执行位置**不明确：

| 执行点 | 候选方案 | 推荐 |
|--------|----------|:---:|
| Gin 中间件 | 路由级 `role:tenant_admin` | ✅ 菜单/路由守卫 |
| Handler 入口 | 函数开头检查 `ctx.Role` | ✅ 粗粒度 |
| Service 层 | 数据级 `created_by == ctx.UserID` | ✅ editor 自己的资源 |

**决策待定**：是否存在统一的权限检查函数 `RequirePermission(ctx, "kb:create")`？

### 2.3 错误码体系（未统一）

现有需求中出现：

| 错误码 | 触发条件 | 模块 |
|--------|----------|:---:|
| `TOKEN_QUOTA_EXCEEDED` | Token 超额 | T-10 |
| `PLAN_DISABLED` | 套餐停用 | T-03 |
| `TRIAL_EXPIRED` | 体验到期 | T-04 |

**缺失**（需补全）：

| 错误码 | 触发条件 |
|--------|----------|
| `PLAN_LIMIT_REACHED` | 知识库/用户/存储达上限 |
| `USER_EXISTS` | 同名用户已存在 |
| `INVALID_ROLE` | 分配了不存在的角色 |
| `LAST_ADMIN` | 不能移除最后一个 tenant_admin |
| `PERMISSION_DENIED` | 无权限操作 |
| `TENANT_DISABLED` | 租户被禁用 |
| `USER_DISABLED` | 用户被禁用 |
| `USER_NOT_IN_TENANT` | 用户不属于该租户 |

### 2.4 Session 表变更（未标注）

T-01 要求 "Session 查询增加 `user_id` 过滤"，但 `sessions` 表当前无 `user_id` 列。需要：

```sql
ALTER TABLE sessions ADD COLUMN user_id VARCHAR(36);
ALTER TABLE sessions ADD COLUMN created_by VARCHAR(36);  -- 与 user_id 同源
CREATE INDEX idx_sessions_tenant_user ON sessions(tenant_id, user_id);
```

### 2.5 Knowledge/Agent 归属标记

T-06 权限矩阵中 editor 可"编辑/删除自己创建的"，需要 `knowledges` 和 `agents` 表增加 `created_by` 字段。当前代码中未见此字段。

```sql
ALTER TABLE knowledges ADD COLUMN created_by VARCHAR(36);
ALTER TABLE agents ADD COLUMN created_by VARCHAR(36);
```

### 2.6 迁移顺序依赖

```
迁移执行顺序（严格）:
  1. 删除 Organization 表（T-09）
  2. 创建 plans 表（T-03）
  3. 创建 tenant_users 表（T-01）
  4. 修改 users 表（废弃 tenant_id）
  5. 修改 sessions 表（增加 user_id）
  6. 修改 knowledges/agents 表（增加 created_by）
  7. 创建 tenant_stats 表（统计表）
  8. 初始迁移数据到 tenant_stats
  9. 修改 tenants 表（增加 plan_id/is_system，移除 storage_quota/storage_used）
  10. 种子数据：系统租户 + 体验租户 + 默认 admin + plans
```

---

## 三、覆盖率评分

| 模块 | API | DDL | Redis | 任务 | 验收 | 配置 | 均分 |
|------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| T-01 Tenant 语义 | 60% | 30% | 0% | 100% | 0% | 100% | 48% |
| T-02 系统租户 | 100% | 70% | 0% | 50% | 0% | 50% | 45% |
| T-03 套餐体系 | 50% | 90% | 0% | 60% | 0% | 100% | 50% |
| T-04 体验租户 | 100% | 70% | 0% | 40% | 0% | 100% | 52% |
| T-05 用户管理 | 30% | 20% | 0% | 100% | 0% | 60% | 35% |
| T-06 角色体系 | 50% | 70% | 0% | 100% | 0% | 100% | 53% |
| T-07 菜单控制 | 10% | 100% | 0% | 100% | 0% | 100% | 52% |
| T-08 注册流程 | 60% | 70% | 0% | 100% | 0% | 100% | 55% |
| T-09 删除Org | 100% | 90% | 100% | 100% | 30% | 100% | 87% |
| T-10 Token配额 | 50% | 90% | 0% | 60% | 0% | 70% | 45% |
| T-11 管理员后台 | 40% | 90% | 0% | 100% | 0% | 100% | 55% |
| **综合** | **45%** | **70%** | **0%** | **40%** | **0%** | **20%** | **29%** |

---

## 四、阻塞项（进入阶段 3 前必须解决）

### ✅ 已解决（v1.4）

| # | 阻塞项 | 解决方案 |
|---|--------|----------|
| 1 | `tenant_users` 完整 DDL | T-01 新增完整 CREATE TABLE + PK/FK/UNIQUE/CHECK |
| 2 | `sessions.user_id` 列 | T-01 新增 ALTER TABLE + INDEX |
| 3 | `GET /api/v1/menus` 端点 | T-07 新增完整 Request/Response |
| 4 | `POST /api/v1/auth/login` JWT 扩展 | T-08 新增 JWT Payload + login Response |
| 5 | Token 配额 Redis 计数器 | 十六新增 `tenant:{tid}:token_used` + 配额检查流程 |
| 6 | 完整错误码清单 | 十七新增 22 个错误码 |
| 7 | 权限执行点 | 评审已推荐三层执行（见 2.2），待确认 |
| 8-13 | P1 各项 | 全部补全（Redis/验收/created_by/黑名单/Schema） |

### 🔶 剩余待补（卡片阶段细化）

- T-02/T-06/T-07 验收场景
- Admin 列表接口分页参数约定
- Cron 表达式可配置化

---

## 五、待决策事项（请确认）

以下 3 项我在补全时有多个选择，需你做最终决策：

### 1. 默认 Admin 密码来源

- ~~方案 A~~ ~~方案 B~~ → **已确认：固定 `admin_123456`**，首次登录强制修改

### 2. 套餐停用后正在进行的对话

- ~~方案 A~~ ~~方案 B~~ → **已确认：方案 B** — 当前对话继续，下次对话时阻断

### 3. 权限执行点：推荐"三层分工"，是否认可？

→ **已确认**，详细解释见下方。

> 已从 `specs/ARCHITECTURE_REVIEW.md` 移至此处。
