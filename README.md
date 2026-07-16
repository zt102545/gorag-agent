# GoRag - 开箱即用的 Go 智能体客服

基于 [Eino ADK](https://github.com/cloudwego/eino) + Gin 构建的开箱即用智能客服框架，支持 SSE 流式多轮对话、RAG 知识库检索、HITL（Human-in-the-Loop）人工介入，以及 Vue 3 管理后台。

## 功能特性

- **双面 API**：Biz（外部客服接入，Caller Token 鉴权）与 Admin（管理后台，JWT + RBAC）
- **SSE 流式对话** + HITL 转人工（Interrupt/Resume）
- **RAG 知识库**：QA 知识库 CRUD / CSV 导入；分类/标签异步 AI 补全
- **多向量库支持**：RedisSearch（轻量默认）/ Milvus（生产级）
- **结构化存储**：SQLite / MySQL 可切换，对话 sessions/messages 永久留存
- **独立模型配置**：Chat 模型与 Embedding 模型完全分离
- **Vue 3 管理后台**：QA 管理、测试客服、会话留存、Provider/提示词配置、用户管理
- **H5 客服控件**：一行脚本即可嵌入任意网页

## 快速开始

### 1. 启动依赖

```bash
docker compose -f deployments/docker-compose.yml up -d
```

可选基础设施：

```bash
# 启用 MySQL
docker compose -f deployments/docker-compose.yml --profile mysql up -d

# 启用 Milvus
docker compose -f deployments/docker-compose.yml --profile milvus up -d
```

### 2. 环境变量

```bash
cp .env.example .env
```

至少填写：

- `OPENAI_API_KEY` — Chat 模型 API Key
- `EMBEDDING_API_KEY` — 向量化模型 API Key

### 3. 启动后端

```bash
APP_ENV=local go run ./cmd/server
```

默认管理员账号：`admin` / `admin123`（可通过 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 覆盖）。

### 4. 启动前端

```bash
cd web
npm install
npm run dev
```

打开 <http://localhost:5173>，登录后可管理 QA 知识库、Provider、提示词，并使用「测试客服」功能。

### 5. 导入 QA 知识库（可选）

```bash
go run ./cmd/indexer --file ./data/knowledge/qa.csv
```

或在管理后台「QA 知识库」页面上传 CSV。缺分类/标签的条目将由后台 worker 自动补全。

## 配置说明

### Chat 模型

在 `config/config.yaml` 的 `ai.chat_model` 下配置，负责：

- Agent 对话
- 测试客服回复
- QA 异步分类/打标

### Embedding 模型

在 `config/config.yaml` 的顶层 `embedding` 下配置，负责：

- QA 问题向量化
- 用户查询向量化
- 向量库检索

## 向量库切换

### RedisSearch（默认）

```yaml
vector_store:
  provider: redissearch
  top_k: 3
  redissearch:
    index_name: customer_service_qa
    key_prefix: "cs_qa:"
    vector_field: vector_content
```

需要 Redis Stack / 带 RediSearch 的发行版。

### Milvus

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

切换向量库后建议：清理旧索引 → 重新导入 QA → 确保 `embedding.dimensions` 与向量库索引维度一致。

## 存储架构

| 存储 | 内容 |
|------|------|
| SQLite/MySQL | 用户、Caller Token、QA 元数据、Provider/提示词、sessions、messages |
| Redis | 分类队列、HITL checkpoint、以及 redissearch 模式下的向量索引 |
| Milvus | milvus 模式下的 QA 向量与检索副本 |

## Biz API（外部接入）

任意 H5 页面只需引用脚本并填写公开渠道 ID、用户 ID，即可接入悬浮聊天控件。

```html
<script src="http://localhost:8080/customer-service-widget.js"></script>
<customer-service-widget channel-id="website-main" caller-user-id="user_10001"></script>
```

详细接入文档见 [`docs/h5-chat-widget.md`](docs/h5-chat-widget.md)。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/sessions` | 创建会话 |
| GET | `/sessions/:id` | 会话详情 |
| GET | `/sessions/:id/history` | 历史消息 |
| DELETE | `/sessions/:id` | 删除会话 |
| POST | `/sessions/:id/chat` | SSE 流式对话 |
| POST | `/sessions/:id/approve` | HITL 审批 |

## Admin API（管理后台）

前缀：`/api/v1/admin`，先登录获取 JWT：

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
- `/playground/sessions*`（测试客服）

## 项目结构

```text
cmd/server/               HTTP 服务入口
cmd/indexer/              QA CSV 导入 CLI
cmd/migrate/              SQLite → MySQL 迁移 CLI
internal/auth/            JWT 用户 + Caller Token
internal/admin/           Provider / 提示词管理
internal/agent/           Eino Agent 构建与运行
internal/vectorstore/     RedisSearch / Milvus 向量库抽象
internal/knowledge/qa/    QA 知识库核心（SQL + 向量 + 分类 worker）
internal/graph/rag/       RAG 检索管道
internal/session/         会话与消息留存
internal/server/          HTTP 路由与 Handler
web/                      Vue 3 管理前端
config/                   YAML 配置
deployments/              Docker / Docker Compose 部署
docs/                     文档
```

## 部署

### Docker Compose

```bash
make docker-up
```

### Docker 构建

```bash
make build-linux
```

## License

MIT
