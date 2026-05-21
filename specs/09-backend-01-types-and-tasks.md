# Agent 开发卡片：图片库 — 后端类型定义与任务系统

> 模块编号：09-BACKEND-01 | 优先级：P0 | 预估工作量：2-3h

---

## 1. API 契约

> 本卡片仅覆盖"类型定义"与"任务系统"，不涉及 HTTP Handler。Handler 见卡片 09-BACKEND-02。

### 1.1 公共约定

- 前缀：`/api/v1`
- 认证：Bearer Token（`Authorization: Bearer <token>`）或 API Key（`X-API-Key: <key>`）
- 时区：UTC（Go `time.Time`），前端负责格式化
- 错误格式：`{ "success": false, "code": "ERROR_CODE", "message": "人类可读描述" }`

### 1.2 无新增 HTTP 端点（本卡片不涉及）

---

## 2. 数据库 DDL

**零变更。** 本次完全复用现有表结构：

```sql
-- 现有 knowledge_bases 表，type 字段 VARCHAR(32)，无 CHECK 约束
-- 新增合法值 "image"，由应用层保证合法性
SELECT type FROM knowledge_bases WHERE type = 'image';  -- 可正常运行

-- 现有 knowledges 表，type 字段 VARCHAR(50)，存储 "image"
-- file_path 存 provider:// 路径，file_hash 存 MD5
-- metadata JSON 可存图片尺寸、EXIF 等扩展信息

-- 现有 chunks 表，chunk_type 已有 "image_ocr" 和 "image_caption"
-- image_info TEXT 字段，存储 JSON: [{"url":"...", "caption":"...", "ocr_text":"..."}]
```

---

## 3. Redis Key Schema

**本次无需新增 Redis Key。** 处理进度通过 Knowledge 的 `parse_status` 字段反映。

---

## 4. Asynq 任务定义

### 4.1 新增任务类型常量

```go
// internal/types/task.go

const (
    // ... 现有常量 ...
    TypeImageProcess = "image:process"  // 图片处理任务（OCR + Caption + Embedding）
)
```

### 4.2 新增任务 Payload

```go
// internal/types/task.go

// ImageProcessPayload represents the image processing task payload.
// Enqueued when an image is uploaded to an "image" type knowledge base.
type ImageProcessPayload struct {
    TracingContext
    TenantID        uint64 `json:"tenant_id"`
    KnowledgeID     string `json:"knowledge_id"`
    KnowledgeBaseID string `json:"knowledge_base_id"`
    ImageURL        string `json:"image_url"`        // provider:// URL of the uploaded image
    EnableOCR       bool   `json:"enable_ocr"`        // always true for image KB
    EnableCaption   bool   `json:"enable_caption"`    // always true for image KB
    Language        string `json:"language,omitempty"` // Request locale
}
```

**触发条件**：每次图片上传成功后立即入队

**队列**：`"default"`

**最大重试**：3 次

### 4.3 Task Handler 注册

```go
// internal/container/container.go

// 在 NewTaskHandler 或类似位置新增：
mux.Handle(types.TypeImageProcess, imageHandler.HandleImageProcess)
```

---

## 5. 业务规则汇总

### 5.1 KnowledgeBaseType 新增

| 位置 | 代码 |
|------|------|
| `internal/types/knowledgebase.go` | `KnowledgeBaseTypeImage = "image"` |

### 5.2 KnowledgeType 新增

| 位置 | 代码 |
|------|------|
| `internal/types/knowledge.go` | `KnowledgeTypeImage = "image"` |

### 5.3 图片格式白名单

```go
// 放到 handler/knowledge.go 或 utils 中
var allowedImageTypes = map[string]bool{
    "jpg":  true,
    "jpeg": true,
    "png":  true,
    "gif":  true,
    "webp": true,
    "bmp":  true,
}

func IsAllowedImageType(fileType string) bool {
    return allowedImageTypes[strings.ToLower(strings.TrimSpace(fileType))]
}
```

### 5.4 单张图片大小限制

```
MAX_IMAGE_SIZE_BYTES = 20 * 1024 * 1024  // 20 MB
```

### 5.5 批量上传限制

```go
const MaxBatchImageUpload = 20
```

### 5.6 VLM 强制校验规则

创建 `type="image"` 的知识库时：
```
vlm_config.enabled MUST be true
vlm_config.model_id MUST be non-empty
embedding_model_id MUST be non-empty
```

### 5.7 图片去重规则

- 同一图片库内，`file_hash`（MD5 of image bytes）相同时拒绝上传
- 返回 HTTP 409 + `duplicate_image` 错误码
- 错误消息包含已存在的 Knowledge ID，前端可跳转到已有图片

### 5.8 图片库特有的边界条件

| 场景 | 处理方式 |
|------|----------|
| 非 image 类型 KB 调用 image 上传接口 | 400 "该知识库不是图片库类型" |
| image 类型 KB 调用文件上传接口（PDF等） | 400 "图片库不支持文档上传" |
| VLM 模型不可用 | 创建 KB 时 422 "图片库必须配置 VLM 模型" |
| VLM 调用超时 | Task 重试 3 次，全部失败后 status=failed |
| 图片文件损坏或非真实图片 | Task 中检测并 fail，error_message="无法解析图片文件" |

---

## 6. 验收测试场景

### 正例 1：正常创建图片库

```
Given: VLM 模型已配置且可用，Embedding 模型已配置
When: POST /api/v1/knowledge-bases { type: "image", vlm_config: { enabled: true, model_id: "vlm-001" }, ... }
Then: 返回 201，kb.type = "image"，kb.vlm_config.enabled = true
```

### 正例 2：图片处理任务完整流程

```
Given: 图片库已创建，上传一张 800x600 PNG 图片
When: 图片处理任务入队并执行
Then:
  - KNOWLEDGE.parse_status 从 "pending" → "processing" → "completed"
  - 生成 2 个 Chunk（image_ocr + image_caption）
  - Chunk.image_info 包含 url + caption + ocr_text
  - 向量索引成功写入
```

### 反例 1：无 VLM 配置创建图片库

```
Given: 未配置 VLM
When: POST /api/v1/knowledge-bases { type: "image", vlm_config: { enabled: false } }
Then: 返回 422，错误码 "VLM_REQUIRED_FOR_IMAGE_KB"
```

### 反例 2：重复上传同一图片

```
Given: 图片库已有图片 A（MD5=abc123）
When: 再次上传 MD5=abc123 的图片
Then: 返回 409，错误码 "duplicate_image"
      响应 data 包含已存在的 knowledge 信息
```

### 反例 3：上传不支持的图片格式

```
Given: 图片库已创建
When: 上传 .svg 文件
Then: 返回 400 "不支持的文件类型，仅支持 jpg, png, gif, webp, bmp"
```

---

## 7. 文件清单

### 需要修改的文件

| 文件路径 | 改动说明 |
|----------|----------|
| `internal/types/knowledgebase.go` | 新增 `KnowledgeBaseTypeImage = "image"`（1行） |
| `internal/types/knowledge.go` | 新增 `KnowledgeTypeImage = "image"`（1行） |
| `internal/types/task.go` | 新增 `TypeImageProcess` 常量 + `ImageProcessPayload` 结构体 |
| `internal/container/container.go` | 注册 `TypeImageProcess` Handler |
| `internal/types/interfaces/knowledge.go` | `KnowledgeService` 接口新增 `CreateKnowledgeFromImage` 和 `ProcessImageKnowledge` 方法签名 |

### 需要新建的文件

| 文件路径 | 改动说明 |
|----------|----------|
| `internal/application/service/image_knowledge.go` | 图片处理服务：`processImageKnowledge()`（VLM调用 + Chunk生成 + 向量化） |
