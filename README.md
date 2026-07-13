# Agent Customer Service

基于 [Eino ADK](https://github.com/cloudwego/eino) + Gin 的智能客服服务，支持 SSE 流式多轮对话、QA 知识库检索、Interrupt/Resume 人工介入，以及 Vue GM 管理台。

## 功能

- 双面 API：Biz（外部客服接入，Caller Token）与 Admin（GM 管理，JWT+RBAC）
- SSE 流式对话 + HITL 转人工
- QA 知识库 CRUD / CSV 导入；空分类/标签异步 AI 补全
- 结构化数据 SQLite / MySQL 可切换；对话 `sessions/messages` 永久留存
- 向量库支持 `redissearch` / `milvus`
- Embedding 向量化模型独立配置，不再与 Chat 模型混用
- Vue3 GM：QA、测试客服、会话留存、Provider、提示词、用户、调用方 Token

## 快速开始

### 1. 启动依赖

默认本地最轻量方案：

```bash
docker compose -f deployments/docker-compose.yml up -d
```

如果你要切换其它基础设施：

```bash
# 启 MySQL
docker compose -f deployments/docker-compose.yml --profile mysql up -d

# 启 Milvus
docker compose -f deployments/docker-compose.yml --profile milvus up -d
```

### 2. 环境变量

```bash
cp .env.example .env
```

至少填：

- `OPENAI_API_KEY`
- `EMBEDDING_API_KEY` 或兼容旧的 `OPENAI_EMBEDDING_API_KEY`

### 3. 启动后端

```bash
APP_ENV=local go run ./cmd/server
```

默认管理员：`admin` / `admin123`（可用 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 覆盖种子）。

### 4. 启动 GM 前端

```bash
cd web
npm install
npm run dev
```

打开 <http://localhost:5173>，登录后可管理 QA、Provider、提示词，并用「测试客服」页直接走真实 RAG。

### 5. 导入 QA（可选）

```bash
go run ./cmd/indexer --file ./data/knowledge/qa.csv
```

或在 GM「QA 知识库」页上传 CSV。缺分类/标签的条目状态为「待补全」，后台 worker 会结合历史类目/标签调用模型写回。

## 配置重点

### Chat 模型

在 `config/config.yaml` 的 `ai.chat_model` 下配置，只负责：

- Agent 对话
- 测试客服回复
- QA 异步分类/打标

### Embedding 模型

在 `config/config.yaml` 的顶层 `embedding` 下配置，只负责：

- QA 问题向量化
- 用户查询向量化
- 向量库检索

这两套模型现在是彻底分开的。

## 向量库切换

### 1. RedisSearch（默认）

```yaml
vector_store:
  provider: redissearch
  top_k: 3
  redissearch:
    index_name: customer_service_qa
    key_prefix: "cs_qa:"
    vector_field: vector_content
```

要求 Redis 是 **Redis Stack / 带 RediSearch 的发行版**。

### 2. Milvus

```yaml
vector_store:
  provider: milvus
  top_k: 3
  milvus:
    address: "localhost:19530"
    database: default
    collection: customer_service_qa
    metric_type: COSINE
```

云端 Zilliz / 托管 Milvus 也可：

```yaml
vector_store:
  provider: milvus
  milvus:
    uri: "https://your-endpoint:443"
    token: "your-token"
    database: default
    collection: customer_service_qa
```

切换向量库后建议：

1. 清理旧索引 / collection
2. 重新导入 QA
3. 确保 `embedding.dimensions` 与向量库索引维度一致

## 数据库存储职责

| 存储 | 内容 |
|------|------|
| SQLite/MySQL | 用户、Caller Token、QA 元数据、Provider/提示词、sessions、messages |
| Redis | 分类队列、HITL checkpoint、以及 `redissearch` 模式下的向量索引 |
| Milvus | `milvus` 模式下的 QA 向量与检索副本 |

## Biz API（外部客服接入）

任意 H5 页面可直接接入悬浮聊天控件，支持新对话、SSE 流式消息、历史会话列表与续聊。接入方式见 [`docs/h5-chat-widget.md`](docs/h5-chat-widget.md)，本地演示页为 `http://localhost:5173/widget-demo.html`。

前缀：`/api/v1/biz`，请求头 `Authorization: Bearer <caller_token>`（在 GM「调用方 Token」创建）。

更完整的外部接入说明、请求头约定、会话续聊/新会话选择规则、`curl` 示例与 SSE 说明，见：

- [`docs/external-api.md`](docs/external-api.md)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/sessions` | 创建会话 |
| GET | `/sessions/:id` | 详情 |
| GET | `/sessions/:id/history` | 历史 |
| DELETE | `/sessions/:id` | 关闭会话（消息保留） |
| POST | `/sessions/:id/chat` | SSE 对话 |
| POST | `/sessions/:id/approve` | HITL 审批 |

## Admin API（GM）

前缀：`/api/v1/admin`，先登录拿 JWT：

```bash
curl -X POST http://localhost:8080/api/v1/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

主要资源：

- `/auth/me`、`/users`、`/caller-tokens`
- `/knowledge/qa`（CRUD、import、reclassify、categories、tags）
- `/providers/chat|embedding`、`/prompts`
- `/sessions`（只读留存查阅）
- `/playground/sessions*`（GM 测试客服：建会话 / SSE 对话 / 审批）

## 项目结构

```text
cmd/server/               HTTP 服务入口
cmd/indexer/              QA CSV 导入 CLI
internal/db/              GORM 模型与 SQLite/MySQL 拨号
internal/auth/            JWT 用户 + Caller Token
internal/admin/           Provider / 提示词
internal/vectorstore/     RedisSearch / Milvus 向量库抽象
internal/knowledge/qa/    QA（SQL meta + 向量库 + 分类 worker）
internal/session/         SQL 会话与消息留存
internal/server/          路由（biz / admin）
web/                      Vue3 GM 前端
config/                   YAML 配置
```
