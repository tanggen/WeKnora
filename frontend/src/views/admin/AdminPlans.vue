<template>
  <div class="admin-plans-container">
    <div class="plans-content">
      <div class="header">
        <div class="header-title">
          <div class="title-row">
            <t-button variant="text" size="small" class="back-btn" @click="router.push('/platform/admin')">
              <template #icon><t-icon name="chevron-left" size="18px" /></template>
            </t-button>
            <h2>{{ $t('admin.plans.title') }}</h2>
            <t-button theme="primary" size="small" @click="openCreateDialog">
              <template #icon><t-icon name="add" /></template>
              {{ $t('admin.plans.createPlan') }}
            </t-button>
          </div>
          <p class="header-subtitle">{{ $t('admin.plans.subtitle') }}</p>
        </div>
      </div>

      <div class="plans-main">
        <!-- 骨架屏 -->
        <div v-if="loading && plans.length === 0" class="plans-grid">
          <div v-for="n in 4" :key="'skel-' + n" class="plan-card plan-card-skeleton">
            <t-skeleton
              animation="gradient"
              :row-col="[
                { width: '50%', height: '20px' },
                { width: '100%', height: '14px' },
                { width: '80%', height: '14px' },
              ]"
            />
          </div>
        </div>

        <!-- 套餐卡片网格 -->
        <div v-else class="plans-grid">
          <div
            v-for="plan in sortedPlans"
            :key="plan.plan_id"
            class="plan-card"
            :class="{ 'is-inactive': !plan.is_active, 'is-trial': plan.is_trial }"
          >
            <div class="plan-card-header">
              <div class="plan-title-row">
                <span class="plan-name">{{ plan.name }}</span>
                <t-tag v-if="plan.is_trial" theme="warning" size="small" variant="light">
                  {{ $t('admin.plans.trial') }}
                </t-tag>
                <t-tag :theme="plan.is_active ? 'success' : 'default'" size="small" variant="light">
                  {{ plan.is_active ? $t('admin.plans.active') : $t('admin.plans.inactive') }}
                </t-tag>
              </div>
              <p class="plan-description">{{ plan.description || '-' }}</p>
            </div>

            <div class="plan-card-body">
              <div class="quota-grid">
                <div class="quota-item">
                  <span class="quota-label">{{ $t('admin.plans.maxUsers') }}</span>
                  <span class="quota-value">{{ plan.config.max_users || $t('admin.plans.unlimited') }}</span>
                </div>
                <div class="quota-item">
                  <span class="quota-label">{{ $t('admin.plans.maxKnowledgeBases') }}</span>
                  <span class="quota-value">{{ plan.config.max_knowledge_bases || $t('admin.plans.unlimited') }}</span>
                </div>
                <div class="quota-item">
                  <span class="quota-label">{{ $t('admin.plans.maxAgents') }}</span>
                  <span class="quota-value">{{ plan.config.max_agents || $t('admin.plans.unlimited') }}</span>
                </div>
                <div class="quota-item">
                  <span class="quota-label">{{ $t('admin.plans.storageQuota') }}</span>
                  <span class="quota-value">{{ formatBytes(plan.config.storage_quota_bytes) }}</span>
                </div>
                <div class="quota-item">
                  <span class="quota-label">{{ $t('admin.plans.tokenQuotaMonthly') }}</span>
                  <span class="quota-value">{{ formatTokens(plan.config.token_quota_monthly) }}</span>
                </div>
                <div class="quota-item" v-if="plan.is_trial">
                  <span class="quota-label">{{ $t('admin.plans.trialDays') }}</span>
                  <span class="quota-value">{{ plan.trial_days }}</span>
                </div>
              </div>
            </div>

            <div class="plan-card-footer">
              <t-space>
                <t-button variant="text" size="small" @click="openEditDialog(plan)">
                  <template #icon><t-icon name="edit" size="14px" /></template>
                  {{ $t('common.edit') }}
                </t-button>
                <t-button
                  variant="text"
                  size="small"
                  :theme="plan.is_active ? 'danger' : 'success'"
                  @click="handleToggleStatus(plan)"
                >
                  <template #icon><t-icon :name="plan.is_active ? 'poweroff' : 'check'" size="14px" /></template>
                  {{ plan.is_active ? $t('admin.plans.deactivate') : $t('admin.plans.activate') }}
                </t-button>
              </t-space>
            </div>
          </div>
        </div>

        <!-- 空状态 -->
        <div v-if="!loading && plans.length === 0" class="empty-state">
          <img class="empty-img" src="@/assets/img/upload.svg" alt="" />
          <span class="empty-txt">{{ $t('admin.plans.empty') }}</span>
          <t-button theme="primary" size="small" @click="openCreateDialog">
            {{ $t('admin.plans.createPlan') }}
          </t-button>
        </div>
      </div>
    </div>

    <!-- 创建/编辑套餐弹窗 -->
    <t-dialog
      v-model:visible="formDialogVisible"
      :header="formMode === 'create' ? $t('admin.plans.createPlan') : $t('admin.plans.editPlan')"
      :confirm-on-enter="false"
      :on-confirm="handleFormConfirm"
      width="600px"
    >
      <div class="plan-form">
        <div class="form-row">
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.planId') }} *</label>
            <t-input v-model="form.plan_id" :disabled="formMode === 'edit'" placeholder="e.g. basic, pro" />
          </div>
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.planName') }} *</label>
            <t-input v-model="form.name" :placeholder="$t('admin.plans.planNamePlaceholder')" />
          </div>
        </div>
        <div class="form-item">
          <label class="form-label">{{ $t('admin.plans.description') }}</label>
          <t-input v-model="form.description" :placeholder="$t('admin.plans.descriptionPlaceholder')" />
        </div>
        <div class="form-row">
          <div class="form-item half">
            <t-checkbox v-model="form.is_trial">{{ $t('admin.plans.isTrial') }}</t-checkbox>
          </div>
          <div v-if="form.is_trial" class="form-item half">
            <label class="form-label">{{ $t('admin.plans.trialDays') }}</label>
            <t-input-number v-model="form.trial_days" :min="1" :max="365" theme="normal" />
          </div>
        </div>

        <div class="form-section-title">{{ $t('admin.plans.quotaSettings') }}</div>
        <div class="form-row">
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.maxUsers') }}</label>
            <t-input-number v-model="form.max_users" :min="0" theme="normal" />
          </div>
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.maxKnowledgeBases') }}</label>
            <t-input-number v-model="form.max_knowledge_bases" :min="0" theme="normal" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.maxAgents') }}</label>
            <t-input-number v-model="form.max_agents" :min="0" theme="normal" />
          </div>
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.storageQuota') }} (bytes)</label>
            <t-input-number v-model="form.storage_quota_bytes" :min="0" theme="normal" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.tokenQuotaMonthly') }}</label>
            <t-input-number v-model="form.token_quota_monthly" :min="0" theme="normal" />
          </div>
          <div class="form-item half">
            <label class="form-label">{{ $t('admin.plans.sortOrder') }}</label>
            <t-input-number v-model="form.sort_order" :min="0" theme="normal" />
          </div>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { listAllPlans, createPlan, updatePlan, setPlanStatus } from '@/api/admin'
import type { PlanWithConfig, CreatePlanRequest } from '@/api/admin'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const plans = ref<PlanWithConfig[]>([])

const sortedPlans = computed(() => {
  return [...plans.value].sort((a, b) => (a.sort_order ?? 99) - (b.sort_order ?? 99))
})

// Form dialog
const formDialogVisible = ref(false)
const formMode = ref<'create' | 'edit'>('create')
const editingPlanId = ref('')
const form = reactive({
  plan_id: '',
  name: '',
  description: '',
  is_trial: false,
  trial_days: 14,
  max_users: 0,
  max_knowledge_bases: 0,
  max_agents: 0,
  storage_quota_bytes: 0,
  token_quota_monthly: 0,
  sort_order: 0,
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function formatTokens(n: number): string {
  if (!n || n === 0) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function resetForm() {
  form.plan_id = ''
  form.name = ''
  form.description = ''
  form.is_trial = false
  form.trial_days = 14
  form.max_users = 0
  form.max_knowledge_bases = 0
  form.max_agents = 0
  form.storage_quota_bytes = 0
  form.token_quota_monthly = 0
  form.sort_order = 0
}

function openCreateDialog() {
  resetForm()
  formMode.value = 'create'
  editingPlanId.value = ''
  formDialogVisible.value = true
}

function openEditDialog(plan: PlanWithConfig) {
  formMode.value = 'edit'
  editingPlanId.value = plan.plan_id
  form.plan_id = plan.plan_id
  form.name = plan.name
  form.description = plan.description
  form.is_trial = plan.is_trial
  form.trial_days = plan.trial_days
  form.max_users = plan.config.max_users
  form.max_knowledge_bases = plan.config.max_knowledge_bases
  form.max_agents = plan.config.max_agents
  form.storage_quota_bytes = plan.config.storage_quota_bytes
  form.token_quota_monthly = plan.config.token_quota_monthly
  form.sort_order = plan.sort_order
  formDialogVisible.value = true
}

async function fetchPlans() {
  loading.value = true
  try {
    const res = await listAllPlans()
    if (res.success && res.data) {
      plans.value = res.data
    }
  } catch (e) {
    console.error('Failed to fetch plans', e)
  } finally {
    loading.value = false
  }
}

async function handleFormConfirm() {
  if (!form.plan_id || !form.name) {
    MessagePlugin.warning(t('admin.plans.requiredFields'))
    return
  }
  const data = {
    plan_id: form.plan_id,
    name: form.name,
    description: form.description || undefined,
    is_trial: form.is_trial,
    trial_days: form.trial_days,
    max_users: form.max_users,
    max_knowledge_bases: form.max_knowledge_bases,
    max_agents: form.max_agents,
    storage_quota_bytes: form.storage_quota_bytes,
    token_quota_monthly: form.token_quota_monthly,
    sort_order: form.sort_order,
  }

  try {
    if (formMode.value === 'create') {
      await createPlan(data as CreatePlanRequest)
      MessagePlugin.success(t('admin.plans.createSuccess'))
    } else {
      const { plan_id, ...updateData } = data
      await updatePlan(editingPlanId.value, updateData)
      MessagePlugin.success(t('admin.plans.updateSuccess'))
    }
    formDialogVisible.value = false
    fetchPlans()
  } catch (e) {
    MessagePlugin.error(formMode.value === 'create' ? t('admin.plans.createFailed') : t('admin.plans.updateFailed'))
  }
}

async function handleToggleStatus(plan: PlanWithConfig) {
  try {
    await setPlanStatus(plan.plan_id, !plan.is_active)
    MessagePlugin.success(plan.is_active ? t('admin.plans.deactivateSuccess') : t('admin.plans.activateSuccess'))
    fetchPlans()
  } catch (e) {
    MessagePlugin.error(t('admin.plans.statusUpdateFailed'))
  }
}

onMounted(() => {
  fetchPlans()
})
</script>

<style scoped lang="less">
.admin-plans-container {
  height: calc(100vh);
  box-sizing: border-box;
  flex: 1;
  display: flex;
  position: relative;
  min-height: 0;
}

.plans-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 24px 32px 0 32px;
}

.plans-main {
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
  gap: 8px;
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

/* 套餐卡片网格 */
.plans-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.plan-card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  transition: box-shadow 0.2s;

  &:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  }

  &.is-inactive {
    opacity: 0.6;
  }

  &.is-trial {
    border-left: 3px solid var(--td-warning-color);
  }
}

.plan-card-skeleton {
  min-height: 180px;
}

.plan-card-header {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.plan-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.plan-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.plan-description {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin: 0;
}

.plan-card-body {
  flex: 1;
}

.quota-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.quota-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.quota-label {
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.quota-value {
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.plan-card-footer {
  border-top: 1px solid var(--td-component-stroke);
  padding-top: 12px;
}

/* Form */
.plan-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-row {
  display: flex;
  gap: 16px;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;

  &.half {
    flex: 0 0 calc(50% - 8px);
  }
}

.form-label {
  font-size: 13px;
  color: var(--td-text-color-primary);
  font-weight: 500;
}

.form-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  padding-top: 8px;
  border-top: 1px solid var(--td-component-stroke);
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
</style>
