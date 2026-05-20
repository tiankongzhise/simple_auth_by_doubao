# simple_auth_by_doubao

一个 Go 实现的简单鉴权服务，内置管理 UI，支持账号密码登录、服务注册、永久授权码换取 JWT token、token 成对刷新、服务来源地址校验、独立 QPS/QPM 限流。

## 功能

- 管理员账号密码来自 `.env`，密码使用 bcrypt hash，不保存明文。
- 服务注册信息持久化到 PostgreSQL。
- 当前 access token 和 refresh token 以 JWT 明文存储在 PostgreSQL，并同步缓存到 Redis。
- 永久授权码只在注册成功时完整展示一次；数据库只保存 hash 和 masked 展示值。
- 服务名称和服务网址都不允许重复。
- token 刷新时 access token 与 refresh token 成对刷新，旧 token 立即失效。
- 鉴权接口使用请求头 `Service-Name` 和 `Access-Token`。
- 所有时间在数据库中保存 Unix 秒级时间戳；接口返回时间戳和北京时间字符串。
- 开发模式可通过 `model: dev` 在指定 IP 下跳过服务地址校验，禁止生产使用。

## 配置

复制并调整 `.env`：

```env
SERVER_PORT=8080

ADMIN_USER=admin
ADMIN_PASSWORD_HASH=$2a$10$replace-with-bcrypt-hash

DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=simple_auth
DB_SSL_MODE=disable

REDIS_ADDR=127.0.0.1:6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=0
REDIS_KEY_PREFIX=simple-auth:

JWT_SECRET=please-change-me
ACCESS_TOKEN_TTL=24h
REFRESH_TOKEN_TTL=720h
ADMIN_SESSION_TTL=12h

dev_model=false
DEV_IP=127.0.0.1,::1
```

生成管理员密码 hash：

```powershell
go run ./cmd/hash-password -- "你的管理员密码"
```

把输出结果填入 `ADMIN_PASSWORD_HASH`。也可以用任意 bcrypt 工具生成 hash。

## 启动

准备 PostgreSQL 数据库和 Redis 后运行：

```powershell
$env:GOCACHE = Join-Path (Get-Location) ".gocache"
go run .
```

访问：

```text
http://127.0.0.1:8080/
```

服务启动时会自动创建 `services` 表。

## 管理 UI

1. 使用 `.env` 中的 `ADMIN_USER` 和对应明文密码登录。
2. 注册服务，填写服务名称、服务网址、QPS、QPM，可选填写 32 位永久授权码。
3. 注册成功后页面会完整展示永久授权码一次，请立即保存。
4. 服务列表展示 masked 永久授权码、当前生效 access token 和 refresh token。
5. 可以修改服务名称、服务网址、QPS、QPM。
6. 点击“一键获取/刷新 Token”会成对刷新 access token 和 refresh token。

## 服务 API

JSON 字段使用 camelCase，请求头使用短横线命名，避免 Nginx 丢弃下划线请求头。

### 永久授权码换 token

```http
POST /api/token/exchange
Content-Type: application/json
Origin: https://billing.example.com
```

```json
{
  "serviceName": "billing",
  "authorizationCode": "abcd1234abcd1234abcd1234abcd1234"
}
```

### refresh token 刷新 token

```http
POST /api/token/refresh
Content-Type: application/json
Origin: https://billing.example.com
```

```json
{
  "serviceName": "billing",
  "refreshToken": "<currentRefreshJwt>"
}
```

### 鉴权校验

```http
POST /api/auth/verify
Service-Name: billing
Access-Token: <currentAccessJwt>
Origin: https://billing.example.com
```

成功响应：

```json
{
  "ok": true,
  "serviceName": "billing"
}
```

## 开发模式

当请求头包含 `model: dev`，并且 `.env` 中 `dev_model=true`，且请求来源 IP 在 `DEV_IP` 列表内时，系统只跳过服务地址校验。

开发模式仍然校验：

- 服务名称。
- 永久授权码。
- access token / refresh token。
- QPS / QPM 限流。

不要在生产环境开启开发模式。

## 构建

仓库包含 `go_build.ps1`，可构建 Linux amd64 静态二进制：

```powershell
.\go_build.ps1
```

## 测试

```powershell
$env:GOCACHE = Join-Path (Get-Location) ".gocache"
go test ./...
```
