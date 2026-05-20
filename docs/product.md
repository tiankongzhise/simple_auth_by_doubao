# 简单鉴权服务产品文档

## 1. 产品目标

简单鉴权服务为内部或中小型服务提供集中式 token 管理与请求校验能力。管理员通过内置 UI 登录后，可以注册业务服务、维护服务名称和服务网址、设置独立限流，并为服务发放或刷新 access token 与 refresh token。

系统必须保证：

- 管理员密码不保存明文，只从 `.env` 读取 bcrypt hash。
- 永久授权码只在服务注册成功时完整展示一次。
- 数据库只保存永久授权码 hash 与 masked 展示值。
- 当前生效的 access token 与 refresh token 以 JWT 明文形式存入 PostgreSQL，并同步写入 Redis 缓存。
- token 组刷新时，access token 与 refresh token 必须成对刷新，旧 token 立即失效。
- 服务鉴权请求必须来自注册服务地址；开发模式例外仅用于本地或测试环境。

## 2. 角色与使用场景

### 管理员

管理员使用 `.env` 中配置的账号密码登录 UI，完成以下操作：

- 注册服务。
- 查看服务列表。
- 查看 masked 永久授权码。
- 查看当前生效 access token 与 refresh token。
- 修改服务名称、服务地址、QPS、QPM。
- 一键刷新某个服务的 token 组。

### 已注册服务

已注册服务使用永久授权码换取 access token 和 refresh token。业务请求需要鉴权时，调用鉴权接口并在请求头中传递服务名称和 access token。

## 3. 命名规则

为避免 Nginx 默认丢弃包含下划线的请求头，外部 API 不使用下划线参数名：

- HTTP Header 使用短横线风格，例如 `Service-Name`、`Access-Token`。
- JSON 请求体与响应体使用 camelCase，例如 `serviceName`、`authorizationCode`。
- 特殊开发头按需求保留为 `model: dev`。
- `serviceName` 支持中文。服务名放入请求头时建议先做 URL 百分号编码，例如浏览器使用 `encodeURIComponent(serviceName)`；服务端会先 URL 解码再保存或比对。

## 4. 管理 API

除登录接口外，所有管理接口都需要管理员会话 Cookie：

```http
Cookie: authSession=<jwt>
```

### 4.1 登录

```http
POST /api/admin/login
Content-Type: application/json
```

请求体：

```json
{
  "username": "admin",
  "password": "plainPassword"
}
```

成功响应：

```json
{
  "ok": true,
  "message": "登录成功"
}
```

响应会设置 `authSession` HttpOnly Cookie。

### 4.2 登出

```http
POST /api/admin/logout
Cookie: authSession=<jwt>
```

请求体：无。

成功响应：

```json
{
  "ok": true,
  "message": "已退出登录"
}
```

### 4.3 服务列表

```http
GET /api/admin/services
Cookie: authSession=<jwt>
```

成功响应：

```json
{
  "services": [
    {
      "id": 1,
      "serviceName": "billing",
      "serviceUrl": "https://billing.example.com",
      "authorizationCodeMasked": "abcd************************wxyz",
      "qps": 10,
      "qpm": 300,
      "accessToken": "<jwt>",
      "refreshToken": "<jwt>",
      "accessTokenExpiresAt": 1779415200,
      "accessTokenExpiresAtLocal": "2026-05-22 18:00:00",
      "refreshTokenExpiresAt": 1781920800,
      "refreshTokenExpiresAtLocal": "2026-06-20 18:00:00",
      "createdAt": 1779328800,
      "createdAtLocal": "2026-05-21 18:00:00",
      "updatedAt": 1779328800,
      "updatedAtLocal": "2026-05-21 18:00:00"
    }
  ]
}
```

### 4.4 注册服务

```http
POST /api/admin/services
Cookie: authSession=<jwt>
Content-Type: application/json
```

请求体：

```json
{
  "serviceName": "billing",
  "serviceUrl": "https://billing.example.com",
  "authorizationCode": "可选32位永久授权码",
  "qps": 10,
  "qpm": 300
}
```

字段规则：

- `serviceName` 必填，不可与已有服务重复。
- `serviceName` 支持中文；如果调用方对服务名做了 URL 百分号编码，服务端会自动解码。
- `serviceUrl` 必填，保存为规范化 origin，不可与已有服务重复。
- `authorizationCode` 可选；提供时必须为 32 位，未提供则系统随机生成 32 位。
- `qps`、`qpm` 必填，必须大于等于 0。0 表示不限制该维度。

成功响应会完整返回永久授权码，且只返回这一次：

```json
{
  "service": {
    "id": 1,
    "serviceName": "billing",
    "serviceUrl": "https://billing.example.com",
    "authorizationCodeMasked": "abcd************************wxyz",
    "qps": 10,
    "qpm": 300
  },
  "authorizationCode": "abcd1234abcd1234abcd1234abcd1234"
}
```

### 4.5 修改服务

```http
PUT /api/admin/services/{id}
Cookie: authSession=<jwt>
Content-Type: application/json
```

请求体：

```json
{
  "serviceName": "billing-api",
  "serviceUrl": "https://billing-api.example.com",
  "qps": 20,
  "qpm": 600
}
```

修改后的服务名称和服务地址仍需要全局唯一。

### 4.6 刷新服务 token 组

```http
POST /api/admin/services/{id}/tokens/refresh
Cookie: authSession=<jwt>
```

请求体：无。

成功响应：

```json
{
  "accessToken": "<newAccessJwt>",
  "refreshToken": "<newRefreshJwt>",
  "accessTokenExpiresAt": 1779415200,
  "accessTokenExpiresAtLocal": "2026-05-22 18:00:00",
  "refreshTokenExpiresAt": 1781920800,
  "refreshTokenExpiresAtLocal": "2026-06-20 18:00:00"
}
```

## 5. 服务 API

服务 API 均使用 JSON。正式模式下，服务 API 必须携带 `Origin` 或 `Referer`，系统会用其中的 origin 与注册服务地址匹配。

### 5.1 永久授权码换 token

```http
POST /api/token/exchange
Content-Type: application/json
Origin: https://billing.example.com
```

请求体：

```json
{
  "serviceName": "billing",
  "authorizationCode": "abcd1234abcd1234abcd1234abcd1234"
}
```

成功响应：

```json
{
  "accessToken": "<jwt>",
  "refreshToken": "<jwt>",
  "accessTokenExpiresAt": 1779415200,
  "accessTokenExpiresAtLocal": "2026-05-22 18:00:00",
  "refreshTokenExpiresAt": 1781920800,
  "refreshTokenExpiresAtLocal": "2026-06-20 18:00:00"
}
```

### 5.2 refresh token 刷新 token 组

```http
POST /api/token/refresh
Content-Type: application/json
Origin: https://billing.example.com
```

请求体：

```json
{
  "serviceName": "billing",
  "refreshToken": "<currentRefreshJwt>"
}
```

成功后旧 access token 和旧 refresh token 立即失效。

### 5.3 鉴权校验

```http
POST /api/auth/verify
Service-Name: billing
Access-Token: <currentAccessJwt>
Origin: https://billing.example.com
```

请求体：无。

成功响应：

```json
{
  "ok": true,
  "serviceName": "billing"
}
```

失败响应使用 `401`、`403`、`404` 或 `429`。

## 6. 开发模式

开发模式用于本地或测试环境验证服务 API。当请求满足以下条件时，只跳过服务地址校验：

- 请求头包含 `model: dev`。
- `.env` 中 `dev_model=true`。
- 请求来源 IP 在 `.env` 的 `DEV_IP` 逗号分隔列表内。

开发模式不会跳过：

- 服务名称校验。
- 永久授权码校验。
- access token / refresh token 校验。
- QPS / QPM 限流。

开发模式禁止用于生产环境。

## 7. 错误码约定

- `400 Bad Request`：请求体格式错误或字段校验失败。
- `401 Unauthorized`：管理员未登录、密码错误、token 错误或过期。
- `403 Forbidden`：来源地址不匹配或开发模式 IP 不允许。
- `404 Not Found`：服务不存在。
- `409 Conflict`：服务名称或服务地址重复。
- `429 Too Many Requests`：超过 QPS 或 QPM。
- `500 Internal Server Error`：服务器内部错误。
