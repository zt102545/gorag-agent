# 多接入应用提示词组与知识库设计

## 1. 背景与目标

当前客服服务使用一套全局提示词和一套全局 QA 数据。新功能要求每个接入应用可以通过自己的调用 token 绑定一组提示词和一个 QA 知识库，同时继续共享聊天模型、Embedding 模型、Agent 通用参数、会话存储和企业会话分析能力。

本次实现提供：

- 多个提示词组，每组包含完整的提示词项，可从现有组复制创建。
- 多个 QA 知识库，每个库包含独立 QA，可从现有库完整复制创建。
- 接入应用（`caller_tokens`）分别绑定一个提示词组和一个知识库。
- 现有提示词与 QA 自动成为“默认提示词组”和“默认知识库”；历史 token 未显式绑定时继续使用默认组。
- 模型配置仍为全局单套，企业会话归档与分析默认继续使用默认组。

## 2. 核心概念与数据模型

### 2.1 提示词组

`prompt_groups` 保存组信息：`id`、`name`、`description`、`is_default`、创建/更新时间。

`prompt_group_items` 保存组内提示词，唯一键为 `(group_id, prompt_key)`。默认组首次初始化时从旧 `prompts` 表复制；旧表保留用于向后兼容，不再作为多组运行时的主数据源。

### 2.2 QA 知识库

`knowledge_bases` 保存知识库信息：`id`、`name`、`description`、`is_default`、创建/更新时间。

`qa_items` 新增 `knowledge_base_id`。启动迁移把空值写入默认知识库。分类、标签、导入、增删改查均按知识库隔离。

### 2.3 接入应用绑定

`caller_tokens` 新增：

- `prompt_group_id`
- `knowledge_base_id`

新建或修改应用时允许选择二者。字段留空代表默认组，服务端在响应和运行时解析为默认组。删除非默认组前必须确认没有 token 绑定；默认组不可删除。

## 3. 请求链路

1. `BizAuth` 校验调用 token，并从应用记录读取提示词组、知识库绑定。
2. 中间件把绑定写入请求 `context.Context`，不信任客户端自行传入的组 ID。
3. 提示词运行时注册表以 `(prompt_group_id, prompt_key)` 为缓存键；没有绑定或目标项缺失时回退默认组，再回退代码内默认文本。
4. 意图判断、固定回复、RAG 上下文提示以及 Agent 系统提示均从同一个提示词组读取。
5. RAG 先扩大候选召回，再按 SQL 元数据中的 `knowledge_base_id` 做强制过滤，任何其他知识库内容都不会进入模型上下文。该方式兼容现有 RedisSearch/Milvus 索引，无需破坏性重建。
6. 后台无调用 token 的测试、旧接口和企业微信分析使用默认组。

## 4. 管理 API

GM 前缀：`/api/v1/gm/ai-customer-service`。

### 提示词组

- `GET /prompt-groups`
- `POST /prompt-groups`
- `PUT /prompt-groups/:id`
- `POST /prompt-groups/:id/copy`
- `DELETE /prompt-groups/:id`
- `GET /prompt-groups/:id/prompts`
- `PUT /prompt-groups/:id/prompts/:key`

旧 `GET/PUT /prompts...` 保持可用并操作默认提示词组。

### 知识库

- `GET /knowledge-bases`
- `POST /knowledge-bases`
- `PUT /knowledge-bases/:id`
- `POST /knowledge-bases/:id/copy`
- `DELETE /knowledge-bases/:id`

QA API 通过查询参数或请求体的 `knowledge_base_id` 指定后台正在管理的库；导入接口使用 multipart 字段 `knowledge_base_id`。未传时操作默认库。

### 接入应用

现有 caller-token 创建、修改、列表响应增加 `prompt_group_id`、`knowledge_base_id`，凭证签发流程与客户端接入协议不变。

## 5. 前端交互

- 提示词配置页顶部增加提示词组选择、创建、复制、重命名和删除；切换后编辑该组的全部提示词。
- QA 库页顶部增加知识库选择和相同的组管理操作；列表、筛选、导入、新建、批量删除均固定携带当前知识库 ID。
- 接入应用页在新建/编辑弹窗中增加“提示词组”和“QA 知识库”必选下拉框，列表展示绑定关系。
- 模型配置、Agent 通用配置、企业会话分析页面不增加分组入口。

## 6. 兼容、约束与失败处理

- 启动迁移幂等：固定默认组 ID，仅补缺失数据，不覆盖已编辑内容。
- 默认组不可删除；有应用绑定或仍含 QA 的知识库不可删除。
- 复制提示词组在一个数据库事务内复制组和全部提示词项。
- 复制知识库逐条创建新 QA 并生成新向量；部分失败时返回错误并清理已创建的新库，避免出现看似成功但内容不完整的副本。
- token 绑定指向不存在的组时鉴权失败，防止静默串库；历史空绑定除外。
- 会话记录继续保存 `caller_token_id`，因此分析维度不变；需要时可通过 token 反查当时绑定。修改绑定只影响后续请求。

## 7. 验收用例

1. 升级旧数据库后出现各一个默认组，旧提示词和 QA 数量不变。
2. 从默认提示词组复制 A，修改 A 的无命中回复；绑定 A 的 token 返回新回复，默认 token 不受影响。
3. 从默认知识库复制 B，分别修改同一问题答案；不同 token 只能命中各自库。
4. 新建空知识库后导入 CSV，列表、分类和标签只展示该库数据。
5. 有 token 绑定时删除组返回冲突；解绑后可以删除非默认提示词组，非空知识库仍需先清空。
6. 历史 token、公开控件换票、服务端换票和 GM 测试聊天均可继续使用。
7. `go test ./...` 通过，`pnpm typecheck` 与 `pnpm build` 通过。

## 8. API 示例

创建提示词组（自动以默认组内容初始化）：

```json
POST /prompt-groups
{"name":"海外站客服","description":"英文站点提示词"}
```

从指定组复制：

```json
POST /prompt-groups/{source_id}/copy
{"name":"海外站客服-灰度"}
```

创建空知识库、向其中新增 QA：

```json
POST /knowledge-bases
{"name":"海外站知识库"}

POST /qa
{
  "knowledge_base_id":"返回的知识库 ID",
  "question":"How can I request a refund?",
  "answer":"Open the order and choose Request refund.",
  "tags":["refund"]
}
```

绑定接入应用：

```json
PUT /caller-tokens/{id}
{
  "prompt_group_id":"提示词组 ID",
  "knowledge_base_id":"知识库 ID"
}
```

## 9. 部署与回滚

推荐部署顺序为后端、数据库启动迁移、GM 前端。后端先上线后旧 GM 仍可通过兼容接口管理默认组；新 GM 上线后才能管理多组。

上线前备份业务数据库。应用启动时由 GORM 创建三张新表并为两张旧表增加绑定字段；默认 ID 为固定 UUID，迁移可重复执行。升级后建议核对：

```sql
SELECT COUNT(*) FROM prompt_groups WHERE is_default = 1;
SELECT COUNT(*) FROM knowledge_bases WHERE is_default = 1;
SELECT COUNT(*) FROM qa_items WHERE knowledge_base_id = '' OR knowledge_base_id IS NULL;
```

前两项都应为 `1`，最后一项应为 `0`。由于旧表和旧字段未删除，代码回滚不会丢失旧提示词或 QA；但回滚版本无法识别新建组。回滚期间应冻结多组写入，重新上线新版后数据会继续可用。

## 10. 可观测性与后续优化

- HTTP 日志保留 `caller_token_id`、渠道和租户，可通过 token 表关联到当前组绑定。
- 当前向量检索采用“扩大候选 + SQL 强过滤”，优先保证隔离和零破坏升级。知识库数量或总 QA 规模明显增长后，可升级为 RedisSearch TAG filter 或 Milvus partition 原生过滤。
- 绑定修改即时生效，不修改历史会话。若未来需要严格审计历史配置，可在 session 表快照保存两个组 ID 与版本号。
