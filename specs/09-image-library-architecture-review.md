# 图片库功能 — 架构评审报告

> 评审日期：2026-05-21 | 需求版本：v1.0 | 评审结论：✅ 通过（含 3 项关键建议）

---

## 一、评审维度与结论

| 维度 | 评分 | 说明 |
|------|:----:|------|
| **API 契约** | ✅ 8/10 | 端点定义清晰，需补充具体 Error 码和字段验证细节 |
| **数据模型** | ✅ 9/10 | 完全复用现有 3 张表，零 DDL 变更，架构优雅 |
| **Redis** | ✅ N/A | 本次无需新增 Redis Key |
| **异步任务** | ✅ 8/10 | 可复用现有 `TypeDocumentProcess`，建议分支处理 |
| **验收标准** | ⚠️ 6/10 | 需求文档中有正例，反例不够具体 |
| **环境配置** | ✅ 9/10 | 无新增环境变量，依赖现有 VLM + Storage 基础设施 |

---

## 二、关键发现

### 2.1 复用现有表结构 — 零 DDL 变更 ✅

```
knowledge_bases (type="image")
    └── knowledges (type="image", file_path=provider://...)
         └── chunks (chunk_type in {image_ocr, image_caption})
```

**验证结论**：三张表无需任何 `ALTER TABLE`。`KnowledgeBase.Type` 和 `Knowledge.Type` 字段均为 `VARCHAR`，无 CHECK 约束，新增 `"image"` 值完全兼容。

### 2.2 VLM 多模态管线可直接复用 ✅

现有 `ImageMultimodalPayload` + `image_multimodal.go` handler 已支持：
- VLM OCR + Caption 独立调用
- `provider://` 图片读取
- `ImageInfo` 结构 (URL + Caption + OCRText)
- 异步任务入队/处理/状态更新

**需要调整**：现有 `ImageMultimodalPayload` 有 `ChunkID` 和 `ImageLocalPath` 字段，是"文档内图片"场景专用的。图片库场景下图片就是主内容，需要新建一个更简洁的 Payload。

### 2.3 关键决策点

#### 决策 1：是新建 Asynq Task Type 还是复用 `TypeDocumentProcess`？

**建议：新建 `TypeImageProcess = "image:process"`**

| 方案 | 优点 | 缺点 |
|------|------|------|
| 复用 `TypeDocumentProcess` | 改动小，一个分支搞定 | 语义不清，`processDocument` 函数已 500+ 行，再加分支更难维护 |
| **新建 `TypeImageProcess`** ✅ | 职责单一，代码清晰 | 新增一个 handler 文件 |

#### 决策 2：图片上传如何存储？

**建议：复用 `CreateKnowledgeFromFile` 的存储逻辑**

```
上传图片 → FileService.SaveFile() → provider:// URL
    → Knowledge.FilePath = "local://tenant/images/{uuid}.png"
    → Knowledge.FileHash = MD5(imageBytes)
```

现有 `CreateKnowledgeFromFile` 已完整实现此流程，图片库的 handler 可复用 `knowledgeService` 内部逻辑。

#### 决策 3：缩略图生成时机？

**建议：异步生成（在处理任务中）**

```
ImageProcess Task:
  1. 读取原图
  2. 生成缩略图 → provider://{原图路径}_thumb.jpg
  3. VLM OCR
  4. VLM Caption
  5. Embedding + 索引
  6. Knowledge.status = "completed"
```

---

## 三、需要修改的文件清单（预估）

### 后端 (Go) — 共约 12 个文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/types/knowledgebase.go` | 1 行 | 新增 `KnowledgeBaseTypeImage = "image"` |
| `internal/types/knowledge.go` | 1 行 | 新增 `KnowledgeTypeImage = "image"` |
| `internal/types/task.go` | 2 行 | 新增 `TypeImageProcess = "image:process"` |
| `internal/types/task.go` | ~15 行 | 新增 `ImageProcessPayload` 结构体 |
| `internal/handler/knowledge.go` | ~60 行 | 新增 `CreateImageKnowledge` + `CreateImageKnowledgeBatch` handler |
| `internal/handler/knowledgebase.go` | ~10 行 | `CreateKnowledgeBase` 增加 image 类型校验（VLM 必填） |
| `internal/application/service/knowledge.go` | ~120 行 | 新增 `CreateKnowledgeFromImage` + `processImageKnowledge` 方法 |
| `internal/application/service/image_multimodal.go` | ~30 行 | 抽取 VLM 调用为公共函数 |
| `internal/container/container.go` | ~3 行 | 注册新 task handler |
| `internal/router/router.go` | ~8 行 | 注册图片上传路由 |
| `internal/types/interfaces/knowledge.go` | ~5 行 | 接口新增方法签名 |

### 前端 (Vue3+TS) — 共约 8 个文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue` | ~5 行 | 类型选择增加 "image" 选项 |
| `frontend/src/views/knowledge/KnowledgeBase.vue` | ~60 行 | 新增 `isImage` computed，图片网格视图 |
| `frontend/src/views/knowledge/components/ImageGridView.vue` | 新建 ~200 行 | 图片网格组件 |
| `frontend/src/views/knowledge/components/ImageUploadZone.vue` | 新建 ~80 行 | 拖拽上传区域组件 |
| `frontend/src/views/knowledge/components/ImageDetailDrawer.vue` | 新建 ~150 行 | 图片详情抽屉 |
| `frontend/src/api/knowledge-base/index.ts` | ~15 行 | 新增 API 函数 |
| `frontend/src/i18n/locales/zh-CN.ts` | ~20 行 | 新增中文翻译 |
| `frontend/src/i18n/locales/en-US.ts` | ~20 行 | 新增英文翻译 |

---

## 四、风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| VLM 调用超时 | 图片处理失败 | 设置 60s 超时 + 3 次重试 |
| 大量图片并发上传 | 存储压力 + VLM 队列堆积 | 批量限制 20 张/次，Asynq 队列自动排队 |
| 图片格式安全 | 上传恶意文件 | MIME 白名单 + 魔数校验 + 文件头解析 |
| VLM 模型不可用 | 所有图片处理失败 | 创建图片库时强制校验 VLM 可用性 |

---

## 五、评审结论

**✅ 通过。** 该需求与现有架构高度一致，主要工作是新增一个"支路"而非改造主干。

**3 项必须在编码前明确的关键建议：**

1. **新建 `TypeImageProcess` 任务类型，而非复用 `TypeDocumentProcess`**。两者处理逻辑差异大（图片不走 docreader、不分块、不需要 chunking config），强行复用会导致 `processDocument` 函数进一步膨胀。

2. **图片上传 Handler 不作为 `/knowledge-bases/:id/knowledge/file` 的子路径**。建议新增独立端点 `/knowledge-bases/:id/knowledge/image`，语义更清晰，且可以避免 Multipart 字段解析与现有文件上传逻辑冲突。

3. **前端新增 `isImage` computed，与 `isFAQ` 并列**。在 `KnowledgeBase.vue` 中按照 `isFAQ / isImage / isWiki / isDocument` 的优先级做条件渲染，避免多层嵌套 v-if 难以维护。
