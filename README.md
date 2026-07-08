# Agent Customer Service

基于 [Eino ADK](https://github.com/cloudwego/eino) + Gin 的智能客服服务，支持 SSE 流式多轮对话、QA 知识库检索、Interrupt/Resume 人工介入。

## 功能

- SSE 流式对话（`POST /api/v1/sessions/:id/chat`）
- 多轮会话管理（Redis）
- **QA 知识库**：结构化问答对（question/answer/category/tags）
- **QA CRUD + CSV 批量导入** HTTP 接口
- Agent 检索返回 Q/A 对，优先使用标准答案
- 转人工 HITL（`POST /api/v1/sessions/:id/approve`）
- 多 Provider 模型接入（默认 OpenAI 兼容 API）

## 快速开始

### 1. 启动依赖

```bash
docker compose -f deployments/docker-compose.yml up -d
```

### 2. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env，填入 OPENAI_API_KEY
```

### 3. 导入 QA 知识库

```bash
# CLI 方式
go run ./cmd/indexer --file ./data/knowledge/qa.csv

# 或 HTTP 方式
curl -X POST http://localhost:8080/api/v1/knowledge/qa/import \
  -F "file=@./data/knowledge/qa.csv"
```

### 4. 启动服务

```bash
APP_ENV=local go run ./cmd/server
```

## QA 知识库 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/knowledge/qa` | 创建单条 QA |
| GET | `/api/v1/knowledge/qa` | 分页列表（`?category=&page=&size=`） |
| GET | `/api/v1/knowledge/qa/:id` | 获取单条 |
| PUT | `/api/v1/knowledge/qa/:id` | 更新（自动重新向量化） |
| DELETE | `/api/v1/knowledge/qa/:id` | 删除 |
| POST | `/api/v1/knowledge/qa/import` | CSV 文件上传（`multipart/form-data`，字段名 `file`） |

### CSV 格式

```csv
question,answer,category,tags
如何申请退款,7天内可申请退款,退款政策,"退款,售后"
```

### 创建单条 QA

```bash
curl -X POST http://localhost:8080/api/v1/knowledge/qa \
  -H "Content-Type: application/json" \
  -d '{
    "question": "会员有什么权益？",
    "answer": "专属折扣、优先客服、生日优惠券。",
    "category": "会员权益",
    "tags": ["会员", "优惠"]
  }'
```

## 对话 API

```bash
# 创建会话
curl -X POST http://localhost:8080/api/v1/sessions

# SSE 流式对话
curl -N -X POST http://localhost:8080/api/v1/sessions/{session_id}/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"如何申请退款？"}'

# 人工审批恢复（转人工后）
curl -N -X POST http://localhost:8080/api/v1/sessions/{session_id}/approve \
  -H "Content-Type: application/json" \
  -d '{"interrupt_id":"...","approved":true}'
```

## 项目结构

```
cmd/server/               HTTP 服务入口
cmd/indexer/              QA CSV 导入 CLI
internal/knowledge/qa/    QA 知识库（元数据 + 向量双存储）
internal/config/          Viper 配置加载
internal/bootstrap/       依赖组装
internal/agent/           Eino ADK Agent
internal/graph/rag/       QA 检索服务
internal/server/          Gin 路由与 Handler
pkg/sse/                  SSE 事件转换
config/                   YAML 配置文件
data/knowledge/qa.csv     示例 QA 数据
```

## 配置说明

- 基础配置：`config/config.yaml`
- 环境覆盖：`config/config-{env}.yaml`
- 密钥通过环境变量注入：`OPENAI_API_KEY`、`OPENAI_EMBEDDING_API_KEY`
- QA 向量索引：`rag.index_name=customer_service_qa`，`rag.key_prefix=cs_qa:`
