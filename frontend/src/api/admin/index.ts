import { get, post, put, patch } from '@/utils/request'

// ============================================================
// Types
// ============================================================

export interface AdminTenantListItem {
  tenant_id: number
  name: string
  description: string
  plan_id: string
  plan_name: string
  status: string
  is_system: boolean
  trial_expires_at?: string
  user_count: number
  storage_used_bytes: number
  token_used_monthly: number
  knowledge_base_count: number
  created_at: string
}

export interface TenantUserInfoItem {
  user_id: string
  username: string
  email: string
  role: string
  is_active: boolean
  joined_at: string
}

export interface TenantDetailStats {
  knowledge_base_count: number
  knowledge_count: number
  chunk_count: number
  storage_used_bytes: number
  token_used_monthly: number
  token_used_total: number
  agent_count: number
  session_count: number
}

export interface AdminTenantDetail {
  tenant_id: number
  name: string
  description: string
  plan_id: string
  plan_name: string
  status: string
  is_system: boolean
  trial_expires_at?: string
  storage_quota_bytes: number
  created_at: string
  users: TenantUserInfoItem[]
  stats: TenantDetailStats
}

export interface AdminOverviewStats {
  total_tenants: number
  active_tenants: number
  total_users: number
  total_knowledge_bases: number
  total_storage_used_bytes: number
  total_token_used_all_time: number
  trials_active: number
  trials_converted: number
}

export interface AdminTenantStatsItem {
  tenant_id: number
  tenant_name: string
  plan_name: string
  user_count: number
  storage_used_bytes: number
  token_used: number
  token_quota_monthly: number
  is_active: boolean
  created_at: string
}

export interface PlanConfig {
  max_users: number
  max_knowledge_bases: number
  max_knowledge_per_kb: number
  max_chunks_per_knowledge: number
  storage_quota_bytes: number
  token_quota_monthly: number
  max_agents: number
  price_monthly_cny: number
  price_yearly_cny: number
  features: string[]
  marketing_features: string[]
  updated_at: string
}

export interface PlanWithConfig {
  plan_id: string
  name: string
  description: string
  is_trial: boolean
  trial_days: number
  is_active: boolean
  sort_order: number
  created_at: string
  updated_at: string
  config: PlanConfig
}

export interface CreatePlanRequest {
  plan_id: string
  name: string
  description?: string
  is_trial?: boolean
  trial_days?: number
  max_users?: number
  max_knowledge_bases?: number
  max_knowledge_per_kb?: number
  max_chunks_per_knowledge?: number
  storage_quota_bytes?: number
  token_quota_monthly?: number
  max_agents?: number
  features?: string[]
  marketing_features?: string[]
  price_monthly_cny?: number
  price_yearly_cny?: number
  sort_order?: number
}

export interface UpdatePlanRequest {
  name?: string
  description?: string
  is_trial?: boolean
  trial_days?: number
  max_users?: number
  max_knowledge_bases?: number
  max_knowledge_per_kb?: number
  max_chunks_per_knowledge?: number
  storage_quota_bytes?: number
  token_quota_monthly?: number
  max_agents?: number
  features?: string[]
  marketing_features?: string[]
  price_monthly_cny?: number
  price_yearly_cny?: number
  sort_order?: number
}

export interface PaginatedResponse<T> {
  success: boolean
  data: T[]
  total: number
  page: number
  page_size: number
}

export interface ApiResponse<T> {
  success: boolean
  data: T
  message?: string
}

// ============================================================
// Admin Tenant APIs
// ============================================================

export function listTenants(params: {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
  plan_id?: string
}): Promise<PaginatedResponse<AdminTenantListItem>> {
  const query = new URLSearchParams()
  if (params.page) query.set('page', String(params.page))
  if (params.page_size) query.set('page_size', String(params.page_size))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.status) query.set('status', params.status)
  if (params.plan_id) query.set('plan_id', params.plan_id)
  return get(`/api/v1/admin/tenants?${query.toString()}`) as Promise<PaginatedResponse<AdminTenantListItem>>
}

export function getTenant(tenantId: number): Promise<ApiResponse<AdminTenantDetail>> {
  return get(`/api/v1/admin/tenants/${tenantId}`) as Promise<ApiResponse<AdminTenantDetail>>
}

export function updateTenant(tenantId: number, data: { name?: string; description?: string }): Promise<ApiResponse<null>> {
  return put(`/api/v1/admin/tenants/${tenantId}`, data) as Promise<ApiResponse<null>>
}

export function setTenantStatus(tenantId: number, status: string): Promise<ApiResponse<null>> {
  return patch(`/api/v1/admin/tenants/${tenantId}/status`, { status }) as Promise<ApiResponse<null>>
}

export function assignPlan(tenantId: number, planId: string): Promise<ApiResponse<any>> {
  return post(`/api/v1/admin/tenants/${tenantId}/plan`, { plan_id: planId }) as Promise<ApiResponse<any>>
}

export function getTenantUsers(tenantId: number, params: {
  page?: number
  page_size?: number
  keyword?: string
  role?: string
  status?: string
}): Promise<PaginatedResponse<TenantUserInfoItem>> {
  const query = new URLSearchParams()
  if (params.page) query.set('page', String(params.page))
  if (params.page_size) query.set('page_size', String(params.page_size))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.role) query.set('role', params.role)
  if (params.status) query.set('status', params.status)
  return get(`/api/v1/admin/tenants/${tenantId}/users?${query.toString()}`) as Promise<PaginatedResponse<TenantUserInfoItem>>
}

// ============================================================
// Admin Plan APIs
// ============================================================

export function listAllPlans(): Promise<ApiResponse<PlanWithConfig[]>> {
  return get('/api/v1/admin/plans') as Promise<ApiResponse<PlanWithConfig[]>>
}

export function getPlan(planId: string): Promise<ApiResponse<PlanWithConfig>> {
  return get(`/api/v1/admin/plans/${planId}`) as Promise<ApiResponse<PlanWithConfig>>
}

export function createPlan(data: CreatePlanRequest): Promise<ApiResponse<PlanWithConfig>> {
  return post('/api/v1/admin/plans', data) as Promise<ApiResponse<PlanWithConfig>>
}

export function updatePlan(planId: string, data: UpdatePlanRequest): Promise<ApiResponse<PlanWithConfig>> {
  return put(`/api/v1/admin/plans/${planId}`, data) as Promise<ApiResponse<PlanWithConfig>>
}

export function setPlanStatus(planId: string, isActive: boolean): Promise<ApiResponse<any>> {
  return patch(`/api/v1/admin/plans/${planId}/status`, { is_active: isActive }) as Promise<ApiResponse<any>>
}

// ============================================================
// Admin Stats APIs
// ============================================================

export function getAdminOverview(): Promise<ApiResponse<AdminOverviewStats>> {
  return get('/api/v1/admin/stats/overview') as Promise<ApiResponse<AdminOverviewStats>>
}

export function getAdminTenantStats(params: {
  page?: number
  page_size?: number
  keyword?: string
  sort_by?: string
  order?: string
}): Promise<PaginatedResponse<AdminTenantStatsItem>> {
  const query = new URLSearchParams()
  if (params.page) query.set('page', String(params.page))
  if (params.page_size) query.set('page_size', String(params.page_size))
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.sort_by) query.set('sort_by', params.sort_by)
  if (params.order) query.set('order', params.order)
  return get(`/api/v1/admin/stats/tenants?${query.toString()}`) as Promise<PaginatedResponse<AdminTenantStatsItem>>
}
