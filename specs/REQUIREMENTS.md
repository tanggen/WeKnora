# WeKnora SaaS 化改造需求文档

> 版本：v1.4 | 状态：已确认 ✅（评审补全 + 决策确认）

---

## 一、功能清单

| 编号 | 功能 | 优先级 | 说明 |
|------|------|:------:|------|
| **T-01** | Tenant 语义变更：个人→企业 | P0 | tenant_id 对应企业；一个 Tenant 下有多个 User；一个 User 只属于一个 Tenant |
| **T-02** | 系统默认租户 | P0 | 预置系统管理员租户（id=1），与普通租户逻辑一致，可承载业务数据 |
| **T-03** | 套餐体系 | P0 | 预置体验版/基础版/专业版三级套餐；可启用/停用；套餐可动态扩展配置项 |
| **T-04** | 体验租户 | P0 | 预置体验租户（id=2），自行注册用户归入体验租户，角色为 viewer |
| **T-05** | 租户用户管理 | P0 | 租户管理员可新增/编辑/禁用/逻辑删除租户内用户 |
| **T-06** | 角色体系 | P0 | 预置四角色：system_admin / tenant_admin / editor / viewer |
| **T-07** | 菜单权限控制 | P1 | 套餐决定租户菜单；角色决定用户菜单；无权限菜单隐藏 |
| **T-08** | 注册流程变更 | P0 | 自行注册 → 归入体验租户（viewer）；管理员创建用户；套餐升级迁移 |
| **T-09** | 删除 Organization | P0 | 移除现有"共享空间"全部代码 |
| **T-10** | Token 配额统计 | P1 | 租户级配额，超额阻断；属于套餐配置项之一；暂不计入 Embedding/Rerank |
| **T-11** | 系统管理员后台 | P1 | 租户管理、套餐管理、用户迁移等 |

---

## 二、T-01：Tenant 语义变更

### 变更内容

```
变更前：User ─(1:1)─> Tenant ─(1:N)─> KB/Session/Model/Agent/...
变更后：User ─(N:1)─> Tenant ─(1:N)─> KB/Session/Model/Agent/...
               ↑
         tenant_users（关联表）
         ├── tenant_id (PK)
         ├── user_id   (PK, UNIQUE — 一个用户只能属于一个租户)
         ├── role
         └── permissions (JSONB)
```

### `tenant_users` DDL

```sql
CREATE TABLE tenant_users (
    tenant_id    BIGINT NOT NULL,
    user_id      VARCHAR(36) NOT NULL,
    role         VARCHAR(32) NOT NULL DEFAULT 'viewer'
                 CHECK (role IN ('system_admin', 'tenant_admin', 'editor', 'viewer')),
    permissions  JSONB DEFAULT '[]',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id),
    CONSTRAINT fk_tenant_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT fk_tenant_users_user   FOREIGN KEY (user_id)   REFERENCES users(id)   ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_tenant_users_user_id ON tenant_users(user_id);
```

### `users` 表变更

```sql
-- 废弃原有 tenant_id（保留兼容，数据迁移后移除）
-- 新增手机号字段
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
CREATE UNIQUE INDEX idx_users_phone ON users(phone) WHERE phone IS NOT NULL AND phone != '';

-- user_id 使用 UUID（VARCHAR(36)），与现有 users.id 类型一致
```

### `sessions` 表变更（个人会话隔离）

```sql
ALTER TABLE sessions ADD COLUMN user_id VARCHAR(36) NOT NULL DEFAULT '';
CREATE INDEX idx_sessions_tenant_user ON sessions(tenant_id, user_id);

-- 迁移时：现有会话按 tenant_id 关联到该租户唯一用户的 user_id
-- 若租户有多个用户，旧会话统一归属到 tenant_admin
```

### `knowledges` / `agents` 表变更（编辑器归属）

```sql
-- 文档归属（用于 editor 只能操作自己创建的资源）
ALTER TABLE knowledges ADD COLUMN created_by VARCHAR(36);
CREATE INDEX idx_knowledges_created_by ON knowledges(tenant_id, created_by);

ALTER TABLE agents ADD COLUMN created_by VARCHAR(36);
CREATE INDEX idx_agents_created_by ON agents(tenant_id, created_by);

-- 注意：tenant_admin 和 system_admin 不受 created_by 限制，可操作租户内任意资源
```

### 数据可见性（已确认 ✅）

| 数据类型 | 可见范围 | 说明 |
|----------|----------|------|
| 知识库（KB） | 租户内全部可见 | 所有成员可查看、搜索所有知识库 |
| Agent（智能体） | 租户内全部可见 | 所有成员可查看、使用所有 Agent |
| Session（会话） | 仅个人可见 | 每个用户只能看到自己的对话记录 |

### 核心规则

1. 所有业务数据表（KB、Chunk、Model、Agent 等）的 `tenant_id` 字段含义不变，但值由"个人租户ID"变为"企业租户ID"
2. 同一 Tenant 下的用户**默认共享**知识库和 Agent（Role 控制操作权限）
3. Session 表查询需增加 `user_id` 过滤条件，实现个人会话隔离
4. User 表原有的 `tenant_id` 字段废弃（保留兼容，逐步移除），用户归属完全由 `tenant_users` 管理
5. **一个用户只能属于一个租户**（`tenant_users.user_id` 唯一约束）
6. `knowledges.created_by` / `agents.created_by` 记录创建者，用于 editor 的"仅操作自己资源"逻辑

---

## 三、T-02：系统默认租户

> 决策：系统租户与普通租户逻辑完全一致，可以承载业务数据。通过 `is_system` 字段标识。
> 系统管理员账号通过数据库种子脚本预置。

### 种子数据

```sql
-- 系统租户
INSERT INTO tenants (id, name, description, status, is_system) 
VALUES (1, '系统管理', '系统默认管理租户', 'active', true);

-- 系统管理员用户（种子脚本预置，默认密码 admin_123456，首次登录强制修改）
INSERT INTO users (id, username, email, password_hash, is_active) 
VALUES ('sys-admin-001', 'admin', 'admin@system.local', '<bcrypt_of_admin_123456>', true);

-- 关联
INSERT INTO tenant_users (tenant_id, user_id, role) 
VALUES (1, 'sys-admin-001', 'system_admin');
```

### 规则

1. 系统租户 id=1，不可删除、不可禁用
2. 系统租户的管理员（`system_admin` 角色）即**系统最高管理员**，拥有操作所有租户的权限
3. 系统租户**可以承载业务数据**，逻辑上与普通租户无区别
4. 系统管理员账号密码在首次启动后应强制修改

---

## 四、T-03：套餐体系

> 决策：套餐是一等实体，系统管理员可管理（启用/停用）。套餐控制多维度租户能力上限。

### 套餐数据模型

```sql
CREATE TABLE plans (
    id          VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(64) NOT NULL,              -- 体验版 / 基础版 / 专业版
    code        VARCHAR(32) NOT NULL UNIQUE,       -- trial / basic / pro
    is_enabled  BOOLEAN DEFAULT TRUE,              -- 是否启用（停用后该套餐下的租户不可使用大模型）
    sort_order  INT DEFAULT 0,                     -- 排序
    config      JSONB NOT NULL DEFAULT '{}',       -- 套餐配置项
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 套餐配置项（`config` JSONB）

```json
{
  "visible_menus": ["knowledge-bases", "knowledge-search", "agents", "chat", "settings"],
  "max_users": 10,           // 最大用户数（0=无限）
  "max_knowledge_bases": 5,  // 最大知识库数（0=无限）
  "max_storage_bytes": 10737418240,  // 最大存储空间（字节，默认10GB）
  "max_tokens_per_month": 1000000,   // 月最大token数（0=无限）
  "trial_days": 3,           // 试用天数（仅 trial 套餐，0=非试用）
  "allow_model_config": false,       // 是否允许配置模型
  "allow_api_key_access": false      // 是否允许API Key访问
}
```

### 种子数据：三级套餐

| 套餐 | code | 默认配置 |
|------|------|----------|
| 体验版 | `trial` | 菜单4项，1用户，1知识库，1GB存储，10万token/月，3天试用 |
| 基础版 | `basic` | 菜单5项，10用户，10知识库，10GB存储，100万token/月 |
| 专业版 | `pro` | 全部菜单，无限用户/知识库/存储/token |

### 套餐管理规则（已确认 ✅）

1. **启用/停用**：系统管理员可停用某个套餐；停用后，所有该套餐下的租户调用大模型时返回 `PLAN_DISABLED` 错误
2. **升级逻辑**：租户从套餐A升级到套餐B时：
   - 各项新上限按套餐B的配置计算
   - **已使用的配额从新配额中扣除**（如token_used=80万，升级到200万套餐，剩余可用=120万）
   - 超出新上限的项目仍保留但不可新增（如已有15个知识库，升级到上限10个的套餐，不可再创建）
3. **套餐可配置项**（可扩展）：
   - 菜单可见范围
   - 最大用户数
   - 最大知识库数
   - 最大存储空间
   - 最大 token 数
   - （未来可扩展：最大Agent数、是否支持API Key等）

### 配额校验规则

所有套餐上限检查**不实时聚合业务表**，统一从 `tenant_stats` 读取当前用量：

| 操作 | 校验项 | 读取字段 | 对比上限 |
|------|--------|----------|----------|
| 创建知识库 | KB 数量 | `tenant_stats.knowledge_base_count` | `plans.config.max_knowledge_bases` |
| 新增用户 | 用户数 | `tenant_stats.user_count` | `plans.config.max_users` |
| 上传文件 | 存储量 | `tenant_stats.storage_used_bytes` | `plans.config.max_storage_bytes` |
| Chat 调用 | Token 用量 | `tenant_stats.token_used` | `plans.config.max_tokens_per_month` |

### 租户-套餐关联

```sql
ALTER TABLE tenants ADD COLUMN plan_id VARCHAR(36) REFERENCES plans(id);
ALTER TABLE tenants ADD COLUMN plan_code VARCHAR(32) DEFAULT 'basic';
```

---

## 五、租户统计表（`tenant_stats`）

> 设计原则：**统计数据集中存储，避免实时跨表聚合。**
> 所有计数器在对应业务操作时通过 `INSERT ... ON CONFLICT UPDATE` 或悲观锁原子更新。
> 配额检查一律读取此表，不查业务表。

### DDL

```sql
CREATE TABLE tenant_stats (
    tenant_id             BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- 用户统计
    user_count            INT NOT NULL DEFAULT 0,       -- 租户内有效用户数（is_active=true）
    
    -- 知识库统计
    knowledge_base_count  INT NOT NULL DEFAULT 0,       -- 知识库数（不含已删除）
    knowledge_count       INT NOT NULL DEFAULT 0,       -- 知识/文档总数
    chunk_count           BIGINT NOT NULL DEFAULT 0,    -- 分块总数
    
    -- 存储统计
    storage_used_bytes    BIGINT NOT NULL DEFAULT 0,    -- 已用存储（字节）
    
    -- Token统计
    token_used            BIGINT NOT NULL DEFAULT 0,    -- 当月已用 token（仅 Chat）
    token_quota_reset_at  TIMESTAMP,                    -- 下次重置时间
    
    -- Agent统计
    agent_count           INT NOT NULL DEFAULT 0,       -- Agent数
    
    -- 会话统计
    session_count         BIGINT NOT NULL DEFAULT 0,    -- 会话总数
    
    updated_at            TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 计数器详细计算规则

#### 1. `user_count` — 有效用户数

```sql
-- 增量更新（原子）
UPDATE tenant_stats SET user_count = user_count + 1, updated_at = NOW()
WHERE tenant_id = $tid;

-- 减量更新
UPDATE tenant_stats SET user_count = GREATEST(user_count - 1, 0), updated_at = NOW()
WHERE tenant_id = $tid;
```

| 触发时机 | 条件 | delta |
|----------|------|:---:|
| 新增用户 + 加入租户 | `tenant_users` INSERT + `users.is_active=true` | +1 |
| 用户被启用 | `users.is_active` 改为 true | +1 |
| 用户被禁用 | `users.is_active` 改为 false | -1 |
| 用户逻辑删除 | `users.is_active` 改为 false | -1 |
| 用户迁移到其他租户 | 源租户 -1，目标租户 +1 | — |

> 计数的用户条件：`tenant_users.tenant_id = $tid AND users.is_active = true AND users.deleted_at IS NULL`

#### 2. `knowledge_base_count` — 知识库数

```sql
UPDATE tenant_stats SET knowledge_base_count = GREATEST(knowledge_base_count + $delta, 0)
WHERE tenant_id = $tid;
```

| 触发时机 | delta |
|----------|:---:|
| KB 创建成功 | +1 |
| KB 删除（软删除 `deleted_at`） | -1 |

> 计数条件：`knowledge_bases.tenant_id = $tid AND is_temporary = false AND deleted_at IS NULL`
> 临时知识库（`is_temporary=true`）不计入配额。

#### 3. `knowledge_count` — 文档总数

| 触发时机 | delta |
|----------|:---:|
| 文档上传且解析成功（`parse_status = 'completed'`） | +1 |
| 文档删除 | -1 |

> 计数条件：`knowledges.tenant_id = $tid AND deleted_at IS NULL`
> 注意：解析中的文档（`parse_status='processing'`）不计入，只有 completed 才 +1。

#### 4. `chunk_count` — 分块总数

| 触发时机 | delta |
|----------|:---:|
| 文档解析完成，chunk 批量写入后 | +N（实际新增 chunk 数） |
| chunk 被删除（文档级或单条） | -N（实际删除 chunk 数） |

> 计数条件：`chunks.tenant_id = $tid AND is_enabled = true AND deleted_at IS NULL`
> 原子更新：`UPDATE ... SET chunk_count = GREATEST(chunk_count + $delta, 0)`

#### 5. `storage_used_bytes` — 已用存储

```sql
-- 原子增减（含下界保护）
UPDATE tenant_stats 
SET storage_used_bytes = GREATEST(storage_used_bytes + $delta, 0), updated_at = NOW()
WHERE tenant_id = $tid;
```

| 触发时机 | delta |
|----------|:---:|
| 文件上传成功 | +`knowledges.storage_size`（含原始文件 + 向量存储） |
| 文件/分块删除 | -释放的字节数 |
| 租户迁移用户 | 不调整（数据保留在原租户） |

> 计费范围：原始文件大小 + 向量存储占用 + 文本索引大小。
> 该值从现有 `tenants.storage_used` 初始迁移获得，后续由业务操作维护。

#### 6. `token_used` — 当月已用 Token

```sql
-- Chat 返回后累加
UPDATE tenant_stats 
SET token_used = token_used + $total_tokens, updated_at = NOW()
WHERE tenant_id = $tid;
```

| 触发时机 | delta |
|----------|:---:|
| Chat / Agent 对话返回 | +`response.Usage.TotalTokens`（含 input + output） |
| 月初定时任务 | SET 0，同时更新 `token_quota_reset_at` |

> 仅统计 LLM Chat 调用。Embedding / Rerank / VLM / ASR 暂不计入。
> 配额检查顺序：读 `tenant_stats.token_used` → 对比 `plans.config.max_tokens_per_month` → 超额阻断。

#### 7. `agent_count` — Agent 数

| 触发时机 | delta |
|----------|:---:|
| Agent 创建 | +1 |
| Agent 删除（软删除） | -1 |

> 计数条件：`agents.tenant_id = $tid AND deleted_at IS NULL`

#### 8. `session_count` — 会话数

| 触发时机 | delta |
|----------|:---:|
| 会话创建 | +1 |
| 会话删除（软删除） | -1 |

> 计数条件：`sessions.tenant_id = $tid AND deleted_at IS NULL`
> 此计数器仅用于统计展示，不作为配额限制项。

---

### 一致性保障机制

#### 原子更新策略

| 场景 | 策略 | 示例 |
|------|------|------|
| 单字段 +1/-1 | 直接 `UPDATE SET col = col + $delta` | `token_used += total_tokens` |
| 多字段联动 | `SELECT ... FOR UPDATE` 锁行后批量更新 | 文档删除时：kb_count -1, knowledge_count -1, chunk_count -N, storage -M |
| 负值防护 | 所有减法使用 `GREATEST(col + $delta, 0)` | 防止并发导致负数 |

```sql
-- 示例：文档删除时的多字段原子更新
BEGIN;
SELECT * FROM tenant_stats WHERE tenant_id = $tid FOR UPDATE;
UPDATE tenant_stats SET
    knowledge_count = GREATEST(knowledge_count - $doc_count, 0),
    chunk_count     = GREATEST(chunk_count - $chunk_count, 0),
    storage_used_bytes = GREATEST(storage_used_bytes - $storage_size, 0),
    updated_at = NOW()
WHERE tenant_id = $tid;
COMMIT;
```

#### 定期对账（每天凌晨 3:00）

对账目的：修复因异常（程序崩溃、并发竞争、手动改库）导致的计数器偏差。

```sql
-- ============================================
-- 对账函数：reconcile_tenant_stats(tenant_id)
-- ============================================

-- 1. user_count
SELECT COUNT(*) INTO calc_user_count
FROM tenant_users tu
JOIN users u ON tu.user_id = u.id
WHERE tu.tenant_id = $tid AND u.is_active = true AND u.deleted_at IS NULL;

-- 2. knowledge_base_count
SELECT COUNT(*) INTO calc_kb_count
FROM knowledge_bases
WHERE tenant_id = $tid AND is_temporary = false AND deleted_at IS NULL;

-- 3. knowledge_count
SELECT COUNT(*) INTO calc_knowledge_count
FROM knowledges
WHERE tenant_id = $tid AND deleted_at IS NULL;

-- 4. chunk_count
SELECT COUNT(*) INTO calc_chunk_count
FROM chunks
WHERE tenant_id = $tid AND is_enabled = true AND deleted_at IS NULL;

-- 5. storage_used_bytes
SELECT COALESCE(SUM(storage_size), 0) INTO calc_storage_used
FROM knowledges
WHERE tenant_id = $tid AND deleted_at IS NULL;

-- 6. agent_count
SELECT COUNT(*) INTO calc_agent_count
FROM agents
WHERE tenant_id = $tid AND deleted_at IS NULL;

-- 7. session_count
SELECT COUNT(*) INTO calc_session_count
FROM sessions
WHERE tenant_id = $tid AND deleted_at IS NULL;

-- 对比并修复
table stats = SELECT * FROM tenant_stats WHERE tenant_id = $tid;
IF calc_user_count       != stats.user_count            THEN UPDATE tenant_stats SET user_count            = calc_user_count       WHERE tenant_id = $tid; LOG WARN 'reconciled user_count: %d→%d', stats.user_count, calc_user_count;
IF calc_kb_count          != stats.knowledge_base_count  THEN UPDATE tenant_stats SET knowledge_base_count  = calc_kb_count         WHERE tenant_id = $tid;
IF calc_knowledge_count   != stats.knowledge_count       THEN UPDATE tenant_stats SET knowledge_count       = calc_knowledge_count  WHERE tenant_id = $tid;
IF calc_chunk_count       != stats.chunk_count           THEN UPDATE tenant_stats SET chunk_count           = calc_chunk_count      WHERE tenant_id = $tid;
IF calc_storage_used      != stats.storage_used_bytes    THEN UPDATE tenant_stats SET storage_used_bytes    = calc_storage_used     WHERE tenant_id = $tid;
IF calc_agent_count       != stats.agent_count           THEN UPDATE tenant_stats SET agent_count           = calc_agent_count      WHERE tenant_id = $tid;
IF calc_session_count     != stats.session_count         THEN UPDATE tenant_stats SET session_count         = calc_session_count    WHERE tenant_id = $tid;

-- token_used 不参与对账修复（无可靠对账数据源，仅靠月初重置保证上限）
```

#### 对账策略

| 决策项 | 结论 |
|--------|------|
| 修复方向 | 以业务表 COUNT 为准，直接覆盖 `tenant_stats` |
| 修复后 | 记录 WARN 日志（含租户ID、字段名、旧值→新值） |
| 回滚保护 | 修复仅在业务低峰（凌晨）执行，单租户粒度，异常不阻塞后续租户 |
| `token_used` 特殊处理 | **不参与对账**（无可靠对账数据源），仅依赖月初定时任务重置。偏差容忍策略：月底未重置时可能少计，但不会多计 |
| 新增租户 | 创建租户时同步 `INSERT INTO tenant_stats (tenant_id) VALUES ($tid)`（全部默认 0） |

---

### 初始迁移

```sql
-- 迁移脚本：从现有业务表聚合初始值写入 tenant_stats
INSERT INTO tenant_stats (tenant_id, user_count, knowledge_base_count, knowledge_count, 
                          chunk_count, storage_used_bytes, agent_count, session_count, updated_at)
SELECT
    t.id AS tenant_id,
    COALESCE(u.cnt, 0) AS user_count,
    COALESCE(kb.cnt, 0) AS knowledge_base_count,
    COALESCE(k.cnt, 0) AS knowledge_count,
    COALESCE(ch.cnt, 0) AS chunk_count,
    t.storage_used AS storage_used_bytes,  -- 从 tenants 表迁移
    COALESCE(a.cnt, 0) AS agent_count,
    COALESCE(s.cnt, 0) AS session_count,
    NOW() AS updated_at
FROM tenants t
LEFT JOIN (SELECT tenant_id, COUNT(*) AS cnt FROM tenant_users tu 
           JOIN users u ON tu.user_id = u.id 
           WHERE u.is_active = true AND u.deleted_at IS NULL GROUP BY tenant_id) u ON t.id = u.tenant_id
LEFT JOIN (SELECT tenant_id, COUNT(*) AS cnt FROM knowledge_bases 
           WHERE is_temporary = false AND deleted_at IS NULL GROUP BY tenant_id) kb ON t.id = kb.tenant_id
LEFT JOIN (SELECT tenant_id, COUNT(*) AS cnt FROM knowledges 
           WHERE deleted_at IS NULL GROUP BY tenant_id) k ON t.id = k.tenant_id
LEFT JOIN (SELECT tenant_id, COUNT(*) AS cnt FROM chunks 
           WHERE is_enabled = true AND deleted_at IS NULL GROUP BY tenant_id) ch ON t.id = ch.tenant_id
LEFT JOIN (SELECT tenant_id, COUNT(*) AS cnt FROM agents 
           WHERE deleted_at IS NULL GROUP BY tenant_id) a ON t.id = a.tenant_id
LEFT JOIN (SELECT tenant_id, COUNT(*) AS cnt FROM sessions 
           WHERE deleted_at IS NULL GROUP BY tenant_id) s ON t.id = s.tenant_id
ON CONFLICT (tenant_id) DO NOTHING;
```

### 与 tenants 表的字段迁移

```
 tenants 表变更：
   - storage_used  →  迁移到 tenant_stats.storage_used_bytes（移除）
   - storage_quota →  改为从 plans.config.max_storage_bytes 读取（移除）
   + plan_id       →  新增（FK → plans.id）
   + plan_code     →  新增（冗余，方便查询）
   + is_system     →  新增（BOOLEAN，标识系统租户）
```

---

## 六、T-04：体验租户

> 决策：预置体验租户（id=2），自行注册用户**自动归入**，角色为 **viewer**（仅查看使用，不能修改/删除）。

### 种子数据

```sql
INSERT INTO tenants (id, name, description, status, plan_code) 
VALUES (2, '体验空间', '三日免费体验', 'active', 'trial');
```

### 规则

1. 体验租户 id=2，不可删除
2. 自行注册用户自动加入体验租户，角色为 **`viewer`**（仅查看和搜索，不可创建/编辑/删除任何资源）
3. 注册满 3 天（`plan.trial_days`）后，调用大模型返回 `TRIAL_EXPIRED` 错误
4. 系统管理员可将体验用户迁移到正式租户并升级套餐（T-10）
5. 体验租户的套餐为体验版（`trial`），套餐配置继承自 `plans` 表

---

## 七、T-05：租户用户管理

### API 轮廓（待细化）

| 操作 | 方法 | 路径 | 权限 |
|------|------|------|------|
| 列出用户 | GET | `/api/v1/tenants/{id}/users` | tenant_admin |
| 新增用户 | POST | `/api/v1/tenants/{id}/users` | tenant_admin |
| 编辑用户 | PUT | `/api/v1/tenants/{id}/users/{user_id}` | tenant_admin |
| 禁用/启用 | PATCH | `/api/v1/tenants/{id}/users/{user_id}/status` | tenant_admin |
| 删除用户 | DELETE | `/api/v1/tenants/{id}/users/{user_id}` | tenant_admin |
| 设置角色 | PUT | `/api/v1/tenants/{id}/users/{user_id}/role` | tenant_admin |

### 业务规则

1. 租户至少保留 1 个 `tenant_admin`，不可全部移除
2. 禁用用户后该用户的 Token 立即失效
3. **删除用户为逻辑删除**（`is_active=false`），不物理删除数据
4. 用户创建的 Agent/知识库数据**保留在租户内**（数据归属租户）
5. 用户创建的会话数据保留但不可访问（归属个人）
6. 新增/禁用用户时同步更新 `tenant_stats.user_count`

---

## 八、T-06：角色体系

> 决策：预置四角色，不需要自定义角色。tenant_admin 集中管控，可操作任何人的资源。
> 个人用户可创建/编辑/删除自己创建的知识（需要权限：`kb:create` / `kb:edit:own` / `kb:delete:own`）。

### 角色定义

| 角色 | 级别 | 说明 |
|------|:----:|------|
| `system_admin` | 系统 | 管理所有租户、所有用户、全局配置 |
| `tenant_admin` | 租户 | 集中管控：管理租户信息、用户、**所有人的**知识库/Agent/模型 |
| `editor` | 租户 | 创建/编辑/删除**自己**的知识库和 Agent，查看租户内全部资源 |
| `viewer` | 租户 | 仅查看和搜索 |

### 权限矩阵（已确认 ✅）

| 权限 | system_admin | tenant_admin | editor | viewer |
|------|:---:|:---:|:---:|:---:|
| 管理所有租户（CRUD + 设套餐） | ✅ | ❌ | ❌ | ❌ |
| 管理租户信息 | ✅ | ✅ | ❌ | ❌ |
| 管理租户内用户 | ✅ | ✅ | ❌ | ❌ |
| 管理模型配置 | ✅ | ✅ | ❌ | ❌ |
| 创建知识库 | ✅ | ✅ | ✅ | ❌ |
| 编辑/删除**任意**知识库 | ✅ | ✅ | ❌ | ❌ |
| 编辑/删除**自己**的知识库 | — | — | ✅ | ❌ |
| 创建 Agent | ✅ | ✅ | ✅ | ❌ |
| 编辑/删除**任意** Agent | ✅ | ✅ | ❌ | ❌ |
| 编辑/删除**自己**的 Agent | — | — | ✅ | ❌ |
| 对话/搜索 | ✅ | ✅ | ✅ | ✅ |
| 查看 Token 用量 | ✅ | ✅ | ❌ | ❌ |

---

## 九、T-07：菜单权限控制

> 决策：租户级套餐 + 角色默认菜单。无权限菜单**隐藏**。

### 两级控制模型

```
第一级：租户套餐（T-03）→ 决定该租户最多可见哪些菜单（从 plans.config.visible_menus 读取）
        体验版: [知识库, 知识搜索, 智能体, 对话]
        基础版: [知识库, 知识搜索, 智能体, 对话, 设置]
        专业版: [知识库, 知识搜索, 智能体, 对话, 设置, 用户管理]

第二级：用户角色（T-06）→ 在套餐范围内，按角色决定可见菜单
        tenant_admin: 全部套餐菜单
        editor:       [知识库, 知识搜索, 智能体, 对话, 设置]
        viewer:       [知识库, 知识搜索, 智能体, 对话]

最终可见 = min(套餐菜单, 角色菜单)
```

### 菜单项与权限映射

| 菜单 | 路径 | 权限标识 | 体验版 | 基础版 | 专业版 |
|------|------|----------|:---:|:---:|:---:|
| 知识库 | `knowledge-bases` | `kb:view` | ✅ | ✅ | ✅ |
| 知识搜索 | `knowledge-search` | `kb:view` | ✅ | ✅ | ✅ |
| 智能体 | `agents` | `agent:view` | ✅ | ✅ | ✅ |
| 对话 | `creatChat` | `chat:use` | ✅ | ✅ | ✅ |
| 设置 | `settings` | `settings:view` | ❌ | ✅ | ✅ |
| 用户管理 | `tenant-users` | `user:view` | ❌ | ❌ | ✅ |

### 菜单 API

```
GET /api/v1/menus
  描述：返回当前用户可见的菜单树（已合并套餐 + 角色两级过滤）
  Auth：Bearer Token（中间件已注入 tenant_id + role）
  
  Response 200:
  {
    "menus": [
      {
        "id": "knowledge-bases",
        "path": "/knowledge-bases",
        "name": "知识库",
        "icon": "folder",
        "permission": "kb:view",
        "children": []
      },
      {
        "id": "agents",
        "path": "/agents", 
        "name": "智能体",
        "icon": "robot",
        "permission": "agent:view",
        "children": []
      }
    ]
  }
```

> 后端实现逻辑：读 `plans.config.visible_menus`（套餐级）∩ 角色默认菜单 → 返回交集。
> 前端在 App.vue 挂载时调用此接口，动态渲染侧边栏。

### 前端行为

- 菜单从 `GET /api/v1/menus` 获取（前端不再硬编码菜单数组）
- 无权限菜单**直接隐藏**，不在侧边栏渲染
- 路由守卫：无权限路由返回 403

---

## 十、T-08：注册流程变更

> 决策：允许自行注册。自行注册用户**不创建新租户**，而是自动归入预置的体验租户（id=2）。
> 系统管理员可在后台创建正式租户并迁移用户。

### 认证变更：JWT Payload 扩展

```
变更前 JWT Payload:
  { "user_id": "xxx", "tenant_id": 10000, "exp": ... }

变更后 JWT Payload:
  {
    "user_id": "uuid",
    "tenant_id": 10000,
    "role": "tenant_admin",
    "permissions": ["kb:create", "kb:edit", "agent:view", "chat:use", ...],
    "exp": 1700000000
  }
```

> `role` 和 `permissions` 在登录时从 `tenant_users` 查询后注入 JWT。
> 中间件从 JWT 解析后注入 `ctx.UserID`、`ctx.TenantID`、`ctx.Role`、`ctx.Permissions`。

### 登录接口变更

```
POST /api/v1/auth/login
  Request:  { "email": "str", "password": "str" }
  Response 200:
  {
    "token": "jwt_string",
    "user": {
      "id": "uuid",
      "username": "admin",
      "email": "admin@example.com",
      "phone": "13800138000",
      "tenant_id": 10000,
      "role": "tenant_admin",
      "permissions": ["kb:create", ...]
    }
  }
  Errors:
    401 INVALID_CREDENTIALS  — 邮箱或密码错误
    403 USER_DISABLED        — 用户已被禁用
    403 TENANT_DISABLED      — 所属租户已被禁用
    423 FIRST_LOGIN_RESET    — 首次登录需修改密码（重定向到密码修改页）
```

### 三种注册场景

```
场景 A — 自行注册（自动归入体验租户）
  POST /api/v1/auth/register
  输入：username + email + password + phone(可选)
  结果：创建 User + TenantUser(tenant_id=2, role=viewer)
  注意：不创建新的 Tenant；role 固定为 viewer（仅查看）
  Errors:
    409 EMAIL_EXISTS          — 邮箱已注册
    409 PHONE_EXISTS          — 手机号已注册
    403 REGISTRATION_DISABLED — 管理员关闭了自行注册
    429 TOO_MANY_REQUESTS     — 同 IP 短时间多次注册

场景 B — 管理员在租户内创建用户
  POST /api/v1/tenants/{id}/users
  输入：username + email + phone(可选) + role
  结果：创建 User + TenantUser，系统生成随机初始密码（8位）
  Errors:
    409 USER_EXISTS           — 同名/同邮箱/同手机用户已存在
    422 INVALID_ROLE          — 角色不在四角色范围内
    403 PLAN_USER_LIMIT       — 租户套餐用户数已达上限

场景 C — 系统管理员创建新租户
  POST /api/v1/admin/tenants
  输入：tenant_name + admin_username + admin_email + phone(可选) + plan_code
  结果：创建 Tenant + AdminUser + TenantUser(role=tenant_admin) + tenant_stats行
```

### 配置开关

```yaml
# config.yaml
tenant:
  allow_self_registration: true   # 是否允许自行注册
  self_registration_tenant_id: 2  # 自行注册归入的体验租户ID
  default_password_length: 8      # 管理员创建用户时初始密码长度
  force_password_change: true     # 首次登录是否强制修改密码
```

---

## 十一、T-09：删除 Organization

### 要删除的数据库表

```
organizations
organization_members
knowledge_base_shares
agent_shares
join_requests
tenant_disabled_shared_agents
```

### 要删除的后端代码

```
internal/types/organization.go
internal/types/interfaces/organization.go
internal/application/repository/organization.go
internal/application/repository/kb_share.go
internal/application/repository/agent_share.go
internal/application/service/organization.go
internal/application/service/kb_share.go
internal/application/service/agent_share.go
internal/handler/organization.go
router.go: RegisterOrganizationRoutes
router.go: kbShares/agentShares/shared-knowledge-bases 路由
```

### 要删除的前端代码

```
frontend/src/views/organization/ (整个目录)
frontend/src/api/organization/index.ts
frontend/src/stores/organization.ts
frontend/src/components/ListSpaceSidebar.vue (organization部分)
```

### 确认结论 ✅

全部删除，不留兼容。迁移脚本 `000040_drop_organizations.up.sql` 做 `DROP TABLE IF EXISTS` 系列操作。

---

## 十二、T-10：Token 配额统计

> 决策：租户级配额，由套餐控制上限。超额**阻断**。暂不计入 Embedding/Rerank。
> Token 用量写入 `tenant_stats.token_used`，不从 tenants 表存。

### 数据模型

配额上限由套餐配置 `plans.config.max_tokens_per_month` 控制，已使用量写入统计表：

```sql
-- 已由 tenant_stats 表覆盖，无需在 tenants 表额外添加字段
-- tenant_stats.token_used         -- 当月已用 token
-- tenant_stats.token_quota_reset_at  -- 下次重置时间
```

### 配额逻辑

1. 每次 **Chat 调用**（对话 + Agent）返回后，将 `response.Usage.TotalTokens` 累加到 `tenant_stats.token_used`
2. **暂不计入** Embedding / Rerank / VLM / ASR 调用
3. 调用 LLM **前**从套餐读取 `max_tokens_per_month`，同时查 `tenant_stats.token_used`，超额**阻断**返回错误码 `TOKEN_QUOTA_EXCEEDED`
4. 月度重置：定时任务每月 1 日 0 点重置 `tenant_stats.token_used = 0`，更新 `token_quota_reset_at`
5. **套餐升级**时：`token_used` 不重置，新上限 = 新套餐的 `max_tokens_per_month`；已用额度从新额度中扣除
6. 前端捕获 `TOKEN_QUOTA_EXCEEDED` 错误，提示"Token 已用完，请联系管理员升级套餐"
7. 捕获 `TRIAL_EXPIRED` 错误（体验到期），提示"试用已到期，请联系管理员升级"

---

## 十三、T-11：系统管理员后台

### 租户管理 API

| 操作 | 方法 | 路径 | 权限 |
|------|------|------|------|
| 列出全部租户 | GET | `/api/v1/admin/tenants` | system_admin |
| 查看租户详情 | GET | `/api/v1/admin/tenants/{id}` | system_admin |
| 创建租户 | POST | `/api/v1/admin/tenants` | system_admin |
| 编辑租户 | PUT | `/api/v1/admin/tenants/{id}` | system_admin |
| 禁用/启用租户 | PATCH | `/api/v1/admin/tenants/{id}/status` | system_admin |
| 设置租户套餐 | PUT | `/api/v1/admin/tenants/{id}/plan` | system_admin |
| 迁移用户到其他租户 | POST | `/api/v1/admin/users/{id}/migrate` | system_admin |
| 查看租户统计 | GET | `/api/v1/admin/tenants/{id}/stats` | system_admin |

> 租户详情接口返回关联的 `tenant_stats` 数据，前端可展示统计面板。

### 套餐管理 API（新增）

| 操作 | 方法 | 路径 | 权限 |
|------|------|------|------|
| 列出套餐 | GET | `/api/v1/admin/plans` | system_admin |
| 查看套餐 | GET | `/api/v1/admin/plans/{id}` | system_admin |
| 创建套餐 | POST | `/api/v1/admin/plans` | system_admin |
| 编辑套餐（含 config） | PUT | `/api/v1/admin/plans/{id}` | system_admin |
| 启用/停用套餐 | PATCH | `/api/v1/admin/plans/{id}/status` | system_admin |

### 业务规则

1. 禁用租户后，该租户下所有用户的 Token 立即失效
2. 系统租户（id=1）不可被禁用/删除；体验租户（id=2）不可被删除
3. 系统管理员可将体验租户中的用户迁移到任意正式租户
4. 停用套餐后，该套餐下所有租户调用大模型返回 `PLAN_DISABLED`
5. **升级套餐**时：已使用的配额从新配额中扣除；超出新上限的项目保留但不可新增
6. 系统管理员后台可查看任意租户的统计数据（直接查询 `tenant_stats`）

---

## 十四、确认结论汇总

| # | 问题 | 结论 |
|---|------|------|
| 1 | 同一 Tenant 下数据可见性 | KB/Agent 全部可见；Session 仅个人可见 |
| 2 | 系统租户是否承载业务数据 | 逻辑与普通租户一致，可承载业务 |
| 3 | 系统管理员账号创建方式 | 数据库种子脚本预置 |
| 4 | 删除用户时数据处理 | 逻辑删除（`is_active=false`），数据保留在租户内 |
| 5 | 是否需要自定义角色 | 不需要，预置四角色足够 |
| 6 | editor 资源控制权 | tenant_admin 集中管控；editor 可管理自己创建的资源 |
| 7 | 是否需要租户级菜单套餐 | 需要：体验版/基础版/专业版三级，套餐为独立实体可启用/停用 |
| 8 | 无权限菜单处理 | 直接隐藏 |
| 9 | 自行注册行为 | 允许，自动归入体验租户，角色为 viewer（仅查看） |
| 10 | Token 配额粒度 | 租户级，上限由套餐控制 |
| 11 | Embedding/Rerank 是否计入配额 | 暂不计入 |
| 12 | 一个用户属于几个租户 | **仅一个**（`tenant_users.user_id` 唯一约束） |
| 13 | 套餐可配置项 | 菜单范围、最大用户数、最大知识库数、最大存储、最大token数（可扩展） |
| 14 | 套餐升级配额处理 | 已使用从新配额中扣除；超限项保留但不可新增 |
| 15 | 统计数据存储方式 | 集中到 `tenant_stats` 表，计数器原子更新，配额校验直接读此表 |
| 16 | 数据一致性保障 | 原子 UPDATE + 悲观锁 + 定期对账 |
| 17 | 用户手机号 | users 表新增 phone 字段，注册/创建用户时可选填写 |
| 18 | Token 配额高并发方案 | Redis `tenant:{tid}:token_used` 原子计数器 + 每小时同步 DB |
| 19 | Token 黑名单 | Redis `blacklist:token:{jti}`，用户禁用/登出时加入 |
| 20 | 菜单获取方式 | `GET /api/v1/menus` 动态返回，含 Redis 缓存 (TTL=5min) |
| 21 | JWT 扩展 | Payload 新增 role + permissions，登录时注入 |
| 22 | 权限执行点 | 三层分工：中间件(role) + Service(owner) + Handler(LAST_ADMIN) ✅ 已确认 |
| 23 | 套餐停用后已进行对话 | 当前对话继续，下次对话时阻断 ✅ 已确认（方案B） |
| 24 | 首次登录强制改密 | 登录接口返回 423 FIRST_LOGIN_RESET，默认密码固定为 `admin_123456` |

---

## 十五、关键数据模型总览

```
┌───────────────────────────────────────────────────┐
│                     plans                          │
│  id │ code │ name │ is_enabled │ config (JSONB)   │
│     │      │      │            │ visible_menus     │
│     │      │      │            │ max_users         │
│     │      │      │            │ max_knowledge_bases│
│     │      │      │            │ max_storage_bytes │
│     │      │      │            │ max_tokens/month  │
│     │      │      │            │ trial_days ...    │
└──────────────┬────────────────────────────────────┘
               │ (plan_id FK)
               ▼
┌───────────────────────────────────────────────────┐
│                   tenants                          │
│  id │ name │ plan_id │ plan_code │ is_system       │
│     │ api_key │ status │ business │ ...(业务配置)  │
│     │ (storage_quota 已移除 → plans.config)        │
│     │ (storage_used  已移除 → tenant_stats)        │
├───────────────────────────────────────────────────┤
│  1  │ 系统管理  │ plan-01 │ pro   │ true            │
│  2  │ 体验空间  │ plan-02 │ trial │ false           │
│  3+ │ 正式租户  │ plan-03 │ basic │ false           │
└────────────┬──────────────────────────────────────┘
             │
    ┌────────┼─────────────────────────┐
    │        │                         │
    ▼        ▼                         ▼
┌──────────────┐ ┌──────────────┐ ┌─────────────────┐
│ tenant_users │ │ tenant_stats │ │ KB/Agent/Model  │
│ tenant_id    │ │ tenant_id(PK)│ │ /Session/Chunk  │
│ user_id(UNIQUE)│ user_count  │ │ /Knowledge ...  │
│ role ────────┼─│ kb_count    │ │ (tenant_id)     │
│ permissions  │ │ storage_used│ └─────────────────┘
└──────┬───────┘ │ token_used  │
       │         │ agent_count │
       ▼         │ session_cnt │
┌──────────────┐ └─────────────┘
│    users     │
│ id │ username│
│    │ email   │
│    │ phone   │
│    │ is_active│
│    │ (废弃tenant_id)│
└──────────────┘
```

---

## 十六、Redis Key Schema

### 设计原则

- Key 命名规范：`{domain}:{entity}:{identifier}`
- 所有 Key 设置合理 TTL，避免内存泄漏
- 缓存与 DB 最终一致（缓存失效后回源 DB）

### Key 清单

| Key | 类型 | TTL | 用途 |
|-----|------|:---:|------|
| `blacklist:token:{jti}` | String | JWT 剩余有效期 | Token 黑名单（用户禁用/登出后加入，中间件校验） |
| `tenant:{tid}:token_used` | String | 永久（定时回写 DB） | Token 配额计数器，每次 Chat 后 INCRBY，每小时回写 `tenant_stats.token_used`，每月 1 日 DEL |
| `menu:{tid}:{uid}` | String | 5 min | 用户可见菜单 JSON 缓存，登录时写入，角色/套餐变更时主动 DEL |
| `plan:{pid}:config` | String | 1 hour | 套餐配置 JSON 缓存，`plans.config` 的热缓存，plan 编辑时 DEL |
| `ratelimit:register:{ip}` | String | 1 min | 注册接口限流计数器（INCR + EXPIRE），超频返回 429 |
| `ratelimit:login:{ip}` | String | 5 min | 登录接口限流计数器（同 IP 5 次/5min） |

### 配额检查流程（Token）

```
每次 Chat 请求前:
  local_quota = GET tenant:{tid}:token_used
  if local_quota == null:
    local_quota = SELECT token_used FROM tenant_stats WHERE tenant_id = $tid
    SET tenant:{tid}:token_used = local_quota
  if local_quota >= plans.config.max_tokens_per_month:
    → 429 TOKEN_QUOTA_EXCEEDED

每次 Chat 返回后:
  INCRBY tenant:{tid}:token_used {total_tokens}

每小时定时任务:
  FOR EACH tenant:
    val = GET tenant:{tid}:token_used
    UPDATE tenant_stats SET token_used = val WHERE tenant_id = $tid
```

---

## 十七、完整错误码定义

### HTTP 状态码约定

| HTTP | 含义 |
|:----:|------|
| 400 | 请求参数校验失败 |
| 401 | 未认证 / Token 过期 |
| 403 | 已认证但无权限 / 配额/套餐限制 |
| 404 | 资源不存在 |
| 409 | 资源冲突（重复创建） |
| 422 | 请求格式正确但业务校验失败 |
| 423 | 资源被锁定（首次登录需改密） |
| 429 | 频率限制 |

### 错误码清单

| 错误码 | HTTP | 模块 | 触发条件 |
|--------|:----:|------|----------|
| `INVALID_CREDENTIALS` | 401 | Auth | 邮箱或密码错误 |
| `TOKEN_EXPIRED` | 401 | Auth | JWT 已过期 |
| `TOKEN_REVOKED` | 401 | Auth | Token 已被加入黑名单（用户禁用/登出） |
| `USER_DISABLED` | 403 | Auth | 用户 `is_active=false` |
| `TENANT_DISABLED` | 403 | Auth | 租户 `status != 'active'` |
| `FIRST_LOGIN_RESET` | 423 | Auth | 首次登录需修改密码 |
| `PERMISSION_DENIED` | 403 | RBAC | 用户无该操作权限 |
| `USER_NOT_IN_TENANT` | 403 | RBAC | 用户不属于该租户 |
| `EMAIL_EXISTS` | 409 | Register | 注册邮箱已被使用 |
| `PHONE_EXISTS` | 409 | Register | 手机号已被使用 |
| `USERNAME_EXISTS` | 409 | User | 用户名已被使用 |
| `REGISTRATION_DISABLED` | 403 | Register | 管理员关闭了自行注册 |
| `TOO_MANY_REQUESTS` | 429 | RateLimit | 同 IP 短时间超频 |
| `PLAN_DISABLED` | 403 | Plan | 套餐已被管理员停用 |
| `TRIAL_EXPIRED` | 403 | Plan | 体验租户已满 3 天 |
| `PLAN_USER_LIMIT` | 403 | Plan | 租户用户数已达套餐上限 |
| `PLAN_KB_LIMIT` | 403 | Plan | 知识库数已达套餐上限 |
| `PLAN_STORAGE_LIMIT` | 403 | Plan | 存储已达套餐上限 |
| `TOKEN_QUOTA_EXCEEDED` | 429 | Token | 月 Token 配额用尽 |
| `INVALID_ROLE` | 422 | User | 指定角色不在四角色范围内 |
| `LAST_ADMIN` | 422 | User | 不能移除/禁用租户最后一个 tenant_admin |
| `RESOURCE_NOT_FOUND` | 404 | Common | 请求的资源不存在 |

---

## 十八、定时任务

| 任务 | Cron | 描述 |
|------|------|------|
| `reconcile_tenant_stats` | 每天 03:00 | 遍历所有租户，对 `tenant_stats` 与业务表做 COUNT 对账，偏差自动修复 |
| `reset_token_quota` | 每月 1 日 00:00 | 所有租户 `tenant_stats.token_used = 0`，更新 `token_quota_reset_at`；DEL Redis `tenant:{*}:token_used` |
| `sync_token_redis_to_db` | 每小时 | 将 Redis `tenant:{tid}:token_used` 批量回写到 `tenant_stats.token_used` |
| `check_trial_expiry` | 每天 01:00 | 检查体验租户（plan_code=trial）中 `created_at + trial_days < NOW()` 的用户，标记为过期 |
| `clean_expired_tokens` | 每天 04:00 | 清理 `blacklist:token:*` 中已过期的 Key |

---

## 十九、验收场景

### T-01 Tenant 语义变更

| # | 类型 | 场景 | 预期结果 |
|---|:----:|------|----------|
| 1 | ✅ 正例 | 两个用户同属一个 Tenant，A 创建 KB，B 在列表中可见 | B 可查看/搜索该 KB |
| 2 | ❌ 反例 | 用户 A 的 Session 列表 | 不显示用户 B 的 Session |
| 3 | ❌ 反例 | 尝试将同一用户加入两个 Tenant | 返回 409 USER_EXISTS |

### T-03 套餐体系

| # | 类型 | 场景 | 预期结果 |
|---|:----:|------|----------|
| 1 | ✅ 正例 | 创建租户时指定 plan_code=basic | 租户套餐为 basic，10 用户/10KB/10GB 上限 |
| 2 | ❌ 反例 | 停用 basic 套餐后，basic 租户调用 Chat | 返回 403 PLAN_DISABLED |
| 3 | ❌ 反例 | trial 租户创建第 2 个知识库 | 返回 403 PLAN_KB_LIMIT |

### T-05 租户用户管理

| # | 类型 | 场景 | 预期结果 |
|---|:----:|------|----------|
| 1 | ✅ 正例 | tenant_admin 新增用户并设为 editor | 创建成功，user_count+1，editor 可创建 KB |
| 2 | ❌ 反例 | 移除租户最后一个 tenant_admin | 返回 422 LAST_ADMIN |
| 3 | ❌ 反例 | 禁用用户后使用其 Token 访问 | 返回 401 TOKEN_REVOKED |

### T-08 注册流程

| # | 类型 | 场景 | 预期结果 |
|---|:----:|------|----------|
| 1 | ✅ 正例 | 自行注册 → 登录 | 用户属于体验租户 (id=2)，role=viewer，只能查看 |
| 2 | ❌ 反例 | viewer 尝试创建知识库 | 返回 403 PERMISSION_DENIED |
| 3 | ❌ 反例 | 关闭自行注册后访问 register | 返回 403 REGISTRATION_DISABLED |

### T-10 Token 配额

| # | 类型 | 场景 | 预期结果 |
|---|:----:|------|----------|
| 1 | ✅ 正例 | 租户 token_used=80万,max=100万，调用 Chat | 正常返回，token_used 增加 |
| 2 | ❌ 反例 | 租户 token_used=100万,max=100万，调用 Chat | 返回 429 TOKEN_QUOTA_EXCEEDED |
| 3 | ❌ 反例 | 月初定时任务执行 | token_used 归零，reset_at 更新为下月 1 日 |
