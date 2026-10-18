# GoRag AI 客服框架 · 学习笔记

> 本文档整理自一次完整的学习会话，涵盖：项目全景、AI Agent 核心知识点、核心代码导读、
> AI 客服「提问 → 回答」的完整代码流程，以及会话中所有问答的总结。
>
> 面向想通过本项目学习「AI Agent 开发」的后端开发者。阅读顺序建议按目录自上而下。

---

## 目录

1. [项目全景](#一项目全景)
2. [AI Agent 核心知识点清单](#二ai-agent-核心知识点清单)
3. [核心代码导读（按优先级）](#三核心代码导读按优先级)
4. [AI 客服问答完整流程（附代码块）](#四ai-客服问答完整流程附代码块)
5. [会话问答总结（Q&A）](#五会话问答总结qa)

---

## 一、项目全景

### 1.1 定位与技术栈

一个**开箱即用的 Go 智能客服 Agent 框架**，核心 AI 能力构建在字节开源的
[CloudWeGo Eino](https://github.com/cloudwego/eino)（尤其 `adk` 包 = Agent Development Kit）之上。

| 层 | 技术 | 作用 |
|---|---|---|
| Agent 框架 | `eino/adk` | Agent 组装、Tool Calling 循环、流式、Checkpoint |
| HTTP | Gin | 双面 API（Biz 客服 + Admin 管理） |
| 检索 | RedisSearch / Milvus | 混合检索（BM25 + 向量 KNN） |
| 存储 | SQLite/MySQL (GORM) + Redis | 会话留存 + checkpoint + 任务队列 |
| 模型 | OpenAI 兼容 / 火山方舟 (ark) | Chat 与 Embedding 完全分离 |

### 1.2 一条请求的完整链路（先看这个，串起所有知识点）

用户发消息 → 最终回答，走的是这条「主链路」：

```
HTTP POST /sessions/:id/chat
  └─ handler/chat.go:120  Chat()
       ├─ decideChat()         上下文长度检查 + 交给 Agent 自主决策（chat.go:514）
       ├─ buildMessages()      拼 system prompt + 历史(≤10条) + 用户输入（chat.go:493）
       ├─ runner.Run(ctx, msgs, adk.WithCheckPointID(sessionID))   ← Agent 执行器（chat.go:195）
       │    └─ Eino ADK 内部：模型生成 → 决定调工具 → 执行 → 回填 → 再生成（循环）
       │         ├─ 调 knowledge_search 工具
       │         │    └─ rag.Service.SearchHits()  （graph/rag/pipeline.go:62）
       │         │         ├─ goroutine 并发跑 BM25 文本检索（SearchText）
       │         │         ├─ Embedding(query) → KNN 向量检索（Search）
       │         │         ├─ fuseRecords()  RRF 融合两路结果（pipeline.go:176）
       │         │         ├─ rerankRecords() 编辑距离相似度重排（pipeline.go:248）
       │         │         └─ filterRecords() 按知识库权限过滤 + 截断 topK
       │         └─ 调 transfer_human 工具 → HITL 中断 → 存 checkpoint
       └─ 消费 runner 的事件流 → sse.Writer 流式写出（chat.go:199-219）
```

转人工（HITL）续接：`handler/approval.go:52` 用
`runner.ResumeWithParams(sessionID, Targets{interruptID: approved})`
把人工审批结果塞回 Agent，从 checkpoint 恢复继续生成。

---

## 二、AI Agent 核心知识点清单

### 2.1 Agent 核心（最重要）

- **Agent = 模型 + 指令 + 工具**：`adk.NewChatModelAgent` 三样东西打包成 Agent（`agent/builder.go:25`）
- **Tool Calling 循环**：模型输出 `tool_calls` → 框架执行 → 结果回填 → 模型继续，直到模型决定直接回复。这是 ReAct 范式的工程实现
- **用函数签名自动生成工具 schema**：`utils.InferTool` 靠 Go 结构体的 `jsonschema:"description=..."` tag 生成 Function Calling 参数描述（`agent/tools/rag_search.go:30`）
- **模型自主决策 vs 显式路由**：本项目用「model-driven」——system prompt 里教模型什么时候调工具（`chat.go:502-508`），而非写死 if-else 路由。`graph/rag/intent.go`、`relation.go` 是另一套「显式 LLM 判断路由」写法（意图识别/追问消解），当前只在测试里用，是很好的对照材料

### 2.2 RAG 检索（技术含量最高，必啃）

- **混合检索 Hybrid Search**：BM25（关键词）+ 向量（语义）同时跑
- **RRF 融合（Reciprocal Rank Fusion）**：BM25 分数和余弦距离量纲不同，不能直接相加，用排名倒数融合（`pipeline.go:176-231`）
- **重排 Rerank**：本项目未上重排模型，用「编辑距离相似度」做轻量重排（`pipeline.go:248`），配合归一化（去标点/空格/大小写）
- **召回放大再截断**：先取 `topK*20` 候选，融合后截到 topK（`pipeline.go:67`）
- **中文分词补丁**：RedisSearch 中文解析是 AND 语义，`redissearch.go:215` 的 `buildRelaxedTextQuery` 用 3-gram 做召回扩张

### 2.3 向量库抽象

- **接口 + 多实现 + 工厂**：`vectorstore/store.go:22` 的 `Store` 接口，RedisSearch / Milvus 两实现，`factory.go` 按 `provider` 切换

### 2.4 模型层

- **Chat / Embedding 分离**：两个模型独立配置（`model/factory.go`）
- **Provider 抽象**：openai、openai_compatible、ark（火山方舟）统一到一个接口
- **动态热切换**：`settingsapp/dynamic_models.go` 的 `DynamicChatModel`/`DynamicEmbedder` 每次调用时才解析当前激活的 provider，改配置不用重启；`runtime_registry.go` 做带锁缓存 + 失效机制

### 2.5 HITL / 多轮状态

- **Checkpoint**：`checkpoint/redis_store.go` 把 Agent 中间状态（含中断点）存 Redis
- **Interrupt / Resume**：工具触发中断 → 外部审批 → `ResumeWithParams` 传回结果继续
- **多轮上下文管理**：历史消息硬上限 10 条（`chat.go:470`）+ 字符数上限，超限直接回默认回复

### 2.6 LLM 结构化输出（异步任务）

- **强制 JSON 输出 + 容错解析**：`classifier.go` 的 QA 分类打标，System 里要求「只输出 JSON」，解析时容错截取 `{...}`（`classifier.go:141`）
- **Redis 队列 + Worker Pool**：`BRPop` 阻塞消费 + 多 worker 并发（`classifier.go:45`）

### 2.7 可观测 & 工程化

- **Eino callbacks**：`observability/callback.go` 全局挂 OnStart/OnEnd/OnError 打日志
- **链路追踪**：`chat.go` 的 `chatTraceStep` 记录每步耗时/输入输出并落库
- **AI 错误收集**：`admin/ai_errors.go` + `reportAIError`，把 AI 各阶段错误集中入库
- **未回答问题池**：`knowledge/unanswered/service.go` 捕获「没答上来」的问题（`looksUnanswered` 关键词匹配），供人工补知识库，形成 RAG 生产闭环
- **分层架构**：handler → application/service → domain → store 接口，`bootstrap/wire.go` 手工依赖注入

---

## 三、核心代码导读（按优先级）

### T1 · 必读（AI 主链路，3 个文件）

- `internal/server/handler/chat.go` — 从 HTTP 到 SSE 的完整编排，Agent 事件流的消费方式
- `internal/agent/builder.go` + `internal/agent/tools/rag_search.go` — Agent 组装 + 工具定义（含 observer 设计）
- `internal/graph/rag/pipeline.go` — **混合检索 + RRF 融合 + 重排，全项目最有含金量的一段**

### T2 · 检索与模型（4 个文件）

- `internal/vectorstore/redissearch.go` — BM25 与 KNN 的具体实现
- `internal/vectorstore/store.go` — 接口抽象
- `internal/model/factory.go` — 模型工厂 + provider 抽象
- `internal/application/settings/dynamic_models.go` — 动态模型热切换

### T3 · Agent 生态（补齐视野）

- `internal/server/handler/approval.go` — HITL resume（配合 checkpoint 理解多轮状态）
- `internal/checkpoint/redis_store.go` — checkpoint 存储
- `internal/knowledge/qa/classifier.go` — LLM 结构化输出 + 异步 worker
- `internal/graph/rag/intent.go` + `relation.go` — 显式意图/追问判断（对照 model-driven 学）
- `internal/observability/callback.go` — 可观测

### T4 · 工程骨架（跑通整体再回头看）

- `internal/bootstrap/wire.go` — 整个系统的依赖装配图，读一遍就懂全貌
- `internal/config/config.go` — 配置分层与默认值

---

## 四、AI 客服问答完整流程（附代码块）

> 下面按「请求入口 → Agent 执行 → 工具调用 → RAG 检索 → 流式返回 → HITL」的时序，
> 贴出每一步对应的项目代码块。

### 4.0 整体时序图

```
前端 (Vue / H5 控件)
   │  POST /sessions/:id/chat   {message}
   ▼
Gin Router ──> ChatHandler.Chat (chat.go:120)
   │
   ├─ buildMessages:  [system(指令), system(工具规则), history≤10条, user]
   │
   ▼
runner.Run(ctx, messages, adk.WithCheckPointID(sessionID))   (chat.go:195)
   │
   ▼   ┌────────────────────────────────────────────┐
   │   │   Eino ADK 内部「模型 ⇄ 工具」循环          │
   │   │                                            │
   │   │  ┌──────────────────────────────────────┐  │
   │   │  │ ChatModel 生成                        │  │
   │   │  └──────────────┬───────────────────────┘  │
   │   │                 │ tool_calls=[{name:"knowledge_search", args:{query}}]
   │   │                 ▼                          │
   │   │  ┌──────────────────────────────────────┐  │
   │   │  │ 执行工具 knowledge_search            │  │
   │   │  │ (rag_search.go:30 → InferTool)        │  │
   │   │  └──────────────┬───────────────────────┘  │
   │   │                 │ service.SearchHits(query) │
   │   │                 ▼                          │
   │   │  ┌──────────────────────────────────────┐  │
   │   │  │  rag.Service.SearchHits (pipeline:62) │  │
   │   │  └──────────────┬───────────────────────┘  │
   │   │      ┌──────────┴──────────┐               │
   │   │      ▼                     ▼               │
   │   │ goroutine              embedder.EmbedStrings
   │   │ SearchText(BM25)       (query → 向量)
   │   │ RedisSearch            store.Search(KNN)
   │   │      │                     │               │
   │   │      ▼                     ▼               │
   │   │ textRecords           vectorRecords        │
   │   │      └──────────┬──────────┘               │
   │   │                 ▼                          │
   │   │   fuseRecords (RRF 融合)                   │
   │   │                 ▼                          │
   │   │   rerankRecords (编辑距离重排)             │
   │   │                 ▼                          │
   │   │   filterRecords (知识库过滤 + 截断 topK)    │
   │   │                 ▼                          │
   │   │   FormatHits → 拼成文本 (工具返回值)        │
   │   │                 │ 回填给模型                │
   │   │                 ▼                          │
   │   │  ChatModel 再次生成 (拿检索结果作答)        │
   │   │                 │ 若再调工具则再循环；       │
   │   │                 │ 若调 transfer_human 则中断 │
   │   └─────────────────┼──────────────────────────┘
   │                     │ 最终 assistant 文本
   ▼
SSE stream_chunk 事件流 → 前端打字机显示
```

### 4.1 请求入口：路由注册

`Chat` 挂在 3 条路由上（`internal/server/router.go`）：

| 路由 | 调用方 | 位置 |
|---|---|---|
| `POST /api/v1/biz/sessions/:id/chat` | H5 客服控件（外部接入） | `router.go:57` |
| `POST /api/v1/admin/playground/sessions/:id/chat` | 管理后台「测试客服」 | `router.go:112` |
| `POST /api/v1/gm/ai-customer-service/test/sessions/:id/chat` | GM 控制台测试 | `router.go:214` |

```go
// router.go:57
sessions.POST("/:id/chat", middleware.SessionSerial(), deps.ChatHandler.Chat)

// router.go:112（playground，JWT 鉴权）
playground.POST("/sessions/:id/chat", deps.ChatHandler.Chat)
```

### 4.2 Chat Handler 主流程

`internal/server/handler/chat.go`：

```go
func (h *ChatHandler) Chat(c *gin.Context) {
    start := time.Now()
    sessionID := c.Param("id")
    var req ChatRequest
    if err := c.ShouldBindJSON(&req); err != nil { /* 400 */ }

    // 校验会话 + 取历史消息
    history, _ := h.sessionStore.GetMessages(c.Request.Context(), sessionID)
    history = h.limitAgentHistory(c.Request.Context(), history)   // 最多 10 条

    // ① 上下文长度检查；超限直接回默认回复（走 fallback 分支）
    decision, err := h.decideChat(c.Request.Context(), history, req.Message)

    // ② fallback 路径：decision.Reply != "" 时直接写一个 message 事件返回
    if decision.Reply != "" {
        h.sseWriter.WriteEvent(c.Writer, sse.Event{Type: "message", Content: decision.Reply})
        return
    }

    // ③ 正常路径：拼 messages（system + 历史 + 用户）
    userMsg, messages := h.buildMessages(c.Request.Context(), history, req.Message, decision)

    // ④ 设置 SSE 响应头（关键：不是 JSON，是事件流）
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("X-Accel-Buffering", "no")

    // ⑤ 注入「工具结果观察者」，用于溯源/元数据（避免二次检索）
    searches := &toolSearchCollector{}
    runCtx := agenttools.WithSearchObserver(c.Request.Context(), searches.Observe)

    // ⑥ 执行 Agent，按 sessionID 存 checkpoint
    iter := h.runner.Run(runCtx, messages, adk.WithCheckPointID(sessionID))

    // ⑦ 消费 Agent 事件流，逐个写成 SSE 帧并 flush
    for {
        event, ok := iter.Next()
        if !ok { break }
        h.sseWriter.WriteWithObserver(c.Writer, event, ...)
        c.Writer.Flush()
    }

    // ⑧ 收尾：记录工具调用、捕获未回答问题、持久化 assistant 消息
    decision.SearchQuery, decision.SearchHits = searches.Snapshot()
    decision.KnowledgeShared = containsToolCall(assistantToolCalls, "knowledge_search")
    decision.NeedHuman = containsToolCall(assistantToolCalls, "transfer_human")
    h.captureUnanswered(...)
    h.sessionStore.AppendMessages(..., schema.AssistantMessage(content, assistantToolCalls))
}
```

### 4.3 拼消息：buildMessages

```go
// chat.go:493
func (h *ChatHandler) buildMessages(ctx, history, content, _) (*schema.Message, []*schema.Message) {
    userMsg := schema.UserMessage(content)
    messages := ...
    // ① 可配置的全局 instruction（DB 里存，可后台改）
    if instruction := h.settings.GetPromptContent(ctx, admin.PromptAgentInstruction, ""); instruction != "" {
        messages = append(messages, schema.SystemMessage(instruction))
    }
    // ② 工具使用规则（教模型何时调 knowledge_search / transfer_human）
    messages = append(messages, schema.SystemMessage(`由你判断本轮是否需要使用工具：
- 涉及产品功能、操作方法、账号、充值、规则、售后等需要事实依据的问题，调用 knowledge_search。
- knowledge_search 会返回多条候选 QA。你必须比较相关性和答案内容，只采用能直接支持本轮回答的候选。
- 知识库证据足够时，用自然、简短的客服口吻组织最终答复；证据不足时明确说明暂时没有查到，不要编造。
- 寒暄、感谢等无需业务事实的问题可以直接回复；用户明确要求人工客服时调用 transfer_human。
- 你会收到最近最多 10 条历史消息。历史对话只用于理解本轮问题和消解指代，最终只回答本轮诉求。`))
    // ③ 历史消息
    messages = append(messages, history...)
    // ④ 当前用户消息
    messages = append(messages, userMsg)
    return userMsg, messages
}
```

> 关键：这是「model-driven」的核心——**用 prompt 教模型自主决定何时检索/转人工**，而不是代码里写死路由。

### 4.4 Agent 构建（模型 + 指令 + 工具）

`internal/agent/builder.go`：

```go
func NewCustomerServiceAgent(ctx, chatModel, ragService, instruction) (adk.Agent, error) {
    if instruction == "" { instruction = CustomerServiceInstruction }
    ragTool, _ := agenttools.NewRAGSearchTool(ragService)   // 把 RAG 检索包装成工具

    return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "CustomerServiceAgent",
        Description: "智能客服助手，可检索知识库",
        Instruction: instruction,      // 系统指令（可后台覆盖）
        Model:       chatModel,        // Chat 模型
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools: []tool.BaseTool{ragTool},   // 只挂一个检索工具
            },
        },
    })
}
```

`internal/agent/runner.go`（执行器，管流式 + checkpoint）：

```go
func NewRunner(ctx, agent, store) *Runner {
    return &Runner{inner: adk.NewRunner(ctx, adk.RunnerConfig{
        Agent:           agent,
        EnableStreaming: true,
        CheckPointStore: store,   // Redis checkpoint，支撑 HITL 中断恢复
    })}
}
```

### 4.5 工具定义：knowledge_search

`internal/agent/tools/rag_search.go`：

```go
type SearchInput struct {
    Query string `json:"query" jsonschema:"description=用户问题或检索关键词"`
}

func NewRAGSearchTool(service *rag.Service) (tool.InvokableTool, error) {
    // InferTool：用函数签名 + jsonschema tag 自动生成 Function Calling 的 schema
    return utils.InferTool("knowledge_search", "检索 QA 知识库并返回多条候选问答（含标准答案）",
        func(ctx context.Context, input *SearchInput) (string, error) {
            if input.Query == "" { return "", fmt.Errorf("query is required") }
            hits, err := service.SearchHits(ctx, input.Query)   // ← 进入 RAG 检索
            if err != nil { return "", err }
            // observer：把检索结果回传给 chat handler 做溯源（context 传递，避免二次检索）
            if observer, ok := ctx.Value(searchObserverKey{}).(func(string, []rag.SearchHit)); ok {
                observer(input.Query, hits)
            }
            if len(hits) == 0 { return "未找到相关 QA 知识库内容。", nil }
            return rag.FormatHits(hits), nil   // 拼成文本，作为工具返回值给模型
        })
}
```

### 4.6 RAG 检索：SearchHits + RRF 融合

`internal/graph/rag/pipeline.go`（核心，逐段）：

```go
// ① 检索编排器：并发双路检索 → 融合 → 重排 → 过滤
func (s *Service) SearchHits(ctx context.Context, query string) ([]SearchHit, error) {
    query = strings.TrimSpace(query)
    if query == "" { return nil, nil }

    // 召回放大：候选池 = topK*20，最少 100 条
    candidateLimit := s.topK * 20
    if candidateLimit < 100 { candidateLimit = 100 }

    // 文本检索放 goroutine（与向量检索并发）
    textResultCh := make(chan textSearchResult, 1)
    go func() {
        records, err := s.store.SearchText(ctx, query, candidateLimit)   // BM25
        textResultCh <- textSearchResult{records: records, err: err}
    }()

    // 向量检索：先 embed 查询词，再 KNN
    vecs, err := s.embedder.EmbedStrings(ctx, []string{query})
    var vectorRecords []vectorstore.Record
    vectorErr := err
    if vectorErr == nil {
        qv := make([]float32, len(vecs[0]))
        for i, v := range vecs[0] { qv[i] = float32(v) }
        vectorRecords, vectorErr = s.store.Search(ctx, qv, candidateLimit)
    }
    textResult := <-textResultCh

    // 容错：Milvus 不支持文本检索 → 降级纯向量；任一路失败 → 用另一路
    textSearchUnsupported := errors.Is(textResult.err, vectorstore.ErrTextSearchUnsupported)
    ...
    if textResult.err != nil { return s.filterRecords(ctx, rerankRecords(query, fuseRecords(nil, vectorRecords, candidateLimit))) }
    if vectorErr != nil      { return s.filterRecords(ctx, rerankRecords(query, fuseRecords(textResult.records, nil, candidateLimit))) }

    // 正常路径：融合 → 重排 → 过滤
    records := rerankRecords(query, fuseRecords(textResult.records, vectorRecords, candidateLimit))
    return s.filterRecords(ctx, records)
}
```

```go
// ② RRF 融合（全文精华）：用排名倒数融合，解决 BM25 与余弦量纲不统一问题
const (
    rrfRankConstant  = 60.0   // RRF 的 k 值
    textRankWeight   = 1.25   // 文本路权重
    vectorRankWeight = 1.0    // 向量路权重
)

func fuseRecords(textRecords, vectorRecords []vectorstore.Record, limit int) []vectorstore.Record {
    byID := make(map[string]*fusedRecord)
    add := func(records []vectorstore.Record, weight float64) {
        for i, record := range records {
            rank := i + 1   // 第 1 名 rank=1
            entry := byID[record.ID]
            if entry == nil {
                entry = &fusedRecord{record: record, bestRank: rank}
                byID[record.ID] = entry
            } else {
                enrichRecord(&entry.record, record)
                if rank < entry.bestRank { entry.bestRank = rank }
            }
            // 关键公式：weight / (k + rank)，两条路命中则累加
            entry.score += weight / (rrfRankConstant + float64(rank))
        }
    }
    add(textRecords, textRankWeight)    // 文本路 1.25
    add(vectorRecords, vectorRankWeight) // 向量路 1.0

    // 归一化到 [0,1]
    maxScore := (textRankWeight + vectorRankWeight) / (rrfRankConstant + 1)  // = 2.25/61
    for _, entry := range byID {
        entry.record.Score = entry.score / maxScore
    }
    sort.SliceStable(...)  // score 高优先 → bestRank 小优先 → ID 兜底
    if len(entries) > limit { entries = entries[:limit] }
    ...
}
```

```go
// ③ 轻量重排：编辑距离相似度（无重排模型）
func rerankRecords(query string, records []vectorstore.Record) []vectorstore.Record {
    normalizedQuery := normalizeSearchText(query)   // 去标点/空格/大小写
    for i := range records {
        similarity := editSimilarity(normalizedQuery, normalizeSearchText(records[i].Question))
        if similarity < 0.6 { continue }
        records[i].Score = math.Min(1, records[i].Score + 0.35*similarity)   // 加分
    }
    sort.SliceStable(...)
    return records
}
```

```go
// ④ 知识库过滤 + 截断 topK（多租户权限隔离）
func (s *Service) filterRecords(ctx, records) ([]SearchHit, error) {
    if s.filterKnowledgeBase == nil {   // 单知识库：直接截断
        if len(records) > s.topK { records = records[:s.topK] }
        return searchHitsFromRecords(records), nil
    }
    allowed, _ := s.filterKnowledgeBase(ctx, "", ids)   // 只保留当前租户知识库的 QA
    filtered := ...
    for _, rec := range records {
        if _, ok := allowed[rec.ID]; !ok { continue }
        filtered = append(filtered, rec)
        if len(filtered) >= s.topK { break }
    }
    ...
}
```

```go
// ⑤ 把检索结果拼成给模型看的文本
func FormatHits(hits []SearchHit) string {
    // 输出形如：
    // [1] 分类: 售后 标签: 退款, 退货
    // Q: 你们的退款政策是什么？
    // A: 支持 7 天无理由退款...
    // Score: 0.9800
}
```

### 4.7 Embedding 模型调用链（5 层）

**本项目仓库只有「调用」，实现都在 Eino 依赖库里（Go 模块缓存）。**

```
【本项目仓库】业务调用方
  qa/service.go:65        embedder.EmbedStrings(question)   ← 入库侧
  pipeline.go:85          embedder.EmbedStrings(query)      ← 检索侧
        │ 调用接口方法
        ▼
【Eino 核心】接口定义（只有声明）
  eino@v0.9.12/components/embedding/interface.go:37
      type Embedder interface { EmbedStrings(ctx, texts, ...) ([][]float64, error) }
        ▼
【eino-ext】包装实现（OpenAI 版）
  eino-ext/components/embedding/openai@.../embedding.go:129
      func (e *Embedder) EmbedStrings(...) { return e.cli.EmbedStrings(...) }
        ▼
【eino-ext/libs】真正的 HTTP 客户端
  eino-ext/libs/acl/openai@v0.1.17/embedding.go:151
      resp, err := e.cli.CreateEmbeddings(ctx, *req)   // ★ 真正发 HTTP
        ▼
【第三方 SDK】go-openai → POST /v1/embeddings
```

**入库侧调用**（`internal/knowledge/qa/service.go:63`，只 embed 问题不 embed 答案）：

```go
func (s *Service) embedQuestion(ctx context.Context, question string) ([]float32, error) {
    vecs, err := s.embedder.EmbedStrings(ctx, []string{question})   // ← 真正调用 Embedding 模型
    if err != nil { return nil, err }
    out := make([]float32, len(vecs[0]))
    for i, v := range vecs[0] { out[i] = float32(v) }   // float64 → float32
    return out, nil
}
```

**模型工厂**（`internal/model/factory.go:40`，按 provider 挑实现）：

```go
func NewEmbedder(ctx, cfg config.EmbeddingConfig) (embedding.Embedder, error) {
    switch strings.ToLower(cfg.Provider) {
    case "openai":
        return openaiEmbedding.NewEmbedder(ctx, &openaiEmbedding.EmbeddingConfig{...})
    case "ark", "volc", "火山", "火山方舟":
        return arkEmbedding.NewEmbedder(ctx, &arkEmbedding.EmbeddingConfig{...})
    case "ark_multimodal", ...:   // 多模态 embedding
        ...
    }
}
```

> 生产运行时 `bootstrap/wire.go:71` 注入的是 `runtime.DynamicEmbedder()`（`dynamic_models.go:62`），
> 每次 `EmbedStrings` 时才解析当前激活的 provider，改配置不用重启。

### 4.8 向量库（RAG 的存储层）

**接口**（`internal/vectorstore/store.go:22`）：

```go
type Store interface {
    Ensure(ctx context.Context, dim int) error
    Upsert(ctx context.Context, rec Record) error
    Delete(ctx context.Context, id string) error
    SearchText(ctx context.Context, query string, topK int) ([]Record, error)   // BM25
    Search(ctx context.Context, queryVector []float32, topK int) ([]Record, error)  // 向量 KNN
    Close() error
}
```

**工厂**（`internal/vectorstore/factory.go:15`）：按 `vector_store.provider` 切换。

**两种实现对比**：

| | RedisSearch（默认） | Milvus（生产级） |
|---|---|---|
| 定位 | 轻量，复用 Redis | 专业向量库 |
| 文本检索(BM25) | ✅ | ❌（`SearchText` 返回 `ErrTextSearchUnsupported`） |
| 向量索引 | Flat + COSINE | HNSW + COSINE/L2/IP |

**入库主流程**（`internal/knowledge/qa/service.go:93`）：

```go
func (s *Service) Create(ctx, req) (*Item, error) {
    s.ensureIndex(ctx)                    // ① 首次建索引（懒加载）
    item := &Item{ ID: uuid.NewString(), ... }
    s.meta.Save(ctx, item)                // ② 先存元数据（SQLite/MySQL 或 Redis）
    s.upsertItem(ctx, item)               // ③ 再 embed + 写向量库，失败回滚元数据
    if status == ClassifyPending { s.EnqueueClassify(ctx, item.ID) }  // ④ 缺分类进异步队列
    return item, nil
}
```

**双写结构（核心架构点）**：每条 QA 同时存两处，职责分离：

| 存储 | 内容 | 用途 |
|---|---|---|
| 元数据 `meta`（SQLite/MySQL 或 Redis） | 完整 Item（分类/标签/状态/时间） | 管理后台列表/搜索/CRUD |
| 向量库 `store`（RedisSearch/Milvus） | question/answer/category/tags + 向量 | 语义检索 |

### 4.9 HITL 转人工（Interrupt / Resume）

`internal/server/handler/approval.go`：

```go
func (h *ApprovalHandler) Approve(c *gin.Context) {
    sessionID := c.Param("id")
    var req ApprovalRequest   // {interrupt_id, approved}

    // 把人工审批结果塞回 Agent，从 checkpoint 恢复继续执行
    iter, err := h.runner.ResumeWithParams(c.Request.Context(), sessionID, &adk.ResumeParams{
        Targets: map[string]any{ req.InterruptID: req.Approved },
    })
    ...
    // 继续消费事件流，SSE 写出
    for { event, ok := iter.Next(); ... h.sseWriter.WriteWithObserver(...) }
}
```

`transfer_human` 工具（`rag_search.go:53`）触发中断后，Agent 状态存入
`internal/checkpoint/redis_store.go`（Redis），等审批接口 `ResumeWithParams` 恢复。

### 4.10 SSE 流式返回

`pkg/sse/writer.go`（真正把内容写成 SSE 帧的地方）：

```go
func writeEvent(writer io.Writer, event Event) error {
    data, _ := json.Marshal(event)
    _, err := fmt.Fprintf(writer, "data: %s\n\n", data)   // SSE 帧格式
    return err
}
```

事件类型（`Event.Type`）：
- `stream_chunk` — 模型流式吐出的 token（打字机效果的内容来源）
- `message` — 一条完整消息（fallback 路径用）
- `action` + `ActionType: interrupted` — HITL 中断
- `error` — 错误

前端收到的是一连串 `data: {"type":"stream_chunk","content":"..."}` 帧，用 `EventSource` 或
`fetch` + `ReadableStream` 拼起来显示。

> **为什么「没有返回内容」**：`Chat` 不返回 JSON（没有 `response.OK`），而是把 HTTP 连接当管道，
> 用 SSE 一段段推送。对比 `Ask`（`/api/v1/biz/chat/ask`）才是传统 JSON 一次性返回
> （`chat.go:336` 的 `response.OK(c, gin.H{...})`）。

---

## 五、会话问答总结（Q&A）

### Q1：梳理项目 + 知识点 + 从哪入手学习 AI Agent

**答**：项目是「开箱即用的 Go 智能客服 Agent 框架」，基于 CloudWeGo Eino（ADK）+ Gin。
核心知识点分 7 块：Agent 核心（Tool Calling 循环）、RAG 检索（混合检索 + RRF 融合 + 重排）、
向量库抽象、模型层（Chat/Embedding 分离 + 动态热切换）、HITL/多轮状态、LLM 结构化输出（异步任务）、
可观测与工程化。

**入手点**：不要从 `main.go` 进，直接从 `internal/server/handler/chat.go` 的 `Chat()` 函数进，
顺着 `runner.Run → 工具 → RAG 检索` 这条链路读。

### Q2：chat 方法是干嘛的？是前端调用的吗？好像没返回内容？

**答**：`Chat` 是「流式对话」接口，**有返回内容，只是用 SSE 而非 JSON**。挂了 3 条路由
（Biz / playground / GM 测试）。它把响应头设成 `text/event-stream`，循环消费 Agent 事件、
写成 `data: {...}\n\n` 帧并 flush。内容分散在无数 `stream_chunk` 事件里，前端边收边显示（打字机效果）。
对比：`Ask` 才是 JSON 一次性返回。

### Q3：带读 pipeline.go 混合检索/融合 + 时序图 + RAG 代码解读

**答**：见本文第四章完整流程。核心是 `SearchHits`（并发双路检索）→ `fuseRecords`（RRF 融合，
公式 `weight/(k+rank)`，k=60，文本权重 1.25 / 向量 1.0）→ `rerankRecords`（编辑距离重排）→
`filterRecords`（知识库过滤 + topK 截断）。入库侧：`qa/service.go` 的 `Create`（先元数据后向量、
失败回滚），**只 embed 问题不 embed 答案**。

### Q4：向量数据库用什么？

**答**：两种，配置切换。默认 **RedisSearch**（复用 Redis，支持 BM25 + 向量混合检索），生产级可选
**Milvus**（HNSW 索引，但 `SearchText` 不支持 → 降级纯向量）。由 `vector_store.provider` 决定，
`factory.go` 工厂切换，上层通过 `Store` 接口无痛切换。

### Q5：哪里调用 Embedding 模型？

**答**：业务上只有 **两处** `embedder.EmbedStrings`：
① 入库侧 `qa/service.go:64`（embed QA 问题）；② 检索侧 `pipeline.go:85`（embed 用户查询）。
另有第 ③ 处 legacy 文档索引器 `indexer/service.go`（走 Eino 内置 indexer，非 QA 主流程）。

### Q6：EmbedStrings 只是接口方法，实现在哪里？

**答**：实现在 **Eino 依赖库**（Go 模块缓存），不在本项目仓库。5 层调用链：
① 项目业务代码 → ② Eino 核心接口定义（`eino@v0.9.12/components/embedding/interface.go`）→
③ eino-ext 包装实现（`eino-ext/components/embedding/openai@.../embedding.go`）→
④ 真正 HTTP 客户端（`eino-ext/libs/acl/openai@v0.1.17/embedding.go:151` 的 `CreateEmbeddings`）→
⑤ 第三方 `go-openai` 发 `POST /embeddings`。模块缓存只读，要改用 `go mod vendor`。

### Q7：`//go:generate mockgen ...` 这行什么意思？

**答**：Go 的代码生成指令。运行 `go generate` 时执行 `mockgen` 工具，根据 `interface.go` 的
`Embedder` 接口自动生成 `MockEmbedder`（假实现），供测试替换真模型、不发 HTTP、稳定可控。
参数：`-destination` 输出路径（相对源文件目录）、`--package` 包名、`-source` 输入接口文件。
项目自己的测试没直接用 mockgen，而是手写了等价的 `staticEmbedder` stub。

---

## 附：建议学习路线

1. **跑通 + 跟链路（半天）**：跑起项目，用「测试客服」发一条命中知识库的问题，在 `chat.go:120`、
   `rag_search.go:34`、`pipeline.go:62` 打断点，亲眼看到工具被调、检索结果回填。
2. **啃 Agent 循环 + RAG（重点，1-2 天）**：精读 `builder.go`、`rag_search.go`、`pipeline.go`
   的 `fuseRecords`（RRF）和 `rerankRecords`。
3. **理解多轮 + HITL（1 天）**：读 `approval.go` + `checkpoint/redis_store.go`，对比
   `intent.go`/`relation.go`（显式路由）与 system prompt（model-driven）。
4. **工程化扩展（按兴趣）**：`dynamic_models.go`（模型热切换）、`classifier.go`（异步 LLM 任务）、
   `unanswered`（RAG 闭环）。
