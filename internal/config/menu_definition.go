package config

// MenuItem represents a single menu item in the sidebar navigation
type MenuItem struct {
	Key                string      `json:"key" yaml:"key"`
	ParentKey          string      `json:"parent_key,omitempty" yaml:"parent_key"`
	Label              string      `json:"label" yaml:"label"`
	Icon               string      `json:"icon,omitempty" yaml:"icon"`
	Path               string      `json:"path,omitempty" yaml:"path"`
	SortOrder          int         `json:"sort_order" yaml:"sort_order"`
	RequiredPermission string      `json:"required_permission,omitempty" yaml:"required_permission"`
	RequiredFeature    string      `json:"required_feature,omitempty" yaml:"required_feature"`
	Children           []*MenuItem `json:"children,omitempty"`
}

// DefaultMenuItems defines the full menu tree available in the system.
// Menus are filtered at runtime by user permissions and plan features.
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
