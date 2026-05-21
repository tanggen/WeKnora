# 知识库新增图片库功能 — 需求文档

> 版本：v1.0 | 状态：待确认 | 日期：2026-05-21

---

## 一、功能清单

| 编号 | 功能 | 优先级 | 说明 |
|------|------|:------:|------|
| **IMG-01** | 新增 `image` 类型知识库 | P0 | KnowledgeBaseType 新增 `"image"`；创建知识库时可选择"图片库" |
| **IMG-02** | 图片上传 | P0 | 支持 JPG/PNG/GIF/WebP/BMP 格式直接上传到图片库 |
| **IMG-03** | VLM 多模态处理 | P0 | 每张图片通过 VLM 做 OCR 文字提取 + AI 描述（Caption） |
| **IMG-04** | 图片 Chunk 向量化 | P0 | OCR 文本和 Caption 分别生成 `image_ocr` / `image_caption` 类型的 Chunk 并向量索引 |
| **IMG-05** | 自然语言搜索图片 | P1 | 用户输入关键词/描述，通过向量检索匹配到相关图片 |
| **IMG-06** | 图片列表/网格展示 | P0 | 图片库内以网格形式预览图片，支持筛选、排序、删除 |
| **IMG-07** | 图片详情查看 | P1 | 点击图片进入详情页，展示原图、OCR 文本、AI 描述、元信息 |
| **IMG-08** | 图片缩略图生成 | P1 | 上传时自动生成缩略图用于列表快速预览 |
| **IMG-09** | 批量操作 | P2 | 支持批量上传、批量删除图片 |
| **IMG-10** | 图片库独立配置 | P1 | image 类型知识库专用的 VLM 模型配置、存储配置 |

---

## 二、IMG-01：新增 `image` 类型知识库

### 类型定义变更

```
// internal/types/knowledgebase.go
const (
    KnowledgeBaseTypeDocument = "document"
    KnowledgeBaseTypeFAQ      = "faq"
    KnowledgeBaseTypeWiki     = "wiki"
    KnowledgeBaseTypeImage    = "image"   // 新增
)
```

### 创建规则

1. 创建图片库时，`type` 字段传 `"image"`
2. **必填配置项**：
   - `vlm_config.enabled` = true（图片库必须启用 VLM）
   - `vlm_config.model_id` 必须绑定一个有效的 VLM/多模态模型
   - `embedding_model_id` 必须绑定一个有效的 Embedding 模型
   - `storage_provider_config.provider` 指定存储方案（local / minio / cos）
3. **不可用配置**（image 类型不适用）：
   - `chunking_config`：图片不做文本分块
   - `faq_config`：仅 FAQ 类型使用
   - `wiki_config`：仅 Wiki 类型使用
   - `indexing_strategy.wiki_enabled`：不支持 Wiki
   - `indexing_strategy.graph_enabled`：不支持知识图谱
4. 图片库的**索引策略**默认：`vector_enabled=true, keyword_enabled=false`

### 图片库与 Document 库的区别

| 维度 | Document 库 | Image 库 |
|------|-------------|----------|
| 上传内容 | PDF/Word/Markdown/TXT 等 | JPG/PNG/GIF/WebP/BMP |
| 文档内图片 | 提取后 VLM 处理 | N/A（图片本身就是主内容） |
| 文本分块 | 按 ChunkingConfig 分块 | 不分块 |
| Chunk 产出 | text 为主，image_ocr/caption 为辅 | image_ocr + image_caption 为主 |
| 检索方式 | 文本语义搜索 | 图片描述语义搜索 |
| VLM 强制 | 否（可选启用） | 是（必须启用） |

---

## 三、IMG-02：图片上传

### 支持格式

| 格式 | MIME Type | 扩展名 |
|------|-----------|--------|
| JPEG | image/jpeg | .jpg, .jpeg |
| PNG | image/png | .png |
| GIF | image/gif | .gif |
| WebP | image/webp | .webp |
| BMP | image/bmp | .bmp |

### 上传限制

| 限制项 | 值 | 说明 |
|--------|-----|------|
| 单文件最大 | 20 MB | 复用现有 `MAX_FILE_SIZE_MB` 或独立 `MAX_IMAGE_SIZE_MB` |
| 单次批量最大 | 20 张 | 超过需分批上传 |
| 支持格式校验 | 白名单 | 仅允许上述 5 种格式 |
| 去重策略 | 文件 Hash | 同一图片库内相同 MD5 文件拒绝重复上传 |

### API 端点

```
POST /api/v1/knowledge-bases/:id/image              # 单张上传
POST /api/v1/knowledge-bases/:id/image/batch        # 批量上传（最多 20 张）
```

**Request (multipart/form-data)**：
```
file: <binary>           # 图片文件（单张时）
files: <binary[]>        # 图片文件数组（批量时）
tag_id: string (可选)    # 所属分类 ID
```

---

## 四、IMG-03：VLM 多模态处理

### 处理流程

```
图片上传 → 存储到 provider:// → 创建 Knowledge 记录（status=pending）
    → Asynq 入队 ImageProcess 任务
    → 读取图片 → 调用 VLM 模型
    → OCR 提取文字 (image_ocr chunk)
    → AI 生成描述 (image_caption chunk)
    → Embedding 向量化两个 chunk
    → 写入向量数据库
    → Knowledge status → completed
```

### 处理管线（与现有 document 管线的区别）

1. **跳过 docreader 解析**：图片不需要文档解析器，直接从存储读取
2. **跳过文本分块**：不需要 ChunkingConfig
3. **直接调 VLM**：复用现有 `image_multimodal.go` 中的 VLM 调用逻辑：
   - OCR Prompt: 提取图片中的文字信息
   - Caption Prompt: 用自然语言描述图片内容
4. **异步任务**：新增 `TypeImageProcess` 或复用 `TypeDocumentProcess`（通过 `KnowledgeTypeImage` 区分处理分支）

### VLM Prompt 设计

**OCR Prompt：**
```
请提取这张图片中所有可见的文字信息，按照阅读顺序输出。
忽略页眉页脚。表格用 HTML 格式表达。公式用 LaTeX 格式表示。
```

**Caption Prompt：**
```
请用一段自然语言详细描述这张图片的内容。
包括：主体对象、场景、颜色、布局、文字（如果有的话）、
以及这张图片可能用于什么用途。
输出不超过 200 字的中文描述。
```

---

## 五、IMG-04：图片 Chunk 向量化

### Chunk 结构

每张图片产生 **2 个 Chunk**：

```json
// Chunk 1: OCR 文本
{
    "chunk_type": "image_ocr",
    "content": "<OCR 提取的文字>",
    "knowledge_id": "<knowledge_id>",
    "knowledge_base_id": "<kb_id>",
    "parent_chunk_id": "<image_caption_chunk_id>",  // 关联到图片描述 chunk
    "image_info": "{\"url\":\"minio://...\",\"caption\":\"...\",\"ocr_text\":\"...\"}"
}

// Chunk 2: AI 描述
{
    "chunk_type": "image_caption",
    "content": "<VLM 生成的描述文本>",
    "knowledge_id": "<knowledge_id>",
    "knowledge_base_id": "<kb_id>",
    "image_info": "{\"url\":\"minio://...\",\"caption\":\"...\",\"ocr_text\":\"...\"}"
}
```

### 向量索引

- 两个 Chunk **都参与向量索引**
- `image_ocr` Chunk：侧重精确文字匹配
- `image_caption` Chunk：侧重语义搜索
- 检索时返回匹配的 Chunk，前端通过 `knowledge_id` 关联回原图

---

## 六、IMG-05：自然语言搜索图片

### 搜索流程

```
用户输入 "包含架构图的图片"
    → Embedding 向量化查询文本
    → 在 image_caption Chunk 中做向量相似度搜索
    → 返回 Top-K 匹配结果
    → 每个结果包含：图片URL、Caption 文本、OCR 文本、相似度分数
```

### API 端点

复用现有搜索接口：
```
GET /api/v1/knowledge-bases/:id/hybrid-search
    ?query_text=包含架构图的图片
    &match_count=10
```

当 KB 类型为 `image` 时，搜索自动在 `image_ocr` + `image_caption` chunk 中检索。

---

## 七、IMG-06/07：前端展示

### 图片库列表页（KnowledgeBase.vue 内）

当 KB 类型为 `image` 时：

- **网格视图**：每张图片显示缩略图 + 文件名 + AI 描述摘要
- **筛选**：按 tag_id 分类、按上传时间排序
- **操作**：查看详情、删除、下载原图
- **上传入口**：顶部上传按钮，支持拖拽上传

### 图片详情弹窗/页面

```
┌──────────────────────────────────────┐
│  [原图显示区域 - 可缩放]              │
│                                      │
├──────────────────────────────────────┤
│  📝 AI 描述：xxx...                  │
│  📄 OCR 文字：xxx...                 │
│  📅 上传时间：2026-05-21             │
│  📏 大小：2.3 MB                     │
│  📂 分类：xxx                        │
└──────────────────────────────────────┘
```

### 创建知识库时新增类型选项

在 `KnowledgeBaseEditorModal.vue` 中，类型选择增加：
```html
<t-radio-button value="image">{{ $t('knowledgeEditor.basic.typeImage') }}</t-radio-button>
```

类型选择变更后，表单动态调整：
- 选择 `image` → 隐藏分块配置、FAQ 配置、Wiki 配置区域
- 选择 `image` → 强制显示 VLM 模型配置（必填）

---

## 八、IMG-08：缩略图生成（P1）

- 上传图片时，调用图片处理库（如 Go 的 `imaging` 库）生成缩略图（200x200）
- 缩略图存储到同一 provider 路径下：`{原图路径}_thumb.jpg`
- 列表接口返回时带上 `thumbnail_url` 字段
- 缩略图格式统一为 JPEG（体积小、兼容性好）

---

## 九、IMG-09：批量操作（P2）

### 批量上传

- 前端支持多选文件（最多 20 张）
- 后端批量接口：循环创建 Knowledge + 逐个入队处理任务
- 返回结果：`{ success_count: 15, failed: [{file_name: "xxx", reason: "文件类型不支持"}] }`

### 批量删除

- 复用现有 `POST /api/v1/knowledge/batch-delete` 接口
- 前端支持多选 + 全选删除

---

## 十、IMG-10：图片库独立配置

图片库使用以下独立配置项（在 KnowledgeBase 中已有对应字段）：

| 配置项 | 字段 | 说明 |
|--------|------|------|
| VLM 模型 | `vlm_config.model_id` | 必填，图片库创建时强制校验 |
| 多模态开关 | `vlm_config.enabled` | 默认 true，不可关闭（图片库必须开启） |
| 存储引擎 | `storage_provider_config.provider` | local / minio / cos |
| Embedding 模型 | `embedding_model_id` | 用于向量化 OCR/Caption 文本 |
| 图片处理配置 | `image_processing_config` | 复用现有字段 |

---

## 十一、数据流总览

```
┌──────────────────────────────────────────────────────────────┐
│                      图片库完整数据流                          │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  前端上传图片               后端处理管线                       │
│  ┌──────────┐          ┌─────────────────────────┐          │
│  │ 多选图片  │──POST──▶│ /api/v1/knowledge-bases/ │          │
│  │ 拖拽上传  │         │   :id/image              │          │
│  └──────────┘          └───────────┬─────────────┘          │
│                                    │                         │
│                          ┌─────────▼──────────┐              │
│                          │  存储到 provider://  │              │
│                          │  (local/minio/cos)  │              │
│                          └─────────┬──────────┘              │
│                                    │                         │
│                          ┌─────────▼──────────┐              │
│                          │  CreateKnowledge()  │              │
│                          │  type="image"       │              │
│                          │  status="pending"   │              │
│                          └─────────┬──────────┘              │
│                                    │                         │
│                          ┌─────────▼──────────┐              │
│                          │  Asynq:             │              │
│                          │  TypeImageProcess   │              │
│                          └─────────┬──────────┘              │
│                                    │                         │
│                    ┌───────────────┼───────────────┐         │
│                    │               │               │         │
│              ┌─────▼─────┐  ┌──────▼──────┐        │         │
│              │ VLM OCR    │  │ VLM Caption │        │         │
│              │ (文字提取)  │  │ (内容描述)   │        │         │
│              └─────┬─────┘  └──────┬──────┘        │         │
│                    │               │               │         │
│              ┌─────▼─────┐  ┌──────▼──────┐        │         │
│              │ Chunk:     │  │ Chunk:       │        │         │
│              │ image_ocr  │  │ image_caption│        │         │
│              └─────┬─────┘  └──────┬──────┘        │         │
│                    │               │               │         │
│                    └───────┬───────┘               │         │
│                            │                       │         │
│                    ┌───────▼───────┐               │         │
│                    │  Embedding     │               │         │
│                    │  向量化 + 索引  │               │         │
│                    └───────┬───────┘               │         │
│                            │                       │         │
│                    ┌───────▼───────┐               │         │
│                    │  status=       │               │         │
│                    │  completed    │               │         │
│                    └───────────────┘               │         │
│                                                              │
│  ───────────── 检索阶段 ─────────────                        │
│                                                              │
│  用户搜索 "架构图"                                           │
│       │                                                      │
│       ▼                                                      │
│  HybridSearch → Embedding → 向量匹配 image_caption Chunk     │
│       │                                                      │
│       ▼                                                      │
│  返回: [{ image_url, caption, ocr_text, score }]              │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## 十二、确认结论

| # | 问题 | 结论 |
|---|------|------|
| 1 | 图片库是否需要独立的 KnowledgeBase Type？ | **是**，新增 `"image"` 类型 |
| 2 | 每张图片是否作为一个独立的 Knowledge 条目？ | **是**，复用 `knowledges` 表，`type="image"` |
| 3 | Chunk 策略？ | 每张图片生成 2 个 Chunk：`image_ocr` + `image_caption`，均参与向量索引 |
| 4 | 图片库是否必须启用 VLM？ | **是**，创建时强制校验 `vlm_config.enabled=true` 且 `model_id` 有效 |
| 5 | 是否需要新建数据库表？ | **否**，完全复用 `knowledge_bases`、`knowledges`、`chunks` 三张表 |
| 6 | 是否支持批量上传？ | P2 支持，P0 先支持单张上传 |
| 7 | 图片库是否支持"以图搜图"？ | 本期不支持（需引入视觉 Embedding 模型），留待后续版本 |
| 8 | 图片去重策略？ | 同图片库内基于 `file_hash` 去重 |
| 9 | 图片库是否参与 Agent 检索？ | 是，Agent 调用 `kbSearch` 时自动包含图片库的 `image_caption` chunk |
| 10 | 缩略图何时生成？ | 上传时异步生成，存储为 `{原图}_thumb.jpg` |

---

## 十三、简化决策（降低第一版复杂度）

> 与完整方案相比，第一版做以下简化以快速交付：

| 功能 | 完整方案 | 第一版简化 |
|------|----------|-----------|
| 以图搜图 | CLIP 视觉 Embedding | 不做 |
| EXIF 解析 | 提取元数据 | P2 再做 |
| 智能标签 | VLM 自动打标签 | 手动 tag_id 分类即可 |
| 相册/集合 | 多级分组 | P2 再做 |
| 图片编辑 | 旋转/裁剪 | 不做 |
| 相似图片检索 | 去重检测 | 仅 Hash 去重 |
