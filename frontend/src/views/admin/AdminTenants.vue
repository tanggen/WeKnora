<template>
  <div class="admin-tenants-container">
    <div class="tenants-content">
      <div class="header">
        <div class="header-title">
          <div class="title-row">
            <t-button variant="text" size="small" class="back-btn" @click="router.push('/platform/admin')">
              <template #icon><t-icon name="chevron-left" size="18px" /></template>
            </t-button>
            <h2>{{ $t('admin.tenants.title') }}</h2>
          </div>
          <p class="header-subtitle">{{ $t('admin.tenants.subtitle') }}</p>
        </div>
      </div>

      <div class="tenants-main">
        <!-- 搜索与筛选 -->
        <div class="filter-bar">
          <t-input
            v-model="keyword"
            :placeholder="$t('admin.tenants.searchPlaceholder')"
            clearable
            size="medium"
            style="width: 280px"
            @change="debouncedFetch"
            @clear="fetchTenants"
          >
            <template #prefix-icon><t-icon name="search" /></template>
          </t-input>
          <t-select
            v-model="statusFilter"
            :placeholder="$t('admin.tenants.statusFilter')"
            clearable
            size="medium"
            style="width: 140px"
            @change="fetchTenants"
          >
            <t-option value="active" :label="$t('admin.tenants.statusActive')" />
            <t-option value="suspended" :label="$t('admin.tenants.statusSuspended')" />
            <t-option value="trial" :label="$t('admin.tenants.statusTrial')" />
          </t-select>
        </div>

        <!-- 骨架屏 -->
        <div v-if="loading && tenants.length === 0" class="table-skeleton">
          <t-skeleton
            v-for="n in 8"
            :key="'skel-' + n"
            animation="gradient"
            :row-col="[{ width: '100%', height: '48px' }]"
            style="margin-bottom: 8px"
          />
        </div>

        <!-- 租户列表 -->
        <t-table
          v-else
          :data="tenants"
          :columns="columns"
          row-key="tenant_id"
          size="medium"
          :hover="true"
          :loading="loading"
          :pagination="pagination"
          @page-change="handlePageChange"
          @row-click="handleRowClick"
        >
          <template #name="{ row }">
            <div class="tenant-name-cell">
              <span class="tenant-name">{{ row.name }}</span>
              <t-tag v-if="row.is_system" theme="primary" size="small" variant="light">
                {{ $t('admin.tenants.system') }}
              </t-tag>
            </div>
          </template>
          <template #status="{ row }">
            <t-tag :theme="statusTheme(row.status)" size="small" variant="light">
              {{ statusLabel(row.status) }}
            </t-tag>
          </template>
          <template #storage_used_bytes="{ row }">
            {{ formatBytes(row.storage_used_bytes) }}
          </template>
          <template #token_used_monthly="{ row }">
            {{ formatTokens(row.token_used_monthly) }}
          </template>
          <template #created_at="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
          <template #op="{ row }">
            <t-space>
              <t-button variant="text" size="small" @click.stop="openDetail(row)">
                {{ $t('common.view') }}
              </t-button>
              <t-button variant="text" size="small" @click.stop="openEditDialog(row)">
                {{ $t('common.edit') }}
              </t-button>
            </t-space>
          </template>
        </t-table>

        <!-- 空状态 -->
        <div v-if="!loading && tenants.length === 0" class="empty-state">
          <img class="empty-img" src="@/assets/img/upload.svg" alt="" />
          <span class="empty-txt">{{ $t('admin.tenants.empty') }}</span>
        </div>
      </div>
    </div>

    <!-- 租户详情侧边栏 -->
    <Transition name="drawer">
      <div v-if="detailVisible && detailTenant" class="detail-drawer-overlay" @click.self="closeDetail">
        <div class="detail-drawer">
          <div class="drawer-header">
            <h3 class="drawer-title">{{ detailTenant.name }}</h3>
            <button class="drawer-close" @click="closeDetail" :aria-label="$t('common.close')">
              <t-icon name="close" />
            </button>
          </div>

          <div v-if="detailLoading" class="drawer-loading">
            <t-loading size="medium" />
          </div>

          <div v-else-if="detailData" class="drawer-body">
            <!-- 基本信息 -->
            <div class="detail-section">
              <div class="detail-section-title">{{ $t('admin.tenants.basicInfo') }}</div>
              <div class="detail-row">
                <span class="detail-label">{{ $t('admin.tenants.name') }}</span>
                <span class="detail-value">{{ detailData.name }}</span>
              </div>
              <div class="detail-row">
                <span class="detail-label">{{ $t('admin.tenants.description') }}</span>
                <span class="detail-value">{{ detailData.description || '-' }}</span>
              </div>
              <div class="detail-row">
                <span class="detail-label">{{ $t('admin.tenants.plan') }}</span>
                <span class="detail-value">{{ detailData.plan_name }}</span>
              </div>
              <div class="detail-row">
                <span class="detail-label">{{ $t('admin.tenants.status') }}</span>
                <t-tag :theme="statusTheme(detailData.status)" size="small" variant="light">
                  {{ statusLabel(detailData.status) }}
                </t-tag>
              </div>
              <div class="detail-row">
                <span class="detail-label">{{ $t('admin.tenants.createdAt') }}</span>
                <span class="detail-value">{{ formatDate(detailData.created_at) }}</span>
              </div>
            </div>

            <!-- 用量统计 -->
            <div class="detail-section">
              <div class="detail-section-title">{{ $t('admin.tenants.usageStats') }}</div>
              <div class="usage-grid">
                <div class="usage-item">
                  <span class="usage-label">{{ $t('admin.tenants.users') }}</span>
                  <span class="usage-value">{{ detailData.stats?.user_count ?? detailData.users?.length ?? 0 }}</span>
                </div>
                <div class="usage-item">
                  <span class="usage-label">{{ $t('admin.tenants.knowledgeBases') }}</span>
                  <span class="usage-value">{{ detailData.stats?.knowledge_base_count ?? 0 }}</span>
                </div>
                <div class="usage-item">
                  <span class="usage-label">{{ $t('admin.tenants.storage') }}</span>
                  <span class="usage-value">{{ formatBytes(detailData.stats?.storage_used_bytes ?? 0) }}</span>
                </div>
                <div class="usage-item">
                  <span class="usage-label">{{ $t('admin.tenants.tokenMonthly') }}</span>
                  <span class="usage-value">{{ formatTokens(detailData.stats?.token_used_monthly ?? 0) }}</span>
                </div>
              </div>
            </div>

            <!-- 操作 -->
            <div class="detail-section">
              <div class="detail-section-title">{{ $t('admin.tenants.actions') }}</div>
              <div class="detail-actions">
                <t-button
                  v-if="detailData.status !== 'suspended'"
                  theme="danger"
                  variant="outline"
                  size="small"
                  @click="handleSetStatus('suspended')"
                >
                  {{ $t('admin.tenants.suspend') }}
                </t-button>
                <t-button
                  v-else
                  theme="success"
                  variant="outline"
                  size="small"
                  @click="handleSetStatus('active')"
                >
                  {{ $t('admin.tenants.activate') }}
                </t-button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 编辑弹窗 -->
    <t-dialog
      v-model:visible="editDialogVisible"
      :header="$t('admin.tenants.editTenant')"
      :confirm-on-enter="false"
      :on-confirm="handleEditConfirm"
      width="480px"
    >
      <div class="edit-form">
        <div class="form-item">
          <label class="form-label">{{ $t('admin.tenants.name') }}</label>
          <t-input v-model="editForm.name" :placeholder="$t('admin.tenants.namePlaceholder')" />
        </div>
        <div class="form-item">
          <label class="form-label">{{ $t('admin.tenants.description') }}</label>
          <t-textarea v-model="editForm.description" :placeholder="$t('admin.tenants.descriptionPlaceholder')" :autosize="{ minRows: 2, maxRows: 4 }" />
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { listTenants, getTenant, updateTenant, setTenantStatus } from '@/api/admin'
import type { AdminTenantListItem, AdminTenantDetail } from '@/api/admin'
import { formatStringDate } from '@/utils/index'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const tenants = ref<AdminTenantListItem[]>([])
const keyword = ref('')
const statusFilter = ref('')
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showPageSize: false,
  showJumper: false,
})

const columns = [
  { colKey: 'name', title: t('admin.tenants.name'), minWidth: 200 },
  { colKey: 'plan_name', title: t('admin.tenants.plan'), width: 120 },
  { colKey: 'status', title: t('admin.tenants.status'), width: 100 },
  { colKey: 'user_count', title: t('admin.tenants.users'), width: 80 },
  { colKey: 'knowledge_base_count', title: t('admin.tenants.knowledgeBases'), width: 100 },
  { colKey: 'storage_used_bytes', title: t('admin.tenants.storage'), width: 120 },
  { colKey: 'token_used_monthly', title: t('admin.tenants.tokenMonthly'), width: 120 },
  { colKey: 'created_at', title: t('admin.tenants.createdAt'), width: 140 },
  { colKey: 'op', title: t('common.actions'), width: 140, fixed: 'right' as const },
]

// Detail drawer
const detailVisible = ref(false)
const detailTenant = ref<AdminTenantListItem | null>(null)
const detailLoading = ref(false)
const detailData = ref<AdminTenantDetail | null>(null)

// Edit dialog
const editDialogVisible = ref(false)
const editForm = reactive({ name: '', description: '', tenantId: 0 })

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debouncedFetch() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    page.value = 1
    pagination.current = 1
    fetchTenants()
  }, 400)
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function formatTokens(n: number): string {
  if (n === 0) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  return formatStringDate(new Date(dateStr))
}

function statusTheme(status: string): string {
  switch (status) {
    case 'active': return 'success'
    case 'suspended': return 'danger'
    case 'trial': return 'warning'
    default: return 'default'
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'active': return t('admin.tenants.statusActive')
    case 'suspended': return t('admin.tenants.statusSuspended')
    case 'trial': return t('admin.tenants.statusTrial')
    default: return status
  }
}

async function fetchTenants() {
  loading.value = true
  try {
    const res = await listTenants({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      status: statusFilter.value || undefined,
    })
    tenants.value = res.data || []
    total.value = res.total || 0
    pagination.total = total.value
    pagination.current = page.value
  } catch (e) {
    console.error('Failed to fetch tenants', e)
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: { current: number; pageSize: number }) {
  page.value = pageInfo.current
  pageSize.value = pageInfo.pageSize
  pagination.current = pageInfo.current
  fetchTenants()
}

function handleRowClick(row: AdminTenantListItem) {
  openDetail(row)
}

async function openDetail(row: AdminTenantListItem) {
  detailTenant.value = row
  detailVisible.value = true
  detailLoading.value = true
  detailData.value = null
  try {
    const res = await getTenant(row.tenant_id)
    if (res.success && res.data) {
      detailData.value = res.data
    }
  } catch (e) {
    MessagePlugin.error(t('admin.tenants.loadDetailFailed'))
  } finally {
    detailLoading.value = false
  }
}

function closeDetail() {
  detailVisible.value = false
  detailTenant.value = null
  detailData.value = null
}

function openEditDialog(row: AdminTenantListItem) {
  editForm.name = row.name
  editForm.description = row.description
  editForm.tenantId = row.tenant_id
  editDialogVisible.value = true
}

async function handleEditConfirm() {
  try {
    await updateTenant(editForm.tenantId, {
      name: editForm.name || undefined,
      description: editForm.description || undefined,
    })
    MessagePlugin.success(t('admin.tenants.updateSuccess'))
    editDialogVisible.value = false
    fetchTenants()
    // Refresh detail if open
    if (detailVisible.value && detailTenant.value?.tenant_id === editForm.tenantId) {
      const res = await getTenant(editForm.tenantId)
      if (res.success && res.data) detailData.value = res.data
    }
  } catch (e) {
    MessagePlugin.error(t('admin.tenants.updateFailed'))
  }
}

async function handleSetStatus(status: string) {
  if (!detailTenant.value) return
  try {
    await setTenantStatus(detailTenant.value.tenant_id, status)
    MessagePlugin.success(t('admin.tenants.statusUpdateSuccess'))
    fetchTenants()
    // Refresh detail
    const res = await getTenant(detailTenant.value.tenant_id)
    if (res.success && res.data) detailData.value = res.data
  } catch (e) {
    MessagePlugin.error(t('admin.tenants.statusUpdateFailed'))
  }
}

onMounted(() => {
  fetchTenants()
})
</script>

<style scoped lang="less">
.admin-tenants-container {
  height: calc(100vh);
  box-sizing: border-box;
  flex: 1;
  display: flex;
  position: relative;
  min-height: 0;
}

.tenants-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 24px 32px 0 32px;
}

.tenants-main {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 12px 0 24px;
}

.header-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.title-row h2 {
  font-size: 20px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0;
}

.back-btn {
  color: var(--td-text-color-secondary);
  &:hover { color: var(--td-text-color-primary); }
}

.header-subtitle {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin: 0;
}

.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.table-skeleton {
  padding: 8px 0;
}

.tenant-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tenant-name {
  font-weight: 500;
  color: var(--td-text-color-primary);
}

/* Detail drawer */
.detail-drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  justify-content: flex-end;
}

.detail-drawer {
  width: 480px;
  max-width: 90vw;
  height: 100%;
  background: var(--td-bg-color-container);
  box-shadow: -4px 0 16px rgba(0, 0, 0, 0.08);
  display: flex;
  flex-direction: column;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.drawer-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0;
}

.drawer-close {
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  &:hover { background: var(--td-bg-color-container-hover); }
}

.drawer-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
}

.drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}

.detail-section {
  margin-bottom: 24px;
}

.detail-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.detail-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 8px 0;
  gap: 16px;
}

.detail-label {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  flex-shrink: 0;
  min-width: 80px;
}

.detail-value {
  font-size: 13px;
  color: var(--td-text-color-primary);
  text-align: right;
  word-break: break-all;
}

.usage-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.usage-item {
  background: var(--td-bg-color-page);
  border-radius: 6px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.usage-label {
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.usage-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.detail-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* Edit form */
.edit-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 13px;
  color: var(--td-text-color-primary);
  font-weight: 500;
}

/* Empty state */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
  gap: 12px;
}

.empty-img {
  width: 120px;
  height: 120px;
  opacity: 0.6;
}

.empty-txt {
  font-size: 14px;
  color: var(--td-text-color-secondary);
}

/* Drawer transition */
.drawer-enter-active,
.drawer-leave-active {
  transition: opacity 0.2s ease;
  .detail-drawer {
    transition: transform 0.25s ease;
  }
}
.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
  .detail-drawer {
    transform: translateX(100%);
  }
}
</style>
