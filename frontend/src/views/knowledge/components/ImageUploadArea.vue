<script setup lang="ts">
import { ref, computed } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { uploadKnowledgeImage } from '@/api/knowledge-base'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  kbId: string
  tagId?: string
}>()

const emit = defineEmits<{
  (e: 'uploaded', kbId: string): void
}>()

const { t } = useI18n()

const SUPPORTED_FORMATS = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp']
const MAX_FILE_SIZE = 20 * 1024 * 1024 // 20MB
const MAX_UPLOAD_COUNT = 20

const isDragging = ref(false)
const uploadingFiles = ref<Map<string, { file: File; progress: number }>>(new Map())
const uploadingCount = ref(0)
const fileInput = ref<HTMLInputElement | null>(null)

const isUploading = computed(() => uploadingCount.value > 0)

const validateFile = (file: File): string | null => {
  const ext = file.name.substring(file.name.lastIndexOf('.') + 1).toLowerCase()
  if (!SUPPORTED_FORMATS.includes(ext)) {
    return t('knowledgeBase.image.unsupportedFormat') as string
  }
  if (file.size > MAX_FILE_SIZE) {
    return t('knowledgeBase.image.fileTooLarge') as string
  }
  return null
}

const processFiles = (files: FileList) => {
  if (!props.kbId) {
    MessagePlugin.error('Knowledge base ID is required')
    return
  }

  const validFiles: File[] = []
  const errors: string[] = []

  for (let i = 0; i < files.length; i++) {
    const file = files[i]
    const error = validateFile(file)
    if (error) {
      errors.push(`${file.name}: ${error}`)
    } else {
      validFiles.push(file)
    }
  }

  if (validFiles.length + uploadingCount.value > MAX_UPLOAD_COUNT) {
    MessagePlugin.warning(t('chat.imageTooMany') as string)
    return
  }

  if (errors.length > 0) {
    errors.forEach(msg => MessagePlugin.warning(msg))
  }

  if (validFiles.length > 0) {
    uploadFiles(validFiles)
  }
}

const uploadFiles = async (files: File[]) => {
  const tagId = props.tagId || undefined

  for (const file of files) {
    const key = `${file.name}_${Date.now()}_${Math.random()}`
    uploadingFiles.value.set(key, { file, progress: 0 })
    uploadingCount.value++

    try {
      const data: any = { file, tag_id: tagId }
      await uploadKnowledgeFileWithProgress(file, key, tagId)
      uploadingFiles.value.delete(key)
      uploadingCount.value--
    } catch (e: any) {
      uploadingFiles.value.delete(key)
      uploadingCount.value--
      MessagePlugin.error(e?.message || (t('knowledgeBase.image.uploadFailed') as string))
    }
  }

  // Emit uploaded event so parent can refresh
  emit('uploaded', props.kbId)

  // Reset file input
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

const uploadKnowledgeFileWithProgress = (file: File, key: string, tagId?: string): Promise<void> => {
  return new Promise((resolve, reject) => {
    const data: any = { file }
    if (tagId) data.tag_id = tagId

    uploadKnowledgeImage(
      props.kbId,
      data,
      (progressEvent: any) => {
        if (progressEvent?.total) {
          const pct = Math.round((progressEvent.loaded / progressEvent.total) * 100)
          const entry = uploadingFiles.value.get(key)
          if (entry) {
            entry.progress = pct
          }
        }
      }
    )
      .then((responseData: any) => {
        const isSuccess = responseData?.success || responseData?.code === 200 || responseData?.status === 'success' || (!responseData?.error && responseData)
        if (isSuccess) {
          resolve()
        } else {
          reject(new Error(responseData?.error?.message || responseData?.message || (t('knowledgeBase.image.uploadFailed') as string)))
        }
      })
      .catch(reject)
  })
}

const handleDragEnter = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  isDragging.value = true
}

const handleDragLeave = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  isDragging.value = false
}

const handleDragOver = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  isDragging.value = true
}

const handleDrop = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  isDragging.value = false

  const files = e.dataTransfer?.files
  if (files && files.length > 0) {
    processFiles(files)
  }
}

const handleClick = () => {
  fileInput.value?.click()
}

const handleFileChange = (e: Event) => {
  const input = e.target as HTMLInputElement
  const files = input?.files
  if (files && files.length > 0) {
    processFiles(files)
  }
}
</script>

<template>
  <div class="image-upload-area">
    <!-- Drop zone -->
    <div
      class="upload-drop-zone"
      :class="{ dragging: isDragging, uploading: isUploading }"
      @dragenter="handleDragEnter"
      @dragleave="handleDragLeave"
      @dragover="handleDragOver"
      @drop="handleDrop"
      @click="handleClick"
    >
      <input
        ref="fileInput"
        type="file"
        class="upload-input-hidden"
        accept=".jpg,.jpeg,.png,.gif,.webp,.bmp"
        multiple
        @change="handleFileChange"
      />

      <div class="upload-placeholder">
        <t-icon name="image-add" size="36px" class="upload-icon" />
        <p class="upload-title">{{ $t('knowledgeBase.image.uploadTitle') }}</p>
        <p class="upload-desc">{{ $t('knowledgeBase.image.dropHint') }}</p>
        <p class="upload-hint">{{ $t('knowledgeBase.image.uploadHint') }}</p>
      </div>
    </div>

    <!-- Upload progress list -->
    <div v-if="uploadingFiles.size > 0" class="upload-progress-list">
      <div
        v-for="(entry, key) in uploadingFiles"
        :key="key"
        class="upload-progress-item"
      >
        <div class="progress-file-info">
          <t-icon name="image" size="16px" class="progress-file-icon" />
          <span class="progress-file-name">{{ entry.file.name }}</span>
        </div>
        <div class="progress-bar-wrapper">
          <div class="progress-bar">
            <div
              class="progress-bar-fill"
              :style="{ width: entry.progress + '%' }"
            ></div>
          </div>
          <span class="progress-bar-text">{{ entry.progress }}%</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.image-upload-area {
  width: 100%;
  margin-bottom: 16px;
}

.upload-drop-zone {
  border: 2px dashed var(--td-component-stroke);
  border-radius: 10px;
  padding: 32px 24px;
  text-align: center;
  cursor: pointer;
  transition: all 0.25s ease;
  background: var(--td-bg-color-container);
  user-select: none;

  &:hover {
    border-color: var(--td-brand-color-5);
    background: rgba(var(--td-brand-color-rgb, 0, 82, 217), 0.04);
  }

  &.dragging {
    border-color: var(--td-brand-color);
    background: rgba(var(--td-brand-color-rgb, 0, 82, 217), 0.08);
    box-shadow: 0 0 0 4px rgba(var(--td-brand-color-rgb, 0, 82, 217), 0.12);
  }

  &.uploading {
    opacity: 0.7;
    pointer-events: none;
  }
}

.upload-input-hidden {
  display: none;
}

.upload-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.upload-icon {
  color: var(--td-brand-color);
  opacity: 0.7;
}

.upload-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0;
}

.upload-desc {
  font-size: 14px;
  color: var(--td-text-color-secondary);
  margin: 0;
}

.upload-hint {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  margin: 0;
}

.upload-progress-list {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 200px;
  overflow-y: auto;
}

.upload-progress-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 8px;
  gap: 12px;
}

.progress-file-info {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex-shrink: 1;
}

.progress-file-icon {
  color: var(--td-brand-color);
  flex-shrink: 0;
}

.progress-file-name {
  font-size: 13px;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.progress-bar-wrapper {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  width: 160px;
}

.progress-bar {
  flex: 1;
  height: 6px;
  background: var(--td-bg-color-component);
  border-radius: 3px;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  background: var(--td-brand-color);
  border-radius: 3px;
  transition: width 0.3s ease;
}

.progress-bar-text {
  font-size: 11px;
  color: var(--td-text-color-secondary);
  width: 36px;
  text-align: right;
  flex-shrink: 0;
}
</style>
