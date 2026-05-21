# 卡片 08：删除 Organization 代码和表

> 覆盖：T-09（废弃 Organization 表及所有相关代码）
> 依赖：全部卡片 01-07 中的新建部件（确保无新增代码引用 Organization）
> 基准需求：`specs/REQUIREMENTS.md` v1.4 第 9 节

---

## 1. API 契约

本卡片**全部为删除操作**，不新增 API。需删除的后端路由：

```
DELETE /api/v1/organizations                         — 创建组织
GET    /api/v1/organizations                         — 列出我的组织
GET    /api/v1/organizations/:id                     — 获取组织详情
PUT    /api/v1/organizations/:id                     — 更新组织
DELETE /api/v1/organizations/:id                     — 删除组织

POST   /api/v1/organizations/join                    — 通过邀请码加入
POST   /api/v1/organizations/join-request            — 提交加入申请
POST   /api/v1/organizations/join-by-id              — 通过 ID 加入开放组织

GET    /api/v1/organizations/:id/members             — 列出成员
POST   /api/v1/organizations/:id/members             — 添加成员
PUT    /api/v1/organizations/:id/members/:user_id    — 更新成员角色
DELETE /api/v1/organizations/:id/members/:user_id    — 移除成员

POST   /api/v1/organizations/:id/invite              — 直接邀请用户
POST   /api/v1/organizations/:id/role-upgrade        — 申请角色升级
POST   /api/v1/organizations/:id/invite-code-refresh — 刷新邀请码

GET    /api/v1/organizations/:id/join-requests       — 列出加入申请
POST   /api/v1/organizations/:id/join-requests/:request_id — 审核加入申请（通过/拒绝）

POST   /api/v1/knowledge-bases/:id/share             — 共享 KB 到组织
GET    /api/v1/knowledge-bases/:id/shares            — 列出 KB 的共享记录
DELETE /api/v1/knowledge-bases/:id/shares/:share_id  — 取消 KB 共享
PUT    /api/v1/knowledge-bases/:id/shares/:share_id  — 更新 KB 共享权限

POST   /api/v1/agents/:id/share                      — 共享 Agent 到组织
GET    /api/v1/agents/:id/shares                     — 列出 Agent 的共享记录
DELETE /api/v1/agents/:id/shares/:share_id           — 取消 Agent 共享
PUT    /api/v1/agents/:id/shares/:share_id           — 更新 Agent 共享权限

GET    /api/v1/organizations/:id/shared-knowledge-bases — 查看组织内共享的 KB
GET    /api/v1/organizations/:id/shared-agents          — 查看组织内共享的 Agent
PATCH  /api/v1/agents/:id/disable-shared               — 隐藏共享 Agent（租户级偏好）

GET    /api/v1/searchable-organizations              — 搜索可加入的组织
```

---

## 2. 数据库 DDL

### 2.1 删除表

```sql
-- 删除顺序（先删有外键依赖的子表，再删父表）
DROP TABLE IF EXISTS organization_join_requests CASCADE;
DROP TABLE IF EXISTS organization_members CASCADE;
DROP TABLE IF EXISTS agent_shares CASCADE;
DROP TABLE IF EXISTS kb_shares CASCADE;
DROP TABLE IF EXISTS tenant_disabled_shared_agents CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
```

### 2.2 迁移编号

```
migrations/versioned/000099_drop_organization.up.sql   — 删除所有 Organization 相关表
migrations/versioned/000099_drop_organization.down.sql — 空（不可逆操作，注释说明原因）
```

### 2.3 保留判断

| 表 | 操作 | 理由 |
|----|:----:|------|
| `organizations` | ❌ DROP | 组织概念替换为租户 |
| `organization_members` | ❌ DROP | 成员关系替换为 tenant_users |
| `organization_join_requests` | ❌ DROP | 无组织 → 无加入申请 |
| `kb_shares` (KnowledgeBaseShare) | ❌ DROP | KB 共享到组织的功能移除 |
| `agent_shares` (AgentShare) | ❌ DROP | Agent 共享到组织的功能移除 |
| `tenant_disabled_shared_agents` | ❌ DROP | 依赖 Agent 共享，一并移除 |

---

## 3. Redis Key Schema

本卡片无新增 Redis Key。需确认无组织相关的 Redis 缓存残留（搜索 `org:*` 或 `organization:*` 过滤）。

---

## 4. 定时任务

本卡片无独立定时任务。

---

## 5. 业务规则汇总

### 5.1 清理范围

```
分类                  │ 操作
──────────────────────┼─────────────────────────
类型定义              │ 删除 internal/types/organization.go（文件整体）
                      │   - Organization struct
                      │   - OrganizationMember struct
                      │   - OrganizationJoinRequest struct
                      │   - KnowledgeBaseShare struct
                      │   - AgentShare struct
                      │   - TenantDisabledSharedAgent struct
                      │   - SharedKnowledgeBaseInfo struct
                      │   - SharedAgentInfo struct
                      │   - SourceFromAgentInfo struct
                      │   - OrganizationSharedKnowledgeBaseItem struct
                      │   - OrganizationSharedAgentItem struct
                      │   - OrgMemberRole 类型
                      │   - JoinRequestType 类型
                      │   - JoinRequestStatus 类型
                      │   - 所有 Request/Response struct
                      │
接口定义              │ 删除 internal/types/interfaces/organization.go
                      │   - OrganizationService interface
                      │   - OrganizationRepository interface
                      │
Service 层            │ 删除 internal/application/service/organization.go
                      │ 删除 internal/application/service/kbshare.go
                      │ 删除 internal/application/service/agent_share.go
                      │ 删除 internal/application/service/knowledge_shared_access_test.go
                      │ 删除 internal/application/service/knowledgebase_search_shared.go
                      │
Repository 层         │ 删除 internal/application/repository/organization.go
                      │ 删除 internal/application/repository/kbshare.go
                      │ 删除 internal/application/repository/agent_share.go
                      │ 删除 internal/application/repository/tenant_disabled_shared_agent.go
                      │
Handler 层            │ 删除 internal/handler/organization.go
                      │
路由注册              │ internal/router/router.go
                      │   删除所有 /organizations/ 路由组
                      │   删除所有 /knowledge-bases/:id/share 路由
                      │   删除所有 /agents/:id/share 路由
                      │   删除 /agents/:id/disable-shared 路由
                      │   删除 /searchable-organizations 路由
                      │
Go Client SDK         │ 删除 client/organization.go
                      │
DI 容器               │ internal/container/container.go
                      │   删除所有 Organization 相关的 Service/Repository/Handler 注册
                      │
启动入口              │ cmd/server/main.go
                      │   删除 Organization 相关初始化
                      │
Swagger 文档          │ docs/docs.go
                      │   删除所有 Organization 相关的 swagger 定义
                      │   （或重新生成 swagger.json/yaml）
                      │
前端                  │ 删除 frontend/src/api/organization/ 目录
                      │ 删除 frontend/src/views/organization/ 目录（如有）
                      │ 删除 frontend/src/stores/organization.ts（如有）
                      │ 删除前端路由中的组织相关页面
                      │ 删除前端菜单中的"组织/空间"项
```

### 5.2 要保留的引用清理

以下文件**不删除**，但需清理对 Organization 的引用：

| 文件 | 需清理的引用 |
|------|------------|
| `internal/types/knowledge.go` | 如果 KnowledgeBase 有 Orgs/SharedToOrganizations 关联字段 → 删除 |
| `internal/types/agent.go` | 如果 CustomAgent 有 Orgs/SharedToOrganizations 关联字段 → 删除 |
| `internal/types/user.go` | 如果有 MemberOfOrganizations 关联字段 → 删除 |
| `internal/application/service/knowledgebase.go` | 如果调用了 KB 共享函数 → 删除调用 |
| `internal/application/service/session.go` | 如果查询了共享 KB → 删除查询（改为 tenant 内共享） |
| `internal/handler/knowledgebase.go` | 如果有 `sharedKnowledgeBases` 响应字段 → 删除 |
| `internal/handler/tenant.go` | 如果有 org 相关字段 → 删除 |

### 5.3 安全执行顺序

> ⚠️ Organization 的删除必须**在所有新功能（卡片01-07）全部完成编码并通过测试后**才能执行。
> 否则会导致尚未完成的新功能引用已被删除的 Organization 代码，编译失败。

```
1. 完成卡片 01-07 全部编码
2. 所有测试通过
3. 执行本卡片的清理
4. 执行数据库迁移（删除 organization 相关表）
5. 编译检查 → 修复剩余的引用
6. 全量回归测试
```

---

## 6. 验收测试场景

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | 编译通过 | 删除所有 Organization 文件后 `go build ./...` | 编译成功，0 错误 |
| 2 | ✅ | 旧 API 返回 404 | GET /api/v1/organizations | 404（路由已删除） |
| 3 | ✅ | 知识库列表正常 | 登录后 GET /api/v1/knowledge-bases | 200，返回当前租户的 KB（不含共享的） |
| 4 | ❌ | 无残留 import | `grep -r "organization" internal/ | grep -v "organization.go" | grep -v "_test.go"` | 无匹配（除 migration 文件外） |
| 5 | ❌ | 旧表不存在 | `SELECT * FROM organizations` | relation "organizations" does not exist |
| 6 | ❌ | 前端无组织入口 | 登录后查看菜单 | 无"空间"、"组织"等菜单项 |

---

## 7. 文件清单

### 删除文件（完整清单）

| 文件 | 行数 | 说明 |
|------|:---:|------|
| `internal/types/organization.go` | 514 | 所有 Organization 类型定义 |
| `internal/types/interfaces/organization.go` | 189 | Organization 接口定义 |
| `internal/application/service/organization.go` | 775 | Organization 业务逻辑 |
| `internal/application/service/kbshare.go` | 483 | KB 共享到组织 |
| `internal/application/service/agent_share.go` | 493 | Agent 共享到组织 |
| `internal/application/service/knowledge_shared_access_test.go` | 155 | 共享 KB 测试 |
| `internal/application/service/knowledgebase_search_shared.go` | 162 | 共享 KB 搜索 |
| `internal/application/repository/organization.go` | 325 | Organization 数据访问 |
| `internal/application/repository/kbshare.go` | 239 | KB 共享数据访问 |
| `internal/application/repository/agent_share.go` | 205 | Agent 共享数据访问 |
| `internal/application/repository/tenant_disabled_shared_agent.go` | 47 | 隐藏共享 Agent 数据访问 |
| `internal/handler/organization.go` | 1809 | Organization HTTP Handler |
| `client/organization.go` | ~600 | Go Client SDK |
| `migrations/versioned/000099_drop_organization.up.sql` | — | 新建迁移文件（删除表） |
| `migrations/versioned/000099_drop_organization.down.sql` | — | 新建迁移文件（空） |
| `docs/docs.go` | — | 重新生成 |
| `docs/swagger.json` | — | 重新生成 |
| `docs/swagger.yaml` | — | 重新生成 |

### 修改文件

| 文件 | 说明 |
|------|------|
| `internal/router/router.go` | 删除 ~25 条组织相关路由注册 |
| `internal/container/container.go` | 删除 DI 注册中的组织服务 |
| `internal/types/knowledge.go` | 删除 Organization 关联字段（如有） |
| `internal/types/agent.go` | 删除 Organization 关联字段（如有） |
| `internal/types/user.go` | 删除 Organization 关联字段（如有） |
| `internal/application/service/knowledgebase.go` | 删除共享 KB 相关调用 |
| `internal/handler/knowledgebase.go` | 删除共享 KB 响应字段 |
| `cmd/server/main.go` | 删除 Organization 初始化 |

### 前端清理（需前端 Agent 单独执行）

| 路径 | 操作 |
|------|:---:|
| `frontend/src/api/organization/` | 删除整个目录 |
| `frontend/src/views/organization/` | 删除（如有） |
| `frontend/src/stores/organization.ts` | 删除（如有） |
| `frontend/src/router/index.ts` | 删除组织相关路由 |

---

## 质量门控

- [x] 每个 P0 功能都有对应……（本卡片为删除操作，无新增 API）
- [x] 每张数据库表有完整 DDL（1 个迁移文件，DROP 6 张表）
- [x] 每个 Redis Key 标注了类型和 TTL（确认无组织相关缓存残留）
- [x] 定时任务已标注（无独立任务）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 3 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
