# H5 客服聊天控件

控件使用绑定用户和渠道的短期 Token 调用 Biz API。`App Secret` 只保存在渠道服务端，不能出现在 H5、JavaScript 包或浏览器请求中。

## 生产接入流程

1. 在 GM「接入应用」创建应用，取得 `App ID` 和只显示一次的 `App Secret`。
2. 渠道服务端提供自己的换票接口，例如 `POST /api/customer-service/token`。它从登录态确认真实用户 ID，再用 App 凭证调用客服服务换票。
3. H5 控件配置渠道换票地址和当前用户 ID。控件自动缓存短期 Token，并在过期前刷新。

```html
<script src="https://客服前端域名/customer-service-widget.js"></script>
<customer-service-widget
  api-base="https://客服接口域名"
  token-endpoint="https://渠道域名/api/customer-service/token"
  caller-user-id="当前登录用户ID"
  channel-id="h5"
  tenant-id="tenant-a">
</customer-service-widget>
```

渠道换票接口接收控件传入的 `caller_user_id`、`channel_id`、`tenant_id`，但必须用渠道自身登录态校验或覆盖 `caller_user_id`，防止冒用其他用户。渠道服务端随后调用：

```http
POST /api/v1/biz/auth/token
Content-Type: application/json

{
  "app_id": "app_xxx",
  "app_secret": "as_xxx",
  "caller_user_id": "user_10001",
  "channel_id": "h5",
  "tenant_id": "tenant-a"
}
```

成功返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "access_token": "eyJ...",
    "token_type": "Bearer",
    "expires_in": 899,
    "expires_at": "2026-07-13T12:15:00+08:00"
  }
}
```

渠道换票接口将 `data` 原样返回给控件即可。短期 Token 默认 15 分钟有效，绑定应用、用户、渠道和租户；Biz API 以 Token 内的身份为准，不接受前端用请求头覆盖。

## 本地联调

本地可以直接给控件配置 `app-id` 与 `app-secret`，但该方式仅限联调：

```html
<customer-service-widget api-base="http://127.0.0.1:8080" caller-user-id="demo-user" app-id="app_xxx" app-secret="as_xxx" channel-id="h5"></customer-service-widget>
```

演示页：`http://localhost:5173/widget-demo.html`。

## JavaScript 初始化

```js
const widget = CustomerServiceWidget.create({
  apiBase: 'https://客服接口域名',
  tokenEndpoint: 'https://渠道域名/api/customer-service/token',
  callerUserId: currentUser.id,
  channelId: 'h5',
  tenantId: 'tenant-a'
})
```
