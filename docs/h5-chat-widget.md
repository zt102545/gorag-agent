# H5 客服聊天控件接入文档

控件支持 SSE 流式回答、新建对话、最近 50 条历史消息、历史会话续聊、清空消息和删除会话。会话 ID 会按“租户 + 渠道 + 用户”保存在浏览器 `localStorage`，刷新页面后自动恢复。

## 生产接入（推荐）

`App Secret` 只能保存在渠道服务端，不能写进 H5 或 JavaScript 包。完整流程：

1. 在 GM 创建接入应用并取得 `App ID`、`App Secret`。
2. 渠道服务端根据自身登录态确认真实用户，再调用客服服务的 `POST /api/v1/biz/auth/token`。
3. 渠道服务端向 H5 提供换票接口，控件缓存短期 Token 并在过期前刷新。

```html
<script src="https://客服接口域名/customer-service-widget.js"></script>
<customer-service-widget caller-user-id="当前登录用户ID"></customer-service-widget>
```

这是推荐的最简配置。客服 API 默认使用加载控件脚本的域名，换票地址默认使用当前业务站点的 `/api/customer-service/token`，渠道默认 `h5`；租户、标题、欢迎语、位置和主题色都有默认值。只有静态资源与 API 不同域名，或换票路由不同时才需要覆盖配置。

控件脚本已通过 Go `embed` 编入服务二进制，由 `/customer-service-widget.js` 提供。更新控件后重新编译、替换并重启服务二进制即可发布新版本，无需单独同步静态文件。

渠道换票接口必须用服务端登录态校验或覆盖前端提交的 `caller_user_id`，防止冒用其他用户。服务端换票请求：

```http
POST /api/v1/biz/auth/token
Content-Type: application/json

{"app_id":"app_xxx","app_secret":"as_xxx","caller_user_id":"user_10001","channel_id":"h5","tenant_id":"tenant-a"}
```

向控件返回客服服务响应中的 `data`：

```json
{"access_token":"eyJ...","token_type":"Bearer","expires_in":899,"expires_at":"2026-07-13T12:15:00+08:00"}
```

## JavaScript 初始化

```js
const widget = CustomerServiceWidget.create({
  callerUserId: currentUser.id
})
widget.open()
widget.close()
```

## 配置项

| HTML 属性 | JS 配置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `api-base` | `apiBase` | 否 | 默认取控件脚本所在域名 |
| `caller-user-id` | `callerUserId` | 是 | 当前真实登录用户 ID |
| `token-endpoint` | `tokenEndpoint` | 否 | 默认当前业务域名的 `/api/customer-service/token` |
| `channel-id` | `channelId` | 否 | 默认 `h5` |
| `tenant-id` | `tenantId` | 否 | 租户/业务线标识 |
| `title` / `greeting` | `title` / `greeting` | 否 | 标题和欢迎语 |
| `position` | `position` | 否 | `left` 或 `right` |
| `primary-color` | `primaryColor` | 否 | 主题色 |
| `token` | `token` | 仅调试 | 已签发的用户 Token |
| `app-id` / `app-secret` | `appId` / `appSecret` | 仅本地联调 | 严禁生产使用 |

## 留存、清空、删除与续聊

- 服务端每个会话仅留存最近 50 条消息；历史接口及模型上下文按时间正序使用这 50 条。
- 控件保存当前 `session_id`，刷新后恢复历史并继续向同一会话发送消息。
- 在历史列表选择任一会话，会把它设为当前会话并继续对话。
- “清空消息”调用 `DELETE /api/v1/biz/sessions/:id/history`，永久删除消息但保留会话，可从空上下文续聊。
- “删除会话”调用 `DELETE /api/v1/biz/sessions/:id`，永久删除会话及消息。
- “新对话”创建新 `session_id`，旧会话仍可从历史列表恢复。

## 跨域、安全和部署

- 静态脚本可部署至 CDN；API 和换票接口须允许实际 H5 Origin。
- 换票接口使用 Cookie 登录态时需允许 credentials；建议 CSP 的 `connect-src` 仅放行业务域名和客服 API。
- 切换登录用户时更新 `caller-user-id`。存储键包含用户、渠道和租户，不会复用其他用户会话。
- 不要在日志、埋点、URL 或前端持久化中记录 Token/App Secret。

## 本地联调与样式验证

```html
<customer-service-widget api-base="http://127.0.0.1:8080" caller-user-id="demo-user" app-id="app_xxx" app-secret="as_xxx" channel-id="h5"></customer-service-widget>
```

访问 `http://localhost:5173/widget-demo.html`。建议验证桌面端、窄屏、移动端安全区和长文本；控件已使用 `100dvh`、滚动容器 `min-height: 0`、文本溢出保护和 Shadow DOM 样式隔离。
