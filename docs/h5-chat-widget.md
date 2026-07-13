# H5 客服控件接入文档

客服服务地址：`https://testapi.realcopyright.cn`

## 一、接入方要做什么

公开控件模式下，接入方不需要开发换票接口，也不需要保存 `App ID` 或 `App Secret`。只需要在页面中引用脚本，并传入渠道 ID 和用户 ID：

```html
<script src="https://testapi.realcopyright.cn/customer-service-widget.js"></script>

<customer-service-widget
  channel-id="website-main"
  caller-user-id="user_10001"
></customer-service-widget>
```

- `channel-id`：客服系统中配置的公开渠道 ID。
- `caller-user-id`：接入方当前用户的稳定唯一 ID；没有登录体系时，可以生成并持久化一个匿名 ID。

控件默认显示为页面右下角的浮动“在线客服”按钮。点击后打开聊天窗口，移动端打开后全屏显示。

## 二、客服管理员要做什么

在 GM 的“调用凭证”页面创建或编辑接入应用：

1. 填写唯一的渠道 ID，例如 `website-main`。
2. 开启“公开控件”。
3. 配置允许来源域名，例如 `https://www.example.com`，每行一个完整 Origin；留空表示不限制来源。
4. 配置每 IP 每分钟请求上限，默认 `60`。
5. 配置每 IP 每日请求配额，默认 `2000`。

公开渠道 ID 在系统中不可重复。关闭应用或关闭公开控件后，该渠道已签发的公开短期 Token 会立即失效。

## 三、匿名用户 ID 示例

如果接入页面没有登录用户，可以在浏览器中生成匿名 ID：

```html
<script>
  const key = 'customer-service-anonymous-id'
  let userId = localStorage.getItem(key)
  if (!userId) {
    userId = crypto.randomUUID()
    localStorage.setItem(key, userId)
  }

  const widget = document.querySelector('customer-service-widget')
  widget.setAttribute('caller-user-id', userId)
</script>
```

完整页面示例：

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>在线客服</title>
  </head>
  <body>
    <script src="https://testapi.realcopyright.cn/customer-service-widget.js"></script>
    <customer-service-widget channel-id="website-main"></customer-service-widget>

    <script>
      const key = 'customer-service-anonymous-id'
      let userId = localStorage.getItem(key)
      if (!userId) {
        userId = crypto.randomUUID()
        localStorage.setItem(key, userId)
      }
      document
        .querySelector('customer-service-widget')
        .setAttribute('caller-user-id', userId)
    </script>
  </body>
</html>
```

## 四、JavaScript 初始化

单页应用可以动态创建控件：

```js
const widget = CustomerServiceWidget.create({
  channelId: 'website-main',
  callerUserId: currentUser.id,
  title: '在线客服',
  greeting: '您好，请问有什么可以帮您？',
  primaryColor: '#0f766e',
  secondaryColor: '#14b8a6'
})

widget.open()
widget.close()
```

用户退出或切换账号时，应执行 `widget.remove()` 销毁旧控件，再为新用户创建控件。

## 五、配置项

| HTML 属性 | JavaScript 配置 | 必填 | 默认值 / 说明 |
| --- | --- | --- | --- |
| `channel-id` | `channelId` | 是 | GM 中已开启的公开渠道 ID |
| `caller-user-id` | `callerUserId` | 是 | 登录用户 ID 或持久化的匿名 ID |
| `api-base` | `apiBase` | 否 | 自动取脚本域名，即 `https://testapi.realcopyright.cn` |
| `tenant-id` | `tenantId` | 否 | 公开模式使用渠道绑定的租户，接入方通常无需填写 |
| `title` | `title` | 否 | `在线客服` |
| `greeting` | `greeting` | 否 | 控件欢迎语 |
| `position` | `position` | 否 | `right`，可设置为 `left` |
| `primary-color` | `primaryColor` | 否 | 主色，默认 `#246bfd`，用于悬浮按钮、发送按钮、头像和用户消息 |
| `secondary-color` | `secondaryColor` | 否 | 渐变辅色，默认 `#6854dc`，与主色组成标题栏渐变 |

不配置配色时，控件使用默认蓝紫配色：主色 `#246bfd`、辅色 `#6854dc`。如需在调用时使用业务品牌色，可通过 HTML 属性传入：

```html
<customer-service-widget
  channel-id="website-main"
  caller-user-id="user_10001"
  primary-color="#0f766e"
  secondary-color="#14b8a6"
></customer-service-widget>
```

也可以动态更新已经创建的控件：

```js
widget.configure({
  primaryColor: '#0f766e',
  secondaryColor: '#14b8a6'
})
```

左下角显示示例：

```html
<customer-service-widget
  channel-id="website-main"
  caller-user-id="user_10001"
  position="left"
></customer-service-widget>
```

## 六、公开鉴权流程

控件会自动完成以下流程：

```text
加载控件
  → POST /api/v1/biz/auth/public-token
  → 服务端检查公开渠道、Origin、IP 频率和每日配额
  → 返回有效期约 15 分钟的短期 Token
  → 控件调用会话与聊天 API
  → Token 到期前自动重新申请
```

接入方无需处理这些请求。

`caller-user-id` 来自前端，只用于区分用户和隔离会话，不能作为可信身份凭证。当前客服以 QA 知识库咨询为主，不应通过公开模式返回订单、账号、会员资料等敏感信息。

## 七、IP 频率限制和每日配额

- 限制维度：`渠道 ID + 客户端 IP`。
- 统计范围：公开 Token、会话、历史记录和聊天等所有公开控件请求。
- 每分钟限制采用 UTC 分钟窗口。
- 每日配额采用 UTC 自然日，计数存储在 Redis，服务重启不会清空。
- 超出限制时接口返回 HTTP `429` 和 `Retry-After`。
- Redis 不可用时公开请求返回 HTTP `503`，避免限流失效后无限放行。

一次新咨询通常至少产生三次请求：申请 Token、创建会话、发送消息。配置每日配额时需要预留历史列表、续聊和 Token 刷新产生的请求量。

## 八、来源域名限制

建议在 GM 中填写实际业务站点的完整 Origin：

```text
https://www.example.com
https://m.example.com
```

Origin 只包含协议、域名和端口，不包含页面路径。以下写法不正确：

```text
https://www.example.com/customer-service/page
```

来源限制用于减少其他网站直接复用渠道 ID，但它不是用户身份认证机制；非浏览器客户端仍可以伪造 Origin，因此必须同时保留 IP 限流和每日配额。

## 九、CSP 配置

如果业务站点使用了 Content Security Policy，需要放行客服域名：

```http
Content-Security-Policy:
  script-src 'self' https://testapi.realcopyright.cn;
  connect-src 'self' https://testapi.realcopyright.cn;
```

## 十、会话行为

- 每个会话在服务端保留最近 50 条消息。
- 当前会话按“租户 + 渠道 + 用户”保存在浏览器 `localStorage` 中。
- 刷新页面后可以恢复当前会话并继续咨询。
- “新对话”会创建新会话，旧会话仍保留在历史列表。
- “清空消息”永久删除当前会话消息，但保留会话。
- “删除会话”永久删除会话及消息。

## 十一、保留的服务端换票模式

原有服务端换票方式仍然可用，适合未来需要可信用户身份或敏感业务的渠道。使用时显式设置 `token-endpoint`：

```html
<customer-service-widget
  channel-id="account-center"
  caller-user-id="user_10001"
  token-endpoint="/api/customer-service/token"
></customer-service-widget>
```

只要没有设置 `token-endpoint`，控件就会默认使用新的公开控件模式。

## 十二、问题排查

| 现象 | 常见原因 | 处理方式 |
| --- | --- | --- |
| 控件按钮未显示 | 脚本被 CSP 或网络拦截 | 检查脚本请求和 `script-src` |
| 返回 403 | 渠道未开启公开模式、已停用，或 Origin 不在允许列表 | 检查 GM 渠道配置 |
| 返回 429 | 当前 IP 超出分钟限制或每日配额 | 等待 `Retry-After`，或合理调整渠道限额 |
| 返回 503 | Redis 不可用，限流无法可靠执行 | 检查 Redis 连接和服务状态 |
| 刷新后没有恢复会话 | 用户 ID 或渠道 ID 变化，或浏览器禁用存储 | 检查属性和 `localStorage` |
| 消息一直等待 | SSE 被代理缓存或连接超时 | 关闭代理缓冲并提高超时时间 |

服务检查地址：

- `https://testapi.realcopyright.cn/health`
- `https://testapi.realcopyright.cn/customer-service-widget.js`
