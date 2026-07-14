# 企业微信会话内容存档与 QA 缺口分析

## 1. 功能范围

本功能在现有 `cmd/server` 进程内持续拉取一个企业的企业微信会话内容，采集专用企业员工参与的外部用户私聊和指定群聊文本，再按每日固定时间调用现有 Chat 模型提取客户业务问题并检索 QA 知识库。

- 只支持一个企业。
- 支持一个专用企业员工 `userid`。
- 支持配置多个群聊 `roomid`。
- 只保存 `action=send`、`msgtype=text` 的消息。
- 私聊消息的 `roomid` 为空，只要发送人或接收人包含专用员工就会采集；群 ID 配置不会排除私聊。
- 群聊消息带 `roomid`；群 ID 列表非空时只采集指定群，列表为空时采集该员工参与的全部群聊。
- 管理端按日期、群聊或私聊会话展示完整文本消息流，历史记录可以查看。
- 完整配置首次保存时记录 `qa_start_at`；早于该时间的历史消息永远不会成为 QA 来源。
- 每天在配置的 `HH:mm` 时间只处理当天且不早于 `qa_start_at` 的消息，每个自然日成功处理一次。
- 已有 QA 的问题只进入分析统计，不产生待办。
- 未找到完整答案的问题进入现有“未找到答案问题库”。
- 如果员工在会话中已经回答，模型会把员工回答整理为答案草稿；草稿必须由人工审查后才能转入正式 QA。
- 人工转换沿用现有 QA 创建流程，会同步写入 SQL 元数据、RedisSearch/Milvus 向量索引和自动分类队列。

不在当前范围内：图片 OCR、语音转文字、文件解析、视频、红包、位置、撤回消息展示、多企业配置。

## 2. 官方能力与 SDK

- 官方文档：<https://developer.work.weixin.qq.com/document/path/91774>
- 本项目使用的官方包：<https://wwcdn.weixin.qq.com/node/wwcomm/sdk_x86_v3_20250205.tgz>
- SDK 版本：`20250205`
- 官方 C SDK MD5：`f2db3dd1372c516db6290afbd1b5c698`
- 嵌入动态库 SHA-256：`79ced4de6b18d5e96a21cd06f325794dc8957f8925120538d56d4ce827d3dfd0`

官方 SDK 通过全局 `seq` 增量拉取消息。接口返回的消息从传入 `seq + 1` 开始，因此服务只在一批消息成功解密、过滤并写入数据库后推进 `last_seq`。服务重启后从数据库中的游标继续。

## 3. 企业微信后台准备

在企业微信管理后台开通“会话内容存档”，然后准备：

1. 企业 `CorpID`。
2. 会话内容存档 `Secret`，不是普通自建应用 Secret。
3. 已开通会话存档的专用企业员工 `userid`。
4. 需要分析的群聊 `roomid`。
5. 2048 位 RSA 公私钥和企业微信后台显示的公钥版本。
6. 服务生产出口固定 IP，并加入企业微信会话存档可信 IP。

生成 RSA 密钥示例：

```bash
openssl genrsa -out wecom_private.pem 2048
openssl rsa -in wecom_private.pem -pubout -out wecom_public.pem
```

将 `wecom_public.pem` 配置到企业微信管理后台；`wecom_private.pem` 只录入本系统管理页，禁止提交到 Git。

如果暂时不知道 `roomid`，可先把群 ID 列表留空，让服务采集该员工参与的文本会话，从“文本消息”页确认群 ID 后再收紧配置。

## 4. 单进程与 SDK 嵌入方式

没有新增独立采集进程。`bootstrap.NewApp` 会启动企业微信后台 worker，与 HTTP Server 同进程运行：

```text
cmd/server
  ├─ Gin Admin/GM/Biz API
  ├─ QA 分类 worker
  └─ 企业微信 worker
       ├─ SDK 增量拉取
       ├─ RSA + 官方 SDK 解密
       ├─ 文本过滤与入库
       └─ 当日问题分析
```

官方 `libWeWorkFinanceSdk_C.so` 位于：

```text
internal/wecom/sdk/native/linux_amd64/libWeWorkFinanceSdk_C.so
```

它通过 `go:embed` 编入最终 server 二进制。服务第一次使用 SDK 时会：

1. 校验嵌入内容并计算 SHA-256。
2. 释放到系统临时目录 `agent-customer-service/wecom-sdk/`。
3. 使用 CGO 的 `dlopen/dlsym` 加载。
4. 调用官方 `NewSdk`、`Init`、`GetChatData`、`DecryptData` 等接口。

因此部署产物不需要额外携带 `.so`，但运行平台必须是 Linux amd64 + glibc。Windows/macOS 和非 CGO 构建会使用不可用桩，普通开发测试可以运行，但不能真实拉取企业微信消息。

## 5. 构建与部署

正式构建：

```bash
docker build -f deployments/Dockerfile -t agent-customer-service:wecom .
```

或者：

```bash
make build-linux
```

Docker 构建使用：

- `golang:1.25-bookworm`
- `CGO_ENABLED=1`
- `GOOS=linux GOARCH=amd64`
- `debian:bookworm-slim` 运行环境

构建阶段会先运行 `internal/wecom/sdk` 的 Linux 测试，确认嵌入的官方 SDK 可以成功 `dlopen`。

生产还需要配置现有 MySQL、Redis、向量库和 Chat/Embedding 模型。建议生产使用 MySQL，不建议使用 SQLite 承载持续增长的聊天消息。

## 6. 敏感配置加密

生产必须设置：

```bash
WECOM_CONFIG_ENCRYPTION_KEY=<长期不变的高强度随机字符串>
```

可使用：

```bash
openssl rand -base64 32
```

管理页提交的会话存档 Secret 和 RSA 私钥会使用 AES-GCM 加密后写入业务数据库；API 只返回 `has_secret` 和 `has_private_key`，不会回传明文。

注意：修改或丢失 `WECOM_CONFIG_ENCRYPTION_KEY` 后，数据库中的旧 Secret 和私钥将无法解密。未配置时服务仅为本地兼容而回退到 `JWT_SECRET`，生产不允许依赖该回退。

## 7. 管理端使用

正式管理端是 `D:\project\Web-GM-Client`。

菜单路径：

```text
AI 客服 → 企业微信会话分析
```

配置字段：

| 字段 | 说明 |
|---|---|
| CorpID | 单个企业的企业 ID |
| Secret | 会话内容存档 Secret；留空保存表示不修改 |
| RSA 私钥 | 与企业微信后台公钥对应的 PEM；留空表示不修改 |
| 公钥版本 | 企业微信后台给出的版本号，必须与消息一致 |
| 专用员工 userid | 只分析该员工参与的会话 |
| roomid 列表 | 每行一个；非空时只过滤群聊，不影响外部用户私聊 |
| 拉取间隔 | 最小 5 秒，默认 10 秒 |
| 每日 QA 分析时间 | `HH:mm`，默认 `23:50`，按 Asia/Shanghai 执行 |
| 自动任务 | 启用后同一 server 进程自动拉取，并每天分析一次当天消息 |

建议操作顺序：

1. 填写配置但暂不启用自动任务。
2. 保存。
3. 点击“测试连接”。
4. 点击“立即同步”，从“完整聊天记录”确认私聊、员工和群 ID 正确。
5. 选择任意历史日期可查看当天全部已存储会话；查看历史不会触发 QA。
6. 到每日配置时间后，服务自动分析当天且位于 `qa_start_at` 之后的消息。
7. 到“未找到答案问题库”查看企业微信来源问题、原始上下文和答案草稿。
8. 人工修订答案、分类、标签后点击“保存并加入正式 QA”。
9. 确认流程正常后再启用自动任务。

## 8. 拉取和过滤规则

每轮最多调用 100 批，每批最多 1000 条。每条加密消息依次执行：

1. 验证消息 `publickey_ver` 与配置一致。
2. 使用对应 RSA 私钥解密 `encrypt_random_key`。
3. 调用官方 SDK `DecryptData` 解密消息正文。
4. 只接受 `action=send` 与 `msgtype=text`。
5. 验证发送人或接收人包含专用员工。
6. `roomid` 为空的专用员工私聊直接保留；`roomid` 非空时根据群列表过滤。
7. 以 `msgid`、`seq` 唯一约束幂等入库。
8. 成功提交数据库事务后推进游标。

公钥版本不匹配或某条消息解密失败时，本批不会推进游标，避免静默漏消息。修复私钥/版本配置后可重新同步。

## 9. 智能分析与人工审查

分析日期统一按 `Asia/Shanghai` 的自然日计算。完整配置首次保存时写入 `qa_start_at`，历史消息即使已经入库也只用于管理端查看。自动任务到达每日配置时间后分析当天消息，成功后写入 `last_analyze_date`，当天不会重复执行。管理 API 的手动分析也只允许当天，并同样受 `qa_start_at` 限制。

1. 按 `roomid` 聚合群聊；单聊按外部联系人聚合。
2. 把时间、消息 ID、发送人、员工/客户角色和文本发送给当前激活的 Chat 模型。
3. 模型只提取适合进入标准客服知识库的客户业务问题。
4. 对每个问题使用现有 RAG 服务检索 QA。
5. 问题与 QA 完全一致时直接视为已有；其余候选由模型判断能否完整回答。
6. 没有完整答案或判断失败时，宁可进入人工审查，不自动判定已有。
7. 员工已有回答保存为待审问题的 `answer` 草稿。
8. 以客户源消息 `msgid` 做分析幂等键；员工后续补答触发重分析时会更新上下文，并在答案仍为空时补入草稿，不重复创建发生事件，也不会覆盖人工已经编辑的答案。

聊天正文会发送到当前配置的 Chat 模型。使用外部模型服务前，必须确认企业的数据出域和隐私合规要求。

## 10. 数据表

| 表 | 用途 |
|---|---|
| `wecom_archive_configs` | 单企业配置、加密凭据、员工、群、游标、状态 |
| `wecom_messages` | 过滤后的企业微信文本消息 |
| `wecom_analysis_runs` | 每次日期分析的统计、状态与错误 |
| `unanswered_questions` | 聚合后的待人工问题，新增企业微信来源字段 |
| `unanswered_question_events` | 源消息、员工、群聊与上下文证据 |

这些表已加入 `db.Models()` 和 SQLite → MySQL 迁移服务。

## 11. 管理 API

正式 GM 兼容接口前缀：

```text
/api/v1/gm/ai-customer-service
```

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/wecom/config` | 获取掩码配置和运行状态 |
| PUT | `/wecom/config` | 保存配置 |
| POST | `/wecom/test` | 使用官方 SDK 测试连接 |
| POST | `/wecom/sync` | 立即增量同步 |
| POST | `/wecom/analyze` | 运维手动分析今天；历史日期会被拒绝 |
| GET | `/wecom/messages` | 查询文本消息 |
| GET | `/wecom/conversations?date=YYYY-MM-DD` | 查询某天群聊和私聊会话列表 |
| GET | `/wecom/conversations/:id/messages?date=YYYY-MM-DD` | 查询某天该会话的完整文本记录 |
| GET | `/wecom/analysis-runs` | 查询最近分析任务 |

同样的能力在 `/api/v1/admin/wecom` 下提供 JWT + RBAC 版本，其中配置修改、测试、同步和分析要求 Admin，消息与任务查询要求 Viewer。

## 12. 运维与排错

管理页会展示 `last_seq`、`qa_start_at`、每日分析时间、最近处理日期、最近同步时间和最近错误。

常见问题：

- `SDK unavailable`：当前不是 Linux amd64 CGO 正式构建。
- `Init failed`：检查 CorpID、会话存档 Secret、可信 IP。
- 公钥版本不一致：更新管理页公钥版本及对应私钥，不能只修改其中一个。
- RSA 解密失败：检查私钥是否与企业微信后台当前版本公钥成对、PEM 是否完整。
- 长时间没有消息：检查自动任务、员工是否在存档范围、群 roomid 是否过滤过严。
- 没有分析结果：检查 Chat 模型是否激活、当天是否存在未分析文本消息。
- QA 检索错误：检查 RedisSearch/Milvus 和 Embedding 配置。

企业微信待拉取数据保留时间有限，应对 `last_sync_at` 和 `last_error` 做外部监控告警，避免服务长时间中断造成不可恢复的数据缺口。

## 13. 验证命令

```bash
go test ./...
docker build -f deployments/Dockerfile -t agent-customer-service:wecom .
```

正式管理端：

```bash
cd D:\project\Web-GM-Client
pnpm build
```

该管理端当前全仓库 `pnpm typecheck` 存在与本功能无关的历史类型错误；应单独确认本次修改文件没有新增错误，并以 Vite 正式构建作为本功能前端验收门槛。
