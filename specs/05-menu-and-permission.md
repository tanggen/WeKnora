# 卡片 05：动态菜单 + 权限集成

> 覆盖：T-07（动态菜单 API + 前端路由守卫 + 权限中间件完整集成）
> 依赖：卡片 01（tenant_users + 角色体系）、卡片 02（套餐 plans）、卡片 03（JWT + 权限中间件）
> 基准需求：`specs/REQUIREMENTS.md` v1.4 第 7 节

---

## 1. API 契约

### 1.1 获取用户可见菜单

```
GET /api/v1/menus

权限：所有已登录用户
描述：返回当前用户可见的菜单树。
      过滤规则：菜单项的交集（套餐 features ∩ 角色 permissions）
      若租户无活跃套餐，返回基础菜单（仅"个人设置" + "退出登录"）

Response 200:
{
  "data": {
    "menus": [
      {
        "key": "knowledge",
        "label": "知识库",
        "icon": "library",
        "path": "/knowledge",
        "sort_order": 1,
        "children": [
          {
            "key": "kb-list",
            "label": "知识库列表",
            "path": "/knowledge/list",
            "sort_order": 1
          },
          {
            "key": "kb-create",
            "label": "创建知识库",
            "path": "/knowledge/create",
            "sort_order": 2,
            "required_permission": "kb:create"
          }
        ]
      },
      {
        "key": "chat",
        "label": "对话",
        "icon": "chat",
        "path": "/chat",
        "sort_order": 2,
        "children": [
          {
            "key": "chat-sessions",
            "label": "对话列表",
            "path": "/chat/sessions",
            "sort_order": 1
          }
        ]
      },
      {
        "key": "agents",
        "label": "Agent",
        "icon": "robot",
        "path": "/agents",
        "sort_order": 3,
        "required_feature": "agent",
        "children": [...]
      },
      {
        "key": "settings",
        "label": "设置",
        "icon": "settings",
        "path": "/settings",
        "sort_order": 90,
        "children": [
          {
            "key": "tenant-settings",
            "label": "租户管理",
            "path": "/settings/tenant",
            "required_permission": "tenant:manage"
          },
          {
            "key": "user-management",
            "label": "用户管理",
            "path": "/settings/users",
            "required_permission": "user:manage"
          },
          {
            "key": "model-config",
            "label": "模型配置",
            "path": "/settings/models",
            "required_permission": "model:manage"
          },
          {
            "key": "plan-info",
            "label": "套餐信息",
            "path": "/settings/plan"
          },
          {
            "key": "token-usage",
            "label": "用量统计",
            "path": "/settings/usage",
            "required_permission": "tenant:manage"
          }
        ]
      },
      {
        "key": "admin-panel",
        "label": "系统管理",
        "icon": "shield",
        "path": "/admin",
        "sort_order": 99,
        "required_permission": "system:admin",
        "children": [
          {
            "key": "admin-tenants",
            "label": "租户管理",
            "path": "/admin/tenants"
          },
          {
            "key": "admin-plans",
            "label": "套餐管理",
            "path": "/admin/plans"
          }
        ]
      }
    ],
    "cache_ttl": 300                 // 前端可据此设置本地缓存时间
  }
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
  403  ACCOUNT_DISABLED
```

### 1.2 获取当前用户权限

```
GET /api/v1/auth/permissions

权限：所有已登录用户
描述：返回当前用户的权限列表和角色信息。用于前端路由守卫判断。

Response 200:
{
  "data": {
    "user_id": "uuid-xxx",
    "tenant_id": 10042,
    "role": "editor",
    "permissions": ["kb:create", "kb:edit:own", "kb:delete:own", "agent:create", "agent:edit:own", "agent:delete:own", "chat:use"],
    "plan_id": "basic",
    "plan_features": ["chat", "search", "web_search"],
    "is_trial": false,
    "trial_expires_at": null
  }
}

Errors:
  401  TOKEN_EXPIRED / TOKEN_REVOKED
```

---

## 2. 数据库 DDL

本卡片无新建表。菜单数据不由数据库存储，使用代码内定义（`internal/config/menu_definition.go`）。

### 菜单项数据结构（Go struct）

```go
type MenuItem struct {
    Key                string         `json:"key" yaml:"key"`
    ParentKey          string         `json:"parent_key,omitempty" yaml:"parent_key"`
    LabelI18nKey       string         `json:"label_i18n_key" yaml:"label_i18n_key"`
    Label              string         `json:"label" yaml:"label"`          // 中文默认标签
    Icon               string         `json:"icon,omitempty" yaml:"icon"`
    Path               string         `json:"path,omitempty" yaml:"path"`
    SortOrder          int            `json:"sort_order" yaml:"sort_order"`
    RequiredPermission string         `json:"required_permission,omitempty" yaml:"required_permission"`
    RequiredFeature    string         `json:"required_feature,omitempty" yaml:"required_feature"`
    Children           []*MenuItem    `json:"children,omitempty"`
}
```

### 菜单定义配置（代码内常量）

```go
// internal/config/menu_definition.go
var DefaultMenuItems = []MenuItem{
    // ── 知识库 ──
    {Key: "knowledge", Label: "知识库", Icon: "library", Path: "/knowledge", SortOrder: 1},
    {Key: "kb-list", ParentKey: "knowledge", Label: "知识库列表", Path: "/knowledge/list", SortOrder: 1},
    {Key: "kb-create", ParentKey: "knowledge", Label: "创建知识库", Path: "/knowledge/create", SortOrder: 2, RequiredPermission: "kb:create"},

    // ── 对话 ──
    {Key: "chat", Label: "对话", Icon: "chat", Path: "/chat", SortOrder: 2, RequiredFeature: "chat"},
    {Key: "chat-sessions", ParentKey: "chat", Label: "会话", Path: "/chat/sessions", SortOrder: 1},

    // ── Web 搜索 ──
    {Key: "web-search", ParentKey: "chat", Label: "网络搜索", Path: "/chat/web-search", SortOrder: 2, RequiredFeature: "web_search"},

    // ── Agent ──
    {Key: "agents", Label: "Agent", Icon: "robot", Path: "/agents", SortOrder: 3, RequiredPermission: "agent:view"},
    {Key: "agents-list", ParentKey: "agents", Label: "Agent 列表", Path: "/agents/list", SortOrder: 1},
    {Key: "agents-create", ParentKey: "agents", Label: "创建 Agent", Path: "/agents/create", SortOrder: 2, RequiredPermission: "agent:create"},

    // ── 设置 ──
    {Key: "settings", Label: "设置", Icon: "settings", Path: "/settings", SortOrder: 90},
    {Key: "tenant-settings", ParentKey: "settings", Label: "租户管理", Path: "/settings/tenant", SortOrder: 1, RequiredPermission: "tenant:manage"},
    {Key: "user-management", ParentKey: "settings", Label: "用户管理", Path: "/settings/users", SortOrder: 2, RequiredPermission: "user:manage"},
    {Key: "model-config", ParentKey: "settings", Label: "模型配置", Path: "/settings/models", SortOrder: 3, RequiredPermission: "model:manage"},
    {Key: "plan-info", ParentKey: "settings", Label: "套餐信息", Path: "/settings/plan", SortOrder: 4},
    {Key: "token-usage", ParentKey: "settings", Label: "用量统计", Path: "/settings/usage", SortOrder: 5, RequiredPermission: "tenant:manage"},

    // ── 系统管理 ──
    {Key: "admin-panel", Label: "系统管理", Icon: "shield", Path: "/admin", SortOrder: 99, RequiredPermission: "system:admin"},
    {Key: "admin-tenants", ParentKey: "admin-panel", Label: "租户管理", Path: "/admin/tenants", SortOrder: 1},
    {Key: "admin-plans", ParentKey: "admin-panel", Label: "套餐管理", Path: "/admin/plans", SortOrder: 2},
    {Key: "admin-stats", ParentKey: "admin-panel", Label: "数据统计", Path: "/admin/stats", SortOrder: 3},
}
```

---

## 3. Redis Key Schema

| Key | 类型 | TTL | 用途 |
|-----|------|:---:|------|
| `menu:{tenant_id}:{user_id}` | String (JSON) | 5 分钟 | 用户菜单缓存。首次请求生成后缓存，后续直接从 Redis 返回。 |
| `perm:{tenant_id}:{user_id}` | String (JSON) | 5 分钟 | 用户权限快照缓存 |

### 缓存失效条件

```
菜单缓存失效（需清除 menu:xxx:xxx + perm:xxx:xxx）：
  - 用户角色变更（卡片01 1.6 PUT /role）
  - 租户套餐变更（卡片02 1.7 POST /plan）
  - 套餐配置编辑（卡片02 1.6.2 PUT /plans/{id}）
  - 用户被禁用/删除
```

### 菜单过滤算法

```go
func BuildMenuTree(allItems []MenuItem, userPerms []string, planFeatures []string) []MenuItem {
    permSet := toSet(userPerms)
    featureSet := toSet(planFeatures)

    // Step 1: 过滤叶子节点
    var filtered []MenuItem
    for _, item := range allItems {
        // 权限检查
        if item.RequiredPermission != "" && !permSet.Contains(item.RequiredPermission) {
            continue
        }
        // 套餐特性检查
        if item.RequiredFeature != "" && !featureSet.Contains(item.RequiredFeature) {
            continue
        }
        filtered = append(filtered, item)
    }

    // Step 2: 构建树（父节点自动保留如果它有子节点）
    // Step 3: 按 sort_order 排序
    // Step 4: 去除空的父节点（没有可见子节点的父节点也隐藏）
    return buildTree(filtered)
}
```

---

## 4. 定时任务

本卡片无独立定时任务。

---

## 5. 业务规则汇总

### 5.1 菜单可见性控制

| 菜单项 | 显示条件 |
|--------|----------|
| 知识库列表 | 所有人可见（> viewer） |
| 创建知识库 | 有 `kb:create` 权限 |
| 对话 | 套餐 features 含 `chat` |
| 网络搜索 | 套餐 features 含 `web_search` |
| Agent | 套餐 features 含 `agent` OR 有 `agent:view` 权限 |
| 用户管理 | 有 `user:manage` 权限 |
| 租户管理 | 有 `tenant:manage` 权限 |
| 模型配置 | 有 `model:manage` 权限 |
| 系统管理面板 | 有 `system:admin` 权限 |
| 套餐信息 | 所有人可见 |
| 用量统计 | 有 `tenant:manage` 权限 |

### 5.2 权限中间件集成到路由注册

```go
// internal/router/router.go

func RegisterRoutes(r *gin.Engine) {
    api := r.Group("/api/v1")
    api.Use(middleware.AuthMiddleware())  // JWT 解析 → 校验 → 注入 context

    // ── 认证（无角色限制）──
    auth := api.Group("/auth")
    {
        auth.POST("/register", handler.Register)
        auth.POST("/login", handler.Login)
        auth.POST("/logout", handler.Logout)
        auth.POST("/change-password", handler.ChangePassword)
        auth.POST("/refresh", handler.RefreshToken)
        auth.GET("/permissions", handler.GetPermissions)      // 卡片05 1.2
    }

    // ── 菜单（所有登录用户）──
    api.GET("/menus", handler.GetMenus)                       // 卡片05 1.1

    // ── 知识库 ──
    kb := api.Group("/knowledge-bases")
    {
        kb.GET("", handler.ListKnowledgeBases)                // 所有角色可查看
        kb.POST("", middleware.RequireRole("tenant_admin","editor","system_admin"), handler.CreateKnowledgeBase) // kb:create
        kb.PUT("/:id", middleware.RequireRole("tenant_admin","editor","system_admin"), handler.UpdateKnowledgeBase)
        kb.DELETE("/:id", middleware.RequireRole("tenant_admin","editor","system_admin"), handler.DeleteKnowledgeBase)
    }

    // ── 租户内用户管理 ──
    users := api.Group("/tenants/:tenant_id/users")
    users.Use(middleware.RequireRole("tenant_admin", "system_admin"))
    {
        users.GET("", handler.ListUsers)
        users.POST("", handler.CreateUser)
        users.PUT("/:user_id", handler.UpdateUser)
        users.PATCH("/:user_id/status", handler.SetUserStatus)
        users.DELETE("/:user_id", handler.DeleteUser)
        users.PUT("/:user_id/role", handler.SetUserRole)
    }

    // ── 统计 ──
    stats := api.Group("/tenants/:tenant_id/stats")
    stats.Use(middleware.RequireRole("tenant_admin", "system_admin"))
    {
        stats.GET("", handler.GetTenantStats)
    }

    // ── 系统管理（仅 system_admin）──
    admin := api.Group("/admin")
    admin.Use(middleware.RequireRole("system_admin"))
    {
        admin.GET("/stats/overview", handler.GetAdminOverview)
        admin.GET("/stats/tenants", handler.GetAdminTenantStats)
        admin.POST("/plans", handler.CreatePlan)
        admin.PUT("/plans/:id", handler.UpdatePlan)
        admin.DELETE("/plans/:id", handler.DeletePlan)
        admin.PATCH("/plans/:id/status", handler.SetPlanStatus)
        admin.POST("/tenants/:id/plan", handler.SetTenantPlan)
    }
}
```

### 5.3 套餐停用后的菜单行为

```
┌── 套餐正常时 ─────────────────────────────────────┐
│  菜单 = 角色权限 ∩ 套餐 features                     │
│  例：basic 套餐 editor → 知识库 + 对话 + Agent       │
└──────────────────────────────────────────────────┘

┌── 套餐停用/到期（trial_expired）时 ───────────────┐
│  菜单 = ["settings/plan-info"] 仅保留套餐信息页       │
│  目的：引导用户升级                                 │
│  前端看到只有"套餐信息"页面                          │
└──────────────────────────────────────────────────┘
```

---

## 6. 验收测试场景

| # | 类型 | 场景 | 操作 | 预期 |
|---|:----:|------|------|------|
| 1 | ✅ | tenant_admin 获取完整菜单 | tenant_admin GET /menus | 返回设置（含用户管理）+ 知识库（含创建）+ 对话 + Agent |
| 2 | ✅ | viewer 获取受限菜单 | viewer GET /menus | 返回 知识库列表 + 对话 + 套餐信息，无"创建知识库"、无"设置" |
| 3 | ✅ | system_admin 获取超管菜单 | system_admin GET /menus | 返回全部菜单 + "系统管理"面板 |
| 4 | ❌ | trial 套餐不显示高级功能 | trial 租户 editor GET /menus | 无"网络搜索"菜单项（trial features 无 web_search） |
| 5 | ❌ | 过期套餐仅显示套餐信息页 | trial_expired 租户 GET /menus | 返回仅含 settings/plan-info 一项 |
| 6 | ❌ | 角色变更后菜单刷新 | editor → 升级为 tenant_admin → GET /menus | 缓存失效，新菜单含用户管理 |

---

## 7. 文件清单

### 新建文件

| 文件 | 说明 |
|------|------|
| `internal/config/menu_definition.go` | 菜单项定义（DefaultMenuItems 常量 + MenuItem struct） |
| `internal/application/service/menu.go` | MenuService：菜单过滤 + 树构建 + 权限计算 |
| `internal/handler/menu.go` | Handler：GET /menus、GET /auth/permissions |

### 修改文件

| 文件 | 说明 |
|------|------|
| `internal/router/router.go` | 完整路由注册（含所有模块的 RequireRole，以及 /menus、/auth/permissions） |
| `internal/middleware/permission.go` | `RequireRole(...)` 中间件实现（根据 context 中的 role 检验） |
| `internal/middleware/auth.go` | AuthMiddleware 在注入 context 后，额外注入 permissions（从 tenant_users 获取） |
| `internal/application/service/tenant_user.go` | 角色变更时调用 `menu.InvalidateCache(tenantID, userID)` 清除 Redis |
| `internal/application/service/plan.go` | 套餐分配时调用 `menu.InvalidateAllTenantCache(tenantID)` |
| `internal/container/container.go` | 注册 MenuService、MenuHandler |

---

## 质量门控

- [x] 每个 P0 功能都有对应 API 端点 + Request + Response + Errors（2 个端点完整）
- [x] 菜单数据结构完整定义（MenuItem struct + 完整 DefaultMenuItems 清单）
- [x] 每个 Redis Key 标注了类型和 TTL（2 个 Key + 缓存失效条件）
- [x] 定时任务已标注（无独立任务）
- [x] 验收场景至少 1 正例 + 2 反例（3 正例 + 3 反例）
- [x] Agent 拿到卡片后 0 问题可直接开始编码
