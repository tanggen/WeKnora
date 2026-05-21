# Agent 开发卡片：图片库 — 前端 UI

> 模块编号：09-FRONTEND-01 | 优先级：P0 | 预估工作量：6-8h | 依赖：09-BACKEND-02

---

## 1. API 契约（前端消费视角）

### 1.1 创建图片库

```typescript
// frontend/src/api/knowledge-base/index.ts

export function createImageKnowledgeBase(data: {
  name: string;
  description?: string;
  type: 'image';
  vlm_config: { enabled: true; model_id: string };
  embedding_model_id: string;
  storage_provider_config?: { provider: string };
  tag_id?: string;
}): Promise<{ success: boolean; data: KnowledgeBase }>

// 等价于调用现有的 createKnowledgeBase，但 type 固定为 "image"
```

### 1.2 上传图片（单张）

```typescript
export function uploadImageToKB(
  kbId: string,
  data: { file: File; tag_id?: string; channel?: string },
  onProgress?: (progressEvent: any) => void
): Promise<{ success: boolean; data: Knowledge }>

// POST /api/v1/knowledge-bases/{kbId}/knowledge/image
// multipart/form-data
```

### 1.3 批量上传图片

```typescript
export function batchUploadImageToKB(
  kbId: string,
  data: { files: File[]; tag_id?: string; channel?: string },
  onProgress?: (progressEvent: any) => void
): Promise<{
  success: boolean;
  data: {
    success_count: number;
    failed: { file_name: string; reason: string }[];
    knowledges: Knowledge[];
  }
}>
```

### 1.4 获取图片库列表（复用现有）

```typescript
// 复用 listKnowledgeFiles(kbId, params)
// 对 image 类型 KB，返回数据中包含:
//   file_type: "jpg"|"png"|"gif"|"webp"|"bmp"
//   file_size: number
//   file_path: string  — provider:// 路径，用于拼装图片显示 URL
```

---

## 2. 数据库 DDL

**无（前端不涉及）**

---

## 3. 前端状态管理

### 3.1 KnowledgeBase.vue 新增状态

```typescript
// 新增 computed（与 isFAQ 并列）
const isImage = computed(() => (kbInfo.value?.type || '') === 'image');

// 扩展 validTabs
// image 类型 KB 只有一个 tab: 'images'
// 不需要 'documents', 'wiki', 'graph' tabs
const validTabs = computed(() => {
  if (isImage.value) return ['images'] as const;
  if (isFAQ.value) return ['faq'] as const;
  // document: 'documents', 'wiki'(if enabled), 'graph'(if enabled)
  ...
});

// 图片相关状态
const imageUploading = ref(false);
const selectedImageIds = ref<Set<string>>(new Set());
const imageViewMode = ref<'grid' | 'list'>('grid'); // 默认网格模式
```

### 3.2 图片预览 URL 解析

```typescript
// provider:// 路径需要转为 HTTP 可访问的 URL
function resolveImageUrl(filePath: string): string {
  // provider:// 格式
  if (filePath.startsWith('local://') || filePath.startsWith('minio://') || filePath.startsWith('cos://')) {
    return `/files/${filePath}`;
  }
  // 直接 HTTP URL
  if (filePath.startsWith('http://') || filePath.startsWith('https://')) {
    return filePath;
  }
  return `/files/${filePath}`;
}

// 缩略图 URL（P1）
function resolveThumbnailUrl(filePath: string): string {
  const ext = filePath.lastIndexOf('.');
  const base = filePath.substring(0, ext);
  return resolveImageUrl(`${base}_thumb.jpg`);
}
```

---

## 4. 业务规则汇总

### 4.1 KnowledgeBaseEditorModal.vue 改动

**位置**：类型选择区域（行 46-52）

```html
<!-- 当前代码 -->
<t-radio-button value="document">{{ $t('knowledgeEditor.basic.typeDocument') }}</t-radio-button>
<t-radio-button value="faq">{{ $t('knowledgeEditor.basic.typeFAQ') }}</t-radio-button>

<!-- 改为 -->
<t-radio-button value="document">{{ $t('knowledgeEditor.basic.typeDocument') }}</t-radio-button>
<t-radio-button value="faq">{{ $t('knowledgeEditor.basic.typeFAQ') }}</t-radio-button>
<t-radio-button value="image">{{ $t('knowledgeEditor.basic.typeImage') }}</t-radio-button>
```

**联动规则**：选择 `image` 时：
- 隐藏分块配置（`v-if="!isFAQ && !isImage"`）
- 隐藏 FAQ 配置（`v-if="isFAQ"` ➜ 自动隐藏）
- 隐藏 Wiki 配置（`v-if="!isFAQ"` ➜ `v-if="!isFAQ && !isImage"`）
- 强制显示 VLM 模型配置，且 `vlm_config.enabled` 自动设为 `true` 且不可关闭
- 索引策略：vector 默认勾选并锁定

### 4.2 KnowledgeBase.vue 改动

**Tab 结构**：

```html
<!-- 图片库使用独立 tab -->
<div v-if="isImage" class="image-tab-content">
  <ImageUploadZone
    :kb-id="kbId"
    :tag-id="activeTagId"
    :uploading="imageUploading"
    @upload="handleImageUpload"
  />
  <ImageGridView
    :images="imageList"
    :loading="docListLoading"
    :selected-ids="selectedImageIds"
    @select="handleImageSelect"
    @click="handleImageClick"
    @delete="handleImageDelete"
  />
</div>
```

### 4.3 ImageGridView.vue（新建组件）

**功能**：
- 网格布局（CSS Grid，4-5 列响应式）
- 每张图片显示：缩略图 + 文件名 + AI 描述摘要（单行截断）
- 支持多选（勾选框）
- 支持点击查看详情
- 空状态提示

**Props**：
```typescript
interface ImageGridProps {
  images: Knowledge[];        // 图片列表
  loading: boolean;           // 加载状态
  selectedIds?: Set<string>;  // 已选中的 ID 集合
  viewMode?: 'grid' | 'list'; // 视图模式
}
```

**Emits**：
```typescript
interface ImageGridEmits {
  (e: 'select', ids: Set<string>): void;
  (e: 'click', knowledge: Knowledge): void;
  (e: 'delete', ids: string[]): void;
}
```

### 4.4 ImageUploadZone.vue（新建组件）

**功能**：
- 拖拽上传区域（虚线边框 + 图标 + 提示文字）
- 支持点击选择文件
- 上传进度条
- 上传中禁用交互

**Props**：
```typescript
interface ImageUploadZoneProps {
  kbId: string;
  tagId?: string;
  uploading: boolean;
}
```

### 4.5 ImageDetailDrawer.vue（新建组件，P1）

**功能**：
- 侧边抽屉 Drawer
- 原图预览（可缩放）
- AI 描述展示
- OCR 文字展示（可复制）
- 图片元信息（文件名、大小、上传时间、分类）
- 删除按钮

### 4.6 国际化翻译

**zh-CN.ts 新增**：
```typescript
{
  knowledgeEditor: {
    basic: {
      typeLabel: '知识库类型',
      typeDocument: '文档库',
      typeFAQ: '问答库',
      typeImage: '图片库',           // 新增
      typeDescription: '文档库支持文件上传 / 图片库支持图片管理与搜索',
    },
    image: {                          // 全新 section
      title: '图片管理',
      upload: '上传图片',
      uploadHint: '支持 JPG、PNG、GIF、WebP、BMP 格式，单文件不超过 20MB',
      batchUpload: '批量上传',
      batchLimit: '单次最多上传 20 张',
      gridView: '网格视图',
      listView: '列表视图',
      noImages: '暂无图片，点击上方按钮上传',
      deleteConfirm: '确定要删除选中的 {count} 张图片吗？',
      ocrLabel: 'OCR 识別文字',
      captionLabel: 'AI 图片描述',
      infoLabel: '图片信息',
      loadingCaption: '正在生成描述...',
      loadingOCR: '正在识別文字...',
    },
  }
}
```

**en-US.ts 新增**：
```typescript
{
  knowledgeEditor: {
    basic: {
      typeImage: 'Image Library',
      typeDescription: 'Document for file-based knowledge / Image for visual content',
    },
    image: {
      title: 'Image Management',
      upload: 'Upload Image',
      uploadHint: 'Supports JPG, PNG, GIF, WebP, BMP. Max 20MB per file.',
      batchUpload: 'Batch Upload',
      batchLimit: 'Up to 20 images per batch',
      gridView: 'Grid View',
      listView: 'List View',
      noImages: 'No images yet. Click the button above to upload.',
      deleteConfirm: 'Are you sure you want to delete {count} selected images?',
      ocrLabel: 'OCR Text',
      captionLabel: 'AI Caption',
      infoLabel: 'Image Info',
      loadingCaption: 'Generating caption...',
      loadingOCR: 'Recognizing text...',
    },
  }
}
```

---

## 5. 验收测试场景

### 正例 1：创建图片库流程

```
Given: 用户在知识库列表页
When:
  1. 点击"创建知识库"
  2. 选择类型为"图片库"
  3. 自动显示 VLM 模型配置（有默认模型可选）
  4. 填写名称、选择模型、点击"创建"
Then:
  1. 知识库创建成功，跳转到图片库详情页
  2. 页面显示"图片管理"标题，无文档/FAQ 相关 tab
  3. 显示空白图片网格 + 上传引导
```

### 正例 2：上传图片 + 网格展示

```
Given: 图片库已创建
When:
  1. 点击上传按钮，选择 3 张 JPG 图片
  2. 进度条显示上传中
  3. 上传完成后，3 张图片出现在网格中
  4. 每张图片显示文件名 + 缩略图
  5. 等待几秒后，图片上出现"描述生成中..."状态指示器
  6. VLM 处理完成后，缩略图下方显示 AI 描述摘要
Then:
  1. 所有图片状态变为 "已处理"
  2. OCR 和 Caption 内容可查看
```

### 正例 3：搜索图片

```
Given: 图片库中有 10 张已处理的图片
When: 在搜索框中输入 "架构图"
Then:
  1. 网格筛选显示匹配的图片
  2. 匹配基于 AI Caption 和 OCR 文本的语义相似度
```

### 反例 1：上传不支持的格式

```
Given: 图片库详情页
When: 尝试上传 .svg 文件
Then:
  1. 前端拦截：文件选择对话框过滤掉 .svg（accept 属性）
  2. 如果绕过前端（API 直调），后端返回 400
  3. 前端显示错误提示："不支持的文件类型，仅支持 JPG/PNG/GIF/WebP/BMP"
```

### 反例 2：上传超大图片

```
Given: 图片库详情页
When: 选择一张 25MB 的 PNG 图片
Then:
  1. 前端预检（可选）: 提示文件过大
  2. 后端返回 400 "文件大小不能超过20MB"
  3. 前端 Toast 提示错误
```

### 反例 3：上传重复图片

```
Given: 图片库中已有一张 MD5=abc 的图片
When: 再次上传相同图片
Then:
  1. 后端返回 409 DUPLICATE_IMAGE
  2. 前端提示"该图片已存在於图片库中"
```

---

## 6. 文件清单

### 需要修改的文件

| 文件路径 | 改动说明 |
|----------|----------|
| `frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue` | 类型选择新增 "image" RadioButton；联动隐藏/显示配置区域 |
| `frontend/src/views/knowledge/KnowledgeBase.vue` | 新增 `isImage` computed；条件渲染图片网格视图 |
| `frontend/src/api/knowledge-base/index.ts` | 新增 `uploadImageToKB`、`batchUploadImageToKB` API 函数 |
| `frontend/src/i18n/locales/zh-CN.ts` | 新增图片相关翻译 |
| `frontend/src/i18n/locales/en-US.ts` | 新增图片相关翻译 |
| `frontend/src/hooks/useKnowledgeBase.ts` | 如果存在，可能需要扩展 |

### 需要新建的文件

| 文件路径 | 说明 |
|----------|------|
| `frontend/src/views/knowledge/components/ImageGridView.vue` | 图片网格展示组件（~200行） |
| `frontend/src/views/knowledge/components/ImageUploadZone.vue` | 拖拽上传区域组件（~80行） |
| `frontend/src/views/knowledge/components/ImageDetailDrawer.vue` | 图片详情抽屉组件（P1，~150行） |
