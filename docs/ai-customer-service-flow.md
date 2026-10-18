# GoRag 智能客服 — 会话总结与 AI 客服流程梳理

> 本文档记录了本项目从「跑不起来」到「跑通」过程中遇到的所有问题与答案，并梳理了 AI 客服一次提问-回答的完整代码链路。

---

## 一、会话问题与答案梳理

### 1. `go.mod` 报错 `unexpected input character '﻿'`

- **现象**：`go: errors parsing go.mod: unexpected input character '﻿'`
- **原因**：`go.mod` 文件开头多了一个 **UTF-8 BOM**（`EF BB BF`），是编辑器以「UTF-8 with BOM」保存导致的，Go 工具链不识别 BOM。
- **解决**：去掉文件开头的 3 字节 BOM（`sed -i '1s/^\xEF\xBB\xBF//' go.mod`）。

### 2. 项目怎么启动运行

- 依赖：Redis（`docker compose -f deployments/docker-compose.yml up -d redis`）
- 后端：项目根目录 `APP_ENV=local go run ./cmd/server`（监听 8080）
- 前端：`cd web && npm run dev`（监听 5173）
- 管理员默认账号：`admin` / `admin123`

### 3. 后端源码中文乱码（编译失败）

- **现象**：`go build` 报 `newline in string`、`invalid character` 等错误，中文显示为乱码（如「火山方舟」→「鐏北鏂硅垷」）。
- **根因**：最新提交 `bddc47f`（"初始化 GoRag 开源项目"）在重命名包时，把源码里的中文做了 **UTF-8 → GBK → UTF-8 双重编码**，导致乱码且部分字节丢失。
- **修复**：从父提交 `0d03c7f` 恢复干净中文，保留合法的「包重命名 + wecom 删除」。共修复 14 个 `.go` 文件。

### 4. 为什么向量检索需要 embedding（代码位置）

- **原理**：向量检索（KNN）比较的是「向量」间的距离；文字要先转成一串数字（向量）才能比较，这个「文字→向量」就是 embedding。
- **代码**：
  - 查询侧转向量：`internal/graph/rag/pipeline.go:85`（`s.embedder.EmbedStrings`）
  - 入库侧转向量：`internal/knowledge/qa/service.go:63`（`embedQuestion`）
  - KNN 检索本体：`internal/vectorstore/redissearch.go:140`（`*=>[KNN ...]`）
  - 入库强制要求向量：`internal/vectorstore/redissearch.go:76`（`vector is empty` 报错）
  - embedder 接口：`internal/model/factory.go:40`

### 5. 纯关键词 RAG 改造（无 embedding 也能检索）

- 改动点（均已实现）：
  - `internal/vectorstore/redissearch.go` — `Ensure` 在 `dim<=0` 时建纯文本索引（无向量字段）；`Upsert` 允许空向量只存文本。
  - `internal/knowledge/qa/service.go` — `upsertItem` 在 embedding 失败时降级为只存文本。
  - `internal/server/handler/chat.go` / `knowledge.go` — 去掉对 embedding 的硬性拦截。

### 6. Redis 启动

- 命令：`docker compose -f deployments/docker-compose.yml up -d redis`
- 后端连不上 Redis 的报错：`connect redis: ... actively refused`，说明 Redis 容器没起。

### 7. 前端测试 AI 客服点哪里

- 登录后台 `http://localhost:5173`，左侧菜单：
  - **「QA 知识库」**（`/qa`）：导入/管理知识库
  - **「测试客服」**（`/chat`）：直接和 AI 客服对话
- 路由定义：`web/src/router/index.ts:19-29`

### 8. 后端启动的目录和指令

- 目录：项目根目录（有 `go.mod` 和 `config/` 的那层），不能在 `cmd/server` 里。
- 指令：`APP_ENV=local go run ./cmd/server`（Git Bash）；cmd 用 `set APP_ENV=local && go run ./cmd/server`。

### 9. 端口占用 `bind: Only one usage of each socket address`

- 原因：上一个后端进程没退干净，还占着 8080。
- 解决：`netstat -ano | grep ":8080" | grep LISTENING` 找 PID，`taskkill //PID <PID> //F` 结束。

### 10. 调用方 Token 报 `invalid caller token`

- **根因**：把 DeepSeek 的 LLM API Key（`***`）当成了「调用方 Token」。
- **区别**：`***` 是调模型用的；`as_...` 是外部系统接入 Biz API 用的，需在后台「调用方 Token」创建。
- 创建逻辑：`internal/auth/service.go:268`（`CreateCallerToken`），生成 `as_` + 48 位 hex（`generateSecret`），只存 SHA-256 哈希。

### 11. Milvus 启动（遇到网络问题）

- 命令：`docker compose -f deployments/docker-compose.yml --profile milvus up -d`（拉起 etcd + minio + milvus）
- 当时 Docker Hub 被墙、本地代理 HTTPS 不通，镜像拉不下来，属于网络问题而非项目问题。

### 12. 接入火山方舟（Ark）embedding

- 用户提供 API Key（`***`）+ 官方示例，确认为**多模态**接口 `/embeddings/multimodal`。
- 配置（`.env`）：
  ```env
  EMBEDDING_PROVIDER=ark_multimodal
  EMBEDDING_API_KEY=***
  EMBEDDING_BASE_URL=https://ark.cn-beijing.volces.com/api/v3
  EMBEDDING_MODEL_NAME=ep-***
  EMBEDDING_DIMENSIONS=2048
  ```
- 维度实测 = **2048**（模型 `doubao-embedding-vision-251215`）。
- 需删旧 `data/app.db` + 旧索引重新播种（旧 profile 存在时 `seedProvider` 会跳过）。

### 13. 用 Navicat 查看 SQLite

- 数据库文件：`data/app.db`（无用户名密码）
- Navicat 选「SQLite → 现有数据库文件」，浏览选中该文件即可。

### 14. 表结构设计在哪

- 全部在 `internal/db/models.go`：Go 结构体 + GORM 标签定义列；`TableName()` 定义表名。
- `internal/db/db.go:78` 的 `AutoMigrate` + `Models()` 列表负责建表，启动时在 `internal/bootstrap/providers.go:22` 调用。

### 15. 生成调用方 Token

- 走创建接口：`POST /api/v1/admin/caller-tokens`（`internal/server/router.go:134`），返回 `app_secret` 即明文 token。

---

## 二、AI 客服「提问-回答」完整流程

一次用户提问，从 HTTP 入口到流式返回，链路如下：

```
HTTP 请求
  → 路由 (router.go)
  → 鉴权中间件 (middleware.go)
  → ChatHandler.Chat (chat.go)
  → canHandleChat 检查模型配置
  → decideChat 意图/上下文判断
  → buildMessages 组装消息
  → Agent Runner 执行 (Eino ADK)
      → rag_search 工具 → RAG 双路检索
      → chat 模型 (DeepSeek) 生成
  → SSE 流式返回
  → 会话留存 (sessions/messages)
```

### 步骤 1：HTTP 入口 / 路由

`internal/server/router.go`

```go
// Biz API（外部接入，Caller Token 鉴权）
biz.POST("/chat/ask", deps.ChatHandler.Ask)                 // :47
sessions.POST("/:id/chat", middleware.SessionSerial(), deps.ChatHandler.Chat)  // :57

// Admin API（测试客服，JWT 鉴权）
playground.POST("/sessions/:id/chat", deps.ChatHandler.Chat) // :112
```

### 步骤 2：鉴权中间件（Biz API）

`internal/server/middleware/middleware.go:140`

```go
if raw := strings.TrimSpace(c.GetHeader("X-Caller-Token")); raw != "" {
    identity, err := authSvc.ValidateCallerToken(c.Request.Context(), raw)
    if err != nil {
        response.Error(c, http.StatusUnauthorized, "invalid caller token") // 你之前看到的报错
        return
    }
    bizCtx.CallerTokenID = identity.TokenID
}
```

校验逻辑在 `internal/auth/service.go:619`（`ValidateCallerToken`），比对 `HashSecret(raw)`。

### 步骤 3：ChatHandler.Chat 主流程

`internal/server/handler/chat.go:120`

```go
func (h *ChatHandler) Chat(c *gin.Context) {
    // 1) 取会话 + 历史消息
    history, _ := h.sessionStore.GetMessages(ctx, sessionID)
    history = h.limitAgentHistory(ctx, history)

    // 2) 检查模型是否配置
    if !h.canHandleChat(c) { return }

    // 3) 意图/上下文决策（可能直接返回兜底回复）
    decision, _ := h.decideChat(ctx, history, req.Message)
    if decision.Reply != "" {
        // 直接回复（如上下文超限），不走模型
        h.sseWriter.WriteEvent(...)
        return
    }

    // 4) 组装消息，交给 Agent Runner 流式执行
    userMsg, messages := h.buildMessages(ctx, history, req.Message, decision)
    iter := h.runner.Run(runCtx, messages, adk.WithCheckPointID(sessionID))

    // 5) 遍历事件，SSE 逐字写出
    for { event, ok := iter.Next(); ... }
}
```

### 步骤 4：canHandleChat 检查模型配置

`internal/server/handler/chat.go:430`

```go
func (h *ChatHandler) canHandleChat(c *gin.Context) bool {
    if !h.settings.ChatConfigured(ctx) {
        response.Error(c, 503, "chat model is not configured in GM")
        return false
    }
    // 未配置 embedding 时不再拦截（RAG 会退化为纯关键词检索）
    return true
}
```

### 步骤 5：decideChat 意图/上下文判断

`internal/server/handler/chat.go:514`

- 空输入直接返回；
- 上下文超过限制时返回兜底回复（`PromptChatContextLimit`）；
- 否则正常请求统一交给 Agent，由模型自主决定是否调用知识库 / 转人工工具。

### 步骤 6：Agent 构建（启动时）

`internal/bootstrap/wire.go:92`

```go
customerAgent, err := agent.NewCustomerServiceAgent(ctx, chatModel, ragService, instruction)
```

`internal/agent/builder.go:16`

```go
func NewCustomerServiceAgent(...) (adk.Agent, error) {
    ragTool, _ := agenttools.NewRAGSearchTool(ragService)
    return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "CustomerServiceAgent",
        Instruction: instruction,          // 系统提示词
        Model:       chatModel,            // DeepSeek
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools: []tool.BaseTool{ragTool},  // knowledge_search 工具
            },
        },
    })
}
```

系统提示词（客服人设）在 `internal/agent/instruction.go:3`（`CustomerServiceInstruction`）。

### 步骤 7：rag_search 工具 → RAG 双路检索

`internal/agent/tools/rag_search.go:29`

```go
func NewRAGSearchTool(service *rag.Service) (tool.InvokableTool, error) {
    return utils.InferTool("knowledge_search", "检索 QA 知识库...", func(ctx, input *SearchInput) (string, error) {
        hits, err := service.SearchHits(ctx, input.Query)
        if len(hits) == 0 { return "未找到相关 QA 知识库内容。", nil }
        return rag.FormatHits(hits), nil
    })
}
```

### 步骤 8：RAG 双路检索（关键词 + 向量）

`internal/graph/rag/pipeline.go:63`（`SearchHits`）

```go
// ① 文本检索（BM25，不需要 embedding）—— 并发执行
go func() { records, err = s.store.SearchText(ctx, query, candidateLimit) }()

// ② 向量检索（KNN，需要 embedding）—— 先把查询词转向量
vecs, err := s.embedder.EmbedStrings(ctx, []string{query})
vectorRecords, vectorErr = s.store.Search(ctx, qv, candidateLimit)

// ③ 融合 / 重排 / 过滤；向量不可用时退化为纯文本
```

- 文本检索：`internal/vectorstore/redissearch.go:95`（`SearchText` → BM25）
- 向量检索：`internal/vectorstore/redissearch.go:140`（`Search` → `*=>[KNN ...]`）

### 步骤 9：Chat 模型（DeepSeek）

`internal/model/factory.go:24`（`NewChatModel`）

```go
case "openai", "openai_compatible", "compatible", ...:
    return openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{
        BaseURL: cfg.BaseURL,   // https://api.deepseek.com/v1
        APIKey:  cfg.APIKey,
        Model:   cfg.ModelName, // deepseek-flash / deepseek-v4-pro
    })
```

### 步骤 10：SSE 流式返回

`internal/server/handler/chat.go:187-218`

- 设置 `text/event-stream` 响应头；
- 遍历 Agent Runner 的事件流，用 `h.sseWriter.WriteWithObserver` 把每个 `stream_chunk` 逐字写出并 `Flush`。

### 步骤 11：会话留存

`internal/server/handler/chat.go:155 / 239`

- 用户消息、助手回复（含工具调用）通过 `sessionStore.AppendMessages` 写入 `sessions` / `messages` 表。
- 存储实现：`internal/session/sql_store.go`（SQLite/MySQL）。

---

## 三、关键文件速查

| 关注点 | 文件 |
|--------|------|
| 路由 | `internal/server/router.go` |
| 鉴权中间件 | `internal/server/middleware/middleware.go` |
| 对话 Handler | `internal/server/handler/chat.go` |
| Agent 构建 | `internal/agent/builder.go` |
| 客服提示词 | `internal/agent/instruction.go` |
| RAG 检索工具 | `internal/agent/tools/rag_search.go` |
| RAG 检索管道 | `internal/graph/rag/pipeline.go` |
| 模型工厂（chat/embedding） | `internal/model/factory.go` |
| 向量存储（RedisSearch） | `internal/vectorstore/redissearch.go` |
| 会话存储 | `internal/session/sql_store.go` |
| 数据表结构 | `internal/db/models.go` + `internal/db/db.go` |
| 模型配置解析 | `internal/admin/settings.go`、`internal/application/settings/*` |
| 前端路由/页面 | `web/src/router/index.ts`、`web/src/views/*` |
