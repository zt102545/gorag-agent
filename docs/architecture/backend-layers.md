# 后端目录与分层

下一阶段后端按以下目录语义演进，避免业务逻辑继续堆在 `handler` 和 `wire.go` 中。

## 分层约束

1. `internal/bootstrap`
   - 只负责应用装配、生命周期和模块注册。
   - 不写业务判断，不直接承载长流程编排。

2. `internal/application`
   - 负责用例编排、事务边界、幂等、权限协调和异步任务发起。
   - `handler` 只依赖这一层，不直接依赖具体存储实现。

3. `internal/domain`
   - 放稳定业务语义、实体、值对象、仓储接口。
   - 不依赖 Gin、Gorm、Redis、SSE。

4. `internal/infrastructure`
   - 放数据库、缓存、向量库、LLM、队列、观测等具体技术实现。

5. `internal/server`
   - 纯 HTTP 适配层，负责参数解析、鉴权中间件和响应封装。

## 已落地的第一批目录

- `internal/bootstrap/providers.go`
- `internal/bootstrap/modules.go`
- `internal/bootstrap/http.go`
- `internal/application/session`
- `internal/application/knowledge`
- `internal/application/settings`

## 迁移原则

- 优先加一层包装，不做一次性全量搬家。
- 新功能优先落到 `application`，旧逻辑逐步抽离。
- 需要替换存储时，先在 `application` 层收口依赖，再迁移实现目录。
