<template>
  <div class="admin-dashboard-container">
    <div class="dashboard-content">
      <div class="header">
        <div class="header-title">
          <div class="title-row">
            <h2>{{ $t('admin.dashboard.title') }}</h2>
            <t-tooltip :content="$t('common.refresh')" placement="bottom">
              <t-button
                variant="text"
                theme="default"
                size="small"
                class="header-action-btn"
                :loading="loading"
                @click="fetchData"
              >
                <template #icon><t-icon name="refresh" size="16px" /></template>
              </t-button>
            </t-tooltip>
          </div>
          <p class="header-subtitle">{{ $t('admin.dashboard.subtitle') }}</p>
        </div>
      </div>

      <div class="dashboard-main">
        <!-- 骨架屏 -->
        <div v-if="loading && !overview" class="stats-grid">
          <div v-for="n in 6" :key="'skel-' + n" class="stat-card">
            <t-skeleton
              animation="gradient"
              :row-col="[
                { width: '40%', height: '14px' },
                { width: '60%', height: '28px' },
              ]"
            />
          </div>
        </div>

        <!-- 统计卡片 -->
        <div v-else-if="overview" class="stats-grid">
          <div class="stat-card">
            <div class="stat-label">{{ $t('admin.dashboard.totalTenants') }}</div>
            <div class="stat-value">{{ overview.total_tenants }}</div>
            <div class="stat-extra">
              <span class="stat-badge active">{{ overview.active_tenants }} {{ $t('admin.dashboard.active') }}</span>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-label">{{ $t('admin.dashboard.totalUsers') }}</div>
            <div class="stat-value">{{ overview.total_users }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">{{ $t('admin.dashboard.totalKnowledgeBases') }}</div>
            <div class="stat-value">{{ overview.total_knowledge_bases }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">{{ $t('admin.dashboard.totalStorage') }}</div>
            <div class="stat-value">{{ formatBytes(overview.total_storage_used_bytes) }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">{{ $t('admin.dashboard.totalTokenUsage') }}</div>
            <div class="stat-value">{{ formatTokens(overview.total_token_used_all_time) }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">{{ $t('admin.dashboard.trials') }}</div>
            <div class="stat-value">{{ overview.trials_active }}</div>
            <div class="stat-extra">
              <span class="stat-badge converted">{{ overview.trials_converted }} {{ $t('admin.dashboard.converted') }}</span>
            </div>
          </div>
        </div>

        <!-- 租户用量排行 -->
        <div class="section-block">
          <div class="section-header">
            <h3 class="section-title">{{ $t('admin.dashboard.tenantUsageRanking') }}</h3>
            <t-button variant="text" size="small" @click="router.push('/platform/admin/tenants')">
              {{ $t('admin.dashboard.viewAll') }}
              <template #icon><t-icon name="chevron-right" size="14px" /></template>
            </t-button>
          </div>

          <!-- 表格骨架屏 -->
          <div v-if="tenantStatsLoading && tenantStats.length === 0">
            <t-skeleton
              v-for="n in 5"
              :key="'tbl-skel-' + n"
              animation="gradient"
              :row-col="[{ width: '100%', height: '40px' }]"
              style="margin-bottom: 8px"
            />
          </div>

          <!-- 租户统计表格 -->
          <t-table
            v-else
            :data="tenantStats"
            :columns="tenantColumns"
            row-key="tenant_id"
            size="small"
            :pagination="null"
            :hover="true"
            :loading="tenantStatsLoading"
          >
            <template #is_active="{ row }">
              <t-tag :theme="row.is_active ? 'success' : 'default'" size="small" variant="light">
                {{ row.is_active ? $t('admin.tenants.statusActive') : $t('admin.tenants.statusInactive') }}
              </t-tag>
            </template>
            <template #storage_used_bytes="{ row }">
              {{ formatBytes(row.storage_used_bytes) }}
            </template>
            <template #token_usage="{ row }">
              <span>{{ formatTokens(row.token_used) }} / {{ formatTokens(row.token_quota_monthly) }}</span>
            </template>
            <template #created_at="{ row }">
              {{ formatDate(row.created_at) }}
            </template>
          </t-table>
        </div>

        <!-- 空状态 -->
        <div v-if="!loading && !overview" class="empty-state">
          <img class="empty-img" src="@/assets/img/upload.svg" alt="" />
          <span class="empty-txt">{{ $t('admin.dashboard.loadFailed') }}</span>
          <t-button theme="primary" variant="outline" size="small" @click="fetchData">
            {{ $t('common.retry') }}
          </t-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getAdminOverview, getAdminTenantStats } from '@/api/admin'
import type { AdminOverviewStats, AdminTenantStatsItem } from '@/api/admin'
import { formatStringDate } from '@/utils/index'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const overview = ref<AdminOverviewStats | null>(null)
const tenantStatsLoading = ref(false)
const tenantStats = ref<AdminTenantStatsItem[]>([])

const tenantColumns = [
  { colKey: 'tenant_name', title: t('admin.tenants.name'), width: 200 },
  { colKey: 'plan_name', title: t('admin.tenants.plan'), width: 120 },
  { colKey: 'user_count', title: t('admin.tenants.users'), width: 80 },
  { colKey: 'storage_used_bytes', title: t('admin.tenants.storage'), width: 120 },
  { colKey: 'token_usage', title: t('admin.tenants.tokenUsage'), width: 160 },
  { colKey: 'is_active', title: t('admin.tenants.status'), width: 100 },
  { colKey: 'created_at', title: t('admin.tenants.createdAt'), width: 140 },
]

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

async function fetchOverview() {
  loading.value = true
  try {
    const res = await getAdminOverview()
    if (res.success && res.data) {
      overview.value = res.data
    }
  } catch (e) {
    console.error('Failed to fetch admin overview', e)
  } finally {
    loading.value = false
  }
}

async function fetchTenantStats() {
  tenantStatsLoading.value = true
  try {
    const res = await getAdminTenantStats({ page: 1, page_size: 10, sort_by: 'token_used', order: 'desc' })
    if (res.data) {
      tenantStats.value = res.data
    }
  } catch (e) {
    console.error('Failed to fetch tenant stats', e)
  } finally {
    tenantStatsLoading.value = false
  }
}

function fetchData() {
  fetchOverview()
  fetchTenantStats()
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped lang="less">
.admin-dashboard-container {
  height: calc(100vh);
  box-sizing: border-box;
  flex: 1;
  display: flex;
  position: relative;
  min-height: 0;
}

.dashboard-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 24px 32px 0 32px;
}

.dashboard-main {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 12px 0 24px;
}

.header {
  flex-shrink: 0;
}

.header-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.title-row h2 {
  font-size: 20px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0;
}

.header-subtitle {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin: 0;
}

.header-action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 6px;
  color: var(--td-text-color-secondary);
  transition: all 0.2s;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }
}

/* 统计卡片网格 */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: box-shadow 0.2s;

  &:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  }
}

.stat-label {
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--td-text-color-primary);
  line-height: 1.2;
}

.stat-extra {
  margin-top: 4px;
}

.stat-badge {
  display: inline-flex;
  align-items: center;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 10px;
  font-weight: 500;

  &.active {
    background: rgba(7, 192, 95, 0.1);
    color: var(--td-brand-color);
  }

  &.converted {
    background: rgba(0, 82, 217, 0.08);
    color: var(--td-brand-color);
  }
}

/* 区块 */
.section-block {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0;
}

/* 空状态 */
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
</style>
