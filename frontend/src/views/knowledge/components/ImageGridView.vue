<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { delKnowledgeDetails, downKnowledgeDetails } from '@/api/knowledge-base'
import { formatStringDate } from '@/utils/index'
import { formatFileSize } from '@/utils/files'
import { useI18n } from 'vue-i18n'

interface ImageItem {
  id: string
  file_name?: string
  file_size?: number
  file_type?: string
  parse_status: string
  description?: string
  error_message?: string
  updated_at?: string
  created_at?: string
  tag_id?: string
  file_path?: string
}

const props = defineProps<{
  images: ImageItem[]
  loading: boolean
  canEdit: boolean
  kbId: string
}>()

const emit = defineEmits<{
  (e: 'delete', id: string): void
  (e: 'load-more'): void
  (e: 'refresh'): void
}>()

const { t } = useI18n()

// Thumbnail cache
const thumbnails = ref<Record<string, string>>({})
const thumbnailLoading = ref<Record<string, boolean>>({})
const failedThumbnails = ref<Set<string>>(new Set())

// Preview state
const previewVisible = ref(false)
const previewUrl = ref('')

const loadThumbnail = async (item: ImageItem) => {
  if (thumbnails.value[item.id] || thumbnailLoading.value[item.id] || failedThumbnails.value.has(item.id)) return

  thumbnailLoading.value[item.id] = true
  try {
    const res = await downKnowledgeDetails(item.id)
    if (res?.data instanceof Blob) {
      const url = URL.createObjectURL(res.data)
      thumbnails.value[item.id] = url
    }
  } catch {
    failedThumbnails.value.add(item.id)
  } finally {
    thumbnailLoading.value[item.id] = false
  }
}

// Intersection observer for lazy loading
let observer: IntersectionObserver | null = null
const cardRefs = ref<Record<string, HTMLElement>>({})

const setCardRef = (id: string) => (el: HTMLElement | null) => {
  if (el) {
    cardRefs.value[id] = el
    if (observer) observer.observe(el)
  }
}

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          const el = entry.target as HTMLElement
          const imageId = el.dataset.imageId
          if (imageId) {
            const item = props.images.find((img) => img.id === imageId)
            if (item) loadThumbnail(item)
          }
        }
      })
    },
    { rootMargin: '200px' }
  )

  // Observe existing cards
  props.images.forEach((item) => {
    const el = cardRefs.value[item.id]
    if (el) observer.observe(el)
  })
})

onUnmounted(() => {
  observer?.disconnect()
  // Clean up blob URLs
  Object.values(thumbnails.value).forEach((url) => URL.revokeObjectURL(url))
})

// Watch for new images to observe them
watch(
  () => props.images,
  (newImages) => {
    if (!observer) return
    newImages.forEach((item) => {
      const el = cardRefs.value[item.id]
      if (el && !thumbnails.value[item.id] && !failedThumbnails.value.has(item.id)) {
        observer.observe(el)
      }
    })
  },
  { deep: true }
)

// Delete
const deletingId = ref<string | null>(null)
const handleDelete = async (item: ImageItem) => {
  const confirmed = window.confirm(t('knowledgeBase.image.deleteConfirm') as string)
  if (!confirmed) return

  deletingId.value = item.id
  try {
    await delKnowledgeDetails(item.id)
    MessagePlugin.success(t('knowledgeBase.image.deleteSuccess'))
    // Clean up blob URL
    if (thumbnails.value[item.id]) {
      URL.revokeObjectURL(thumbnails.value[item.id])
      delete thumbnails.value[item.id]
    }
    emit('delete', item.id)
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('common.operationFailed'))
  } finally {
    deletingId.value = null
  }
}

// Preview
const handlePreview = async (item: ImageItem) => {
  if (thumbnails.value[item.id]) {
    previewUrl.value = thumbnails.value[item.id]
    previewVisible.value = true
  } else {
    // Load full image for preview
    try {
      const res = await downKnowledgeDetails(item.id)
      if (res?.data instanceof Blob) {
        const url = URL.createObjectURL(res.data)
        previewUrl.value = url
        previewVisible.value = true
        // Cache it
        if (!thumbnails.value[item.id]) {
          thumbnails.value[item.id] = url
        }
      }
    } catch {
      MessagePlugin.error(t('knowledgeBase.chunkLoadFailed'))
    }
  }
}

const closePreview = () => {
  previewVisible.value = false
  previewUrl.value = ''
}

const getStatusText = (status: string) => {
  switch (status) {
    case 'completed':
      return t('knowledgeBase.statusCompleted')
    case 'processing':
    case 'pending':
      return t('knowledgeBase.processing')
    case 'failed':
      return t('knowledgeBase.statusFailed')
    default:
      return status || '--'
  }
}

const getStatusTheme = (status: string) => {
  switch (status) {
    case 'completed':
      return 'success' as const
    case 'processing':
    case 'pending':
      return 'warning' as const
    case 'failed':
      return 'danger' as const
    default:
      return 'default' as const
  }
}

const formatTime = (time?: string) => {
  if (!time) return '--'
  const formatted = formatStringDate(new Date(time))
  return formatted.slice(2, 16)
}
</script>

<template>
  <div class="image-grid-container">
    <!-- Loading skeleton -->
    <div v-if="loading && images.length === 0" class="image-grid">
      <div v-for="n in 8" :key="'skel-' + n" class="image-card image-card-skeleton">
        <div class="image-thumbnail skeleton-thumbnail">
          <t-skeleton animation="gradient" :row-col="[{ width: '100%', height: '100%', type: 'rect' }]" />
        </div>
        <div class="image-card-info">
          <t-skeleton animation="gradient" :row-col="[{ width: '70%', height: '14px' }, { width: '50%', height: '12px' }]" />
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else-if="!loading && images.length === 0" class="empty-state">
      <t-icon name="image" size="48px" class="empty-icon" />
      <p class="empty-title">{{ $t('knowledgeBase.image.noImages') }}</p>
      <p class="empty-desc">{{ $t('knowledgeBase.image.noImagesDesc') }}</p>
    </div>

    <!-- Image grid -->
    <div v-else class="image-grid">
      <div
        v-for="item in images"
        :key="item.id"
        :ref="setCardRef(item.id)"
        :data-image-id="item.id"
        class="image-card"
        :class="{
          'status-processing': item.parse_status === 'processing' || item.parse_status === 'pending',
          'status-failed': item.parse_status === 'failed',
          'status-completed': item.parse_status === 'completed',
        }"
        @click="handlePreview(item)"
      >
        <!-- Thumbnail -->
        <div class="image-thumbnail">
          <img
            v-if="thumbnails[item.id]"
            :src="thumbnails[item.id]"
            :alt="item.file_name || ''"
            loading="lazy"
            class="thumbnail-img"
          />
          <div v-else-if="thumbnailLoading[item.id]" class="thumbnail-placeholder">
            <t-loading size="small" />
          </div>
          <div v-else class="thumbnail-placeholder">
            <t-icon name="image" size="28px" />
          </div>

          <!-- Status badge -->
          <div class="status-badge" :class="getStatusTheme(item.parse_status)">
            <t-icon v-if="item.parse_status === 'processing' || item.parse_status === 'pending'" name="loading" size="12px" class="spin-icon" />
            <t-icon v-else-if="item.parse_status === 'completed'" name="check-circle" size="12px" />
            <t-icon v-else-if="item.parse_status === 'failed'" name="close-circle" size="12px" />
            <span>{{ getStatusText(item.parse_status) }}</span>
          </div>
        </div>

        <!-- Card info bar -->
        <div class="image-card-info">
          <div class="card-name-row">
            <t-tooltip :content="item.file_name || item.id" placement="top">
              <span class="card-name">{{ item.file_name || item.id }}</span>
            </t-tooltip>
            <t-button
              v-if="canEdit"
              variant="text"
              theme="danger"
              size="small"
              class="delete-btn"
              :loading="deletingId === item.id"
              @click.stop="handleDelete(item)"
            >
              <t-icon name="delete" size="14px" />
            </t-button>
          </div>
          <div class="card-meta">
            <span v-if="item.file_size" class="file-size">{{ formatFileSize(item.file_size) }}</span>
            <span v-if="item.updated_at || item.created_at" class="file-time">{{ formatTime(item.updated_at || item.created_at) }}</span>
          </div>
          <div v-if="item.description" class="card-description" :title="item.description">
            {{ item.description }}
          </div>
          <div v-if="item.error_message" class="card-error" :title="item.error_message">
            <t-icon name="error-circle" size="12px" />
            <span>{{ item.error_message }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Preview -->
    <Teleport to="body">
      <t-image-viewer
        :visible="previewVisible"
        :images="previewUrl ? [previewUrl] : []"
        closeOnOverlay
        closeOnEscKeydown
        @close="closePreview"
      />
    </Teleport>
  </div>
</template>

<style scoped lang="less">
.image-grid-container {
  width: 100%;
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.image-card {
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  overflow: hidden;
  background: var(--td-bg-color-container);
  cursor: pointer;
  transition: all 0.2s ease;

  &:hover {
    border-color: var(--td-brand-color);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    transform: translateY(-2px);
  }

  &.status-failed {
    border-color: var(--td-error-color-3);
  }

  &.status-processing {
    opacity: 0.9;
  }
}

.image-card-skeleton {
  cursor: default;
  pointer-events: none;

  &:hover {
    transform: none;
    border-color: var(--td-component-stroke);
    box-shadow: none;
  }
}

.image-thumbnail {
  position: relative;
  width: 100%;
  height: 160px;
  background: var(--td-bg-color-secondarycontainer);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;

  .thumbnail-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .thumbnail-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--td-text-color-placeholder);
  }

  .skeleton-thumbnail {
    width: 100%;
    height: 100%;
  }
}

.status-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  line-height: 18px;
  backdrop-filter: blur(4px);

  &.success {
    background: rgba(0, 168, 112, 0.85);
    color: #fff;
  }

  &.warning {
    background: rgba(255, 152, 0, 0.85);
    color: #fff;
  }

  &.danger {
    background: rgba(227, 77, 89, 0.85);
    color: #fff;
  }

  &.default {
    background: rgba(0, 0, 0, 0.5);
    color: #fff;
  }

  .spin-icon {
    animation: spin 1s linear infinite;
  }
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.image-card-info {
  padding: 10px 12px;
}

.card-name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-bottom: 4px;
}

.card-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.delete-btn {
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.2s ease;

  .image-card:hover & {
    opacity: 1;
  }
}

.card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.card-description {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  margin-top: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 18px;
}

.card-error {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  margin-top: 6px;
  font-size: 11px;
  color: var(--td-error-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;

  .empty-icon {
    color: var(--td-text-color-placeholder);
    margin-bottom: 16px;
  }

  .empty-title {
    font-size: 16px;
    font-weight: 500;
    color: var(--td-text-color-secondary);
    margin: 0 0 8px;
  }

  .empty-desc {
    font-size: 13px;
    color: var(--td-text-color-placeholder);
    margin: 0;
  }
}
</style>
