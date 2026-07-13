# 对外客服接口文档

> **实现对齐说明（2026-07-10）**：当同时传入 `X-Caller-Token` 和 `Authorization` 时，服务端当前优先使用 `X-Caller-Token`；接入方应始终只传一种鉴权头，推荐 `Authorization`。`Idempotency-Key` 与 `client_message_id` 当前只用于请求/消息关联和日志，不提供服务端去重或重复请求回放；重试时请由接入方自行避免重复提交。会话 ID 是敏感资源，接入方应妥善保存，且不应向非所属用户暴露。

> **历史消息说明**：`GET /sessions/:id/history` 的每条消息除 `role`、`content` 外，实际还可能返回 `id`、`request_id`、`client_message_id`、`extra` 与 `created_at` 等元数据。

本文面向外部接入方，按当前唯一上线方案说明：

- 仅支持 `caller token`
- 支持有状态会话问答
- 接口方自行决定继续既有会话还是重新创建新会话
- 提供无状态问答作为补充能力
- 说明每个请求头和请求字段的是否必填、用途与示例

当前知识库为全局共享，不按租户、渠道、调用方隔离。`caller_user_id`、`channel_id`、`tenant_id` 仅用于调用方身份、会话归属、日志、统计与检索索引。

## 基本信息

- 基础前缀：`/api/v1/biz`
- 响应格式：除 SSE 流接口外，统一返回：

```json
{
  "code": 200,
  "message": "ok",
  "data": {}
}
```

- 推荐请求头：

```text
Content-Type: application/json
Authorization: Bearer <caller_token>
X-Caller-User-Id: user_10001
X-Channel-Id: wechat
X-Tenant-Id: acme
X-Request-ID: req_20260708_001
Idempotency-Key: idem_20260708_001
```

## 鉴权说明

当前只支持 `caller token`：

- `Authorization: Bearer <caller_token>`
- 或 `X-Caller-Token: <caller_token>`

推荐统一使用：

```text
Authorization: Bearer <caller_token>
```

## 请求头说明

### `Authorization`

- 是否必填：是
- 格式：`Bearer <caller_token>`
- 用途：调用方身份认证，决定请求是否允许访问 Biz 接口
- 备注：推荐唯一使用这一种鉴权方式

### `X-Caller-Token`

- 是否必填：否
- 用途：`caller token` 的备用传法
- 备注：只有在不方便使用 `Authorization` 时才建议使用；如果传了 `Authorization`，优先直接统一 `Authorization`

### `X-Caller-User-Id`

- 是否必填：强烈建议填写
- 类型：字符串
- 用途：标识调用方侧的最终用户 ID，用于会话归属、日志、查询筛选、统计和用户级限流
- 示例：`user_10001`

### `X-Channel-Id`

- 是否必填：建议填写
- 类型：字符串
- 用途：标识接入渠道，例如 App、H5、小程序、企业微信、公众号等
- 示例：`wechat`、`h5`、`app`

### `X-Tenant-Id`

- 是否必填：按你们业务决定
- 类型：字符串
- 用途：标识业务租户、品牌、商户或业务线；当前不参与知识库隔离，只用于归属、统计和查询
- 示例：`acme`

### `X-Request-ID`

- 是否必填：强烈建议填写
- 类型：字符串
- 用途：请求追踪 ID，便于排障、链路追踪、问题回放
- 示例：`req_20260708_001`

### `Idempotency-Key`

- 是否必填：建议填写
- 类型：字符串
- 用途：请求幂等标识，尤其适合网络重试时避免重复处理
- 示例：`idem_20260708_001`

## 用户级短期凭证

生产渠道先在 GM 创建接入应用，取得 `app_id` 和 `app_secret`。`app_secret` 只能由渠道服务端保存；渠道服务端确认当前登录用户后，调用 `POST /api/v1/biz/auth/token` 换取默认 15 分钟有效的用户 Token。请求字段为 `app_id`、`app_secret`、`caller_user_id`，以及可选的 `channel_id`、`tenant_id`。

后续 Biz API 使用 `Authorization: Bearer <access_token>`。用户、渠道和租户均以 Token 内绑定的信息为准，调用方不能通过请求头切换身份。完整的 H5 接入流程见 [`h5-chat-widget.md`](h5-chat-widget.md)。旧 Caller Token 鉴权暂时保留兼容，新渠道不应继续使用。

## 会话模型

接口方可以自行决定是否继续上下文：

### 方案 A：继续已有会话

适用于：

- 用户在同一个客服窗口继续追问
- 需要保留前文上下文
- 希望模型基于上轮对话继续回答

做法：

1. 首次进入时调用 `POST /sessions` 创建会话
2. 保存返回的 `session_id`
3. 后续继续提问时，始终调用 `POST /sessions/:id/chat`

### 方案 B：重新开始新会话

适用于：

- 用户主动点击“新对话”
- 上下文不希望污染下一轮回答
- 不同业务单据、不同问题域、不同咨询入口

做法：

1. 再次调用 `POST /sessions`
2. 使用新的 `session_id`
3. 后续对话都走新的 `session_id`

一句话原则：

- 想保留上下文，就继续用原来的 `session_id`
- 想清空上下文，就重新创建一个 `session_id`

## 接口清单

### 1. 创建会话

`POST /api/v1/biz/sessions`

字段说明：

- `caller_user_id`
  - 是否必填：否
  - 类型：字符串
  - 用途：最终用户 ID；如果 body 不传，可从 `X-Caller-User-Id` 读取
- `channel_id`
  - 是否必填：否
  - 类型：字符串
  - 用途：渠道标识；如果 body 不传，可从 `X-Channel-Id` 读取
- `tenant_id`
  - 是否必填：否
  - 类型：字符串
  - 用途：租户/业务线标识；如果 body 不传，可从 `X-Tenant-Id` 读取
- `meta`
  - 是否必填：否
  - 类型：对象
  - 用途：透传调用方的业务扩展信息，例如订单号、昵称、场景、设备信息

请求体：

```json
{
  "caller_user_id": "user_10001",
  "channel_id": "wechat",
  "tenant_id": "acme",
  "meta": {
    "nickname": "张三",
    "mobile": "138****0000",
    "scene": "after_sale"
  }
}
```

说明：

- `caller_user_id`、`channel_id`、`tenant_id` 也可放请求头
- 如果 body 和 header 同时传，以 body 中显式传入的值为准
- `meta` 用于透传业务扩展字段

响应示例：

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "id": "66ee0c4e-7f8b-4d35-99da-2a72ef97d901",
    "external_user_id": "user_10001",
    "channel_id": "wechat",
    "tenant_id": "acme",
    "caller_token_id": "7af4c8c7-9c6f-4e11-aeba-260f2d9b0b54",
    "status": "active",
    "meta": {
      "nickname": "张三",
      "mobile": "138****0000",
      "scene": "after_sale"
    },
    "created_at": "2026-07-08T18:30:00+08:00",
    "updated_at": "2026-07-08T18:30:00+08:00"
  }
}
```

`curl` 示例：

```bash
curl -X POST "http://localhost:8080/api/v1/biz/sessions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ct_xxx" \
  -H "X-Caller-User-Id: user_10001" \
  -H "X-Channel-Id: wechat" \
  -H "X-Tenant-Id: acme" \
  -d '{
    "meta": {
      "scene": "after_sale",
      "order_no": "SO20260708001"
    }
  }'
```

### 2. 获取会话信息

`GET /api/v1/biz/sessions/:id`

说明：

- `:id` 为会话 ID
- 是否必填：是
- 用途：查询会话元信息，不返回完整消息内容

响应示例：

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "id": "66ee0c4e-7f8b-4d35-99da-2a72ef97d901",
    "external_user_id": "user_10001",
    "channel_id": "wechat",
    "tenant_id": "acme",
    "status": "active",
    "created_at": "2026-07-08T18:30:00+08:00",
    "updated_at": "2026-07-08T18:31:22+08:00"
  }
}
```

### 3. 查询某个用户的历史会话列表

`GET /api/v1/biz/users/:callerUserID/sessions`

说明：

- `:callerUserID` 为调用方用户 ID
- 是否必填：是
- 用途：给渠道方查询某个用户的历史会话列表，便于展示历史对话入口
- 返回内容包含会话标题、状态、消息数、最后一条摘要和最后活跃时间

查询参数：

- `page`
  - 是否必填：否
  - 默认值：`1`
  - 用途：分页页码
- `size`
  - 是否必填：否
  - 默认值：`20`
  - 用途：每页条数
- `channel_id`
  - 是否必填：否
  - 用途：按渠道过滤；不传时优先使用请求头里的 `X-Channel-Id`
- `tenant_id`
  - 是否必填：否
  - 用途：按租户过滤；不传时优先使用请求头里的 `X-Tenant-Id`

响应示例：

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "total": 2,
    "caller_user_id": "user_10001",
    "items": [
      {
        "id": "66ee0c4e-7f8b-4d35-99da-2a72ef97d901",
        "title": "退款多久到账？",
        "external_user_id": "user_10001",
        "channel_id": "wechat",
        "tenant_id": "acme",
        "caller_token_id": "7af4c8c7-9c6f-4e11-aeba-260f2d9b0b54",
        "status": "active",
        "message_count": 6,
        "last_message_role": "assistant",
        "last_message_preview": "通常会在 1-3 个工作日内原路退回。",
        "last_message_at": "2026-07-08T18:45:00+08:00",
        "created_at": "2026-07-08T18:30:00+08:00",
        "updated_at": "2026-07-08T18:45:00+08:00"
      }
    ]
  }
}
```

`curl` 示例：

```bash
curl "http://localhost:8080/api/v1/biz/users/user_10001/sessions?page=1&size=20" \
  -H "Authorization: Bearer ct_xxx" \
  -H "X-Channel-Id: wechat" \
  -H "X-Tenant-Id: acme"
```

### 4. 获取会话历史

`GET /api/v1/biz/sessions/:id/history`

说明：

- `:id` 为会话 ID
- 是否必填：是
- 用途：获取该会话已经留存的消息历史

响应示例：

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "messages": [
      {
        "role": "user",
        "content": "退款多久到账？"
      },
      {
        "role": "assistant",
        "content": "通常会在 1-3 个工作日内原路退回。"
      }
    ]
  }
}
```

### 5. 关闭会话

`DELETE /api/v1/biz/sessions/:id`

说明：

- 会话关闭后，历史消息仍保留
- 适合在接口方侧将会话标记为已结束

### 6. 会话内继续问答

`POST /api/v1/biz/sessions/:id/chat`

这是**有状态问答接口**，会自动带上当前 `session_id` 下的历史消息继续回答。

字段说明：

- `message`
  - 是否必填：是
  - 类型：字符串
  - 用途：用户本轮输入的问题
- `client_message_id`
  - 是否必填：建议填写
  - 类型：字符串
  - 用途：客户端消息唯一 ID，用于重试幂等、日志定位、排查重复提交

请求体：

```json
{
  "message": "那如果退款失败怎么办？",
  "client_message_id": "msg_20260708_0002"
}
```

说明：

- `client_message_id` 建议由接入方生成，用于幂等控制
- 同一 `session_id` 下重复提交同一个 `client_message_id`，服务端会尽量避免重复写入用户消息
- 如果当前会话还没有标题，第一次真实用户发言时会自动生成标题，供历史会话列表展示
- 如果你希望“另起一轮，不继承前文”，不要复用这个 `session_id`，而是重新创建会话

`curl` 示例：

```bash
curl -N -X POST "http://localhost:8080/api/v1/biz/sessions/66ee0c4e-7f8b-4d35-99da-2a72ef97d901/chat" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ct_xxx" \
  -H "X-Caller-User-Id: user_10001" \
  -H "X-Channel-Id: wechat" \
  -H "X-Tenant-Id: acme" \
  -H "X-Request-ID: req_20260708_0002" \
  -H "Idempotency-Key: idem_20260708_0002" \
  -d '{
    "message": "那如果退款失败怎么办？",
    "client_message_id": "msg_20260708_0002"
  }'
```

## SSE 响应说明

`POST /sessions/:id/chat` 返回 `text/event-stream`。

每一条数据格式类似：

```text
data: {"type":"stream_chunk","content":"通常"}

data: {"type":"stream_chunk","content":"会在 1-3 个工作日内原路退回。"}

data: {"type":"action","action_type":"exit","content":"agent execution completed"}
```

常见事件类型：

- `stream_chunk`：助手流式文本片段
- `message`：完整消息
- `tool_result`：工具调用结果
- `tool_result_chunk`：工具结果流片段
- `tool_calls`：工具调用信息
- `action`：动作事件，如 `exit`、`interrupted`
- `error`：异常

### SSE 读取建议

接口方通常按下面规则处理：

1. 把多个 `stream_chunk` 片段拼成最终回答
2. 收到 `action_type=exit` 时，视为本轮完成
3. 收到 `action_type=interrupted` 时，说明需要人工确认或人工介入

说明：

- `POST /sessions/:id/chat` 的主要结果是 SSE 事件流，不是一次性 JSON 包
- 若需要回看完整结果，可后续调用 `GET /sessions/:id/history`

SSE 示例：

```text
data: {"type":"stream_chunk","content":"退款"}

data: {"type":"stream_chunk","content":"通常会在 1-3 个工作日内原路退回。"}

data: {"type":"action","action_type":"exit","content":"agent execution completed"}
```

## HITL 审批接口

`POST /api/v1/biz/sessions/:id/approve`

当 SSE 中出现：

```text
data: {"type":"action","action_type":"interrupted","interrupt_id":"interrupt_xxx","content":"..."}
```

说明当前会话进入人工确认节点。此时调用：

请求体：

```json
{
  "interrupt_id": "interrupt_xxx",
  "approved": true
}
```

字段说明：

- `interrupt_id`
  - 是否必填：是
  - 类型：字符串
  - 用途：SSE 中 `action_type=interrupted` 返回的中断 ID
- `approved`
  - 是否必填：是
  - 类型：布尔值
  - 用途：是否同意继续执行；`true` 表示继续，`false` 表示拒绝

示例：

```bash
curl -N -X POST "http://localhost:8080/api/v1/biz/sessions/66ee0c4e-7f8b-4d35-99da-2a72ef97d901/approve" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ct_xxx" \
  -d '{
    "interrupt_id": "interrupt_xxx",
    "approved": true
  }'
```

## 无状态问答接口

`POST /api/v1/biz/chat/ask`

这个接口**不会持久化会话上下文**，适合：

- FAQ 单轮问答
- 不想维护 `session_id`
- 接口方自己已经在别处保存上下文，并想手动传入 `history`

字段说明：

- `question`
  - 是否必填：是
  - 类型：字符串
  - 用途：用户当前问题
- `caller_user_id`
  - 是否必填：否
  - 类型：字符串
  - 用途：最终用户 ID；不传则尝试读取 `X-Caller-User-Id`
- `channel_id`
  - 是否必填：否
  - 类型：字符串
  - 用途：渠道 ID；不传则尝试读取 `X-Channel-Id`
- `tenant_id`
  - 是否必填：否
  - 类型：字符串
  - 用途：租户/业务线 ID；不传则尝试读取 `X-Tenant-Id`
- `client_message_id`
  - 是否必填：建议填写
  - 类型：字符串
  - 用途：客户端消息唯一 ID，便于排查与幂等治理
- `history`
  - 是否必填：否
  - 类型：数组
  - 用途：由调用方自行传入上下文历史，适合外部自己维护上下文时使用
- `history[].role`
  - 是否必填：是
  - 类型：字符串
  - 可选值：`user`、`assistant`、`system`
  - 用途：标识这一条历史消息的角色
- `history[].content`
  - 是否必填：是
  - 类型：字符串
  - 用途：历史消息正文
- `meta`
  - 是否必填：否
  - 类型：对象
  - 用途：业务扩展字段；当前主要用于透传和审计上下文

请求体：

```json
{
  "question": "退款多久到账？",
  "caller_user_id": "user_10001",
  "channel_id": "wechat",
  "tenant_id": "acme",
  "client_message_id": "msg_20260708_0001",
  "history": [
    {
      "role": "user",
      "content": "我昨天申请了退款"
    },
    {
      "role": "assistant",
      "content": "好的，请问您想了解退款进度还是退款规则？"
    }
  ],
  "meta": {
    "scene": "faq"
  }
}
```

响应示例：

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "answer": "通常会在 1-3 个工作日内原路退回。",
    "sources": [],
    "matched_qa_ids": [],
    "need_human": false,
    "session_id": "",
    "request_id": "req_20260708_0003",
    "trace_id": "req_20260708_0003",
    "latency_ms": 842,
    "usage": {},
    "caller_user_id": "user_10001",
    "channel_id": "wechat",
    "tenant_id": "acme",
    "tool_calls": [],
    "knowledge_shared": true
  }
}
```

## 接口方如何自己控制“继续状态 or 重新会话”

这是接入时最重要的规则。

### 继续状态

如果你的前端/客户端判断用户还在同一轮咨询里，例如：

- “退款多久到账？”
- “那失败了怎么办？”
- “如果银行卡换了呢？”

那么应该：

1. 复用原来的 `session_id`
2. 继续调用 `POST /sessions/:id/chat`

### 重新会话

如果你的前端/客户端判断用户已经切换问题，例如：

- 从“退款”切到“发票”
- 从“订单售后”切到“会员权益”
- 用户点击“新对话”

那么应该：

1. 调用 `POST /sessions`
2. 获取新的 `session_id`
3. 用新的 `session_id` 调用 `POST /sessions/:id/chat`

### 推荐接入策略

- App / H5 / 小程序客服窗口：推荐使用**有状态会话**
- 搜索框式 FAQ：推荐使用**无状态问答**
- 如果你们前端已有自己的会话状态机：也建议保留 `session_id`，由你们自己决定什么时候丢弃并新建

## 推荐时序

### 首次进入客服

1. 客户端调用 `POST /sessions`
2. 保存返回的 `session_id`
3. 调用 `POST /sessions/:id/chat`
4. 前端按 SSE 逐步渲染回复

### 用户继续追问

1. 继续复用原 `session_id`
2. 继续调用 `POST /sessions/:id/chat`

### 用户点击“新对话”

1. 不再使用旧 `session_id`
2. 重新调用 `POST /sessions`
3. 后续对话全部走新 `session_id`

## 错误码说明

常见 HTTP 状态：

- `200`：成功
- `400`：请求参数错误
- `401`：caller token 无效或缺失
- `404`：会话不存在
- `429`：限流命中
- `500`：服务内部错误

错误示例：

```json
{
  "code": 401,
  "message": "invalid caller token"
}
```

## 最佳实践

- 始终传 `X-Request-ID`，便于排查链路
- 始终生成 `client_message_id`，便于重试幂等
- 需要延续上下文时，稳定复用同一个 `session_id`
- 不需要延续上下文时，重新创建新会话
- 统一只使用 `caller token`，不要再设计第二套对外鉴权方式
