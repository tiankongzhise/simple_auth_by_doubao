# 简单鉴权服务开发文档

## 1. 技术栈

- 语言：Go 1.22+。
- HTTP：标准库 `net/http`。
- PostgreSQL：`github.com/jackc/pgx/v5/pgxpool`。
- Redis：`github.com/redis/go-redis/v9`。
- 环境变量：`github.com/joho/godotenv`。
- 密码 hash：`golang.org/x/crypto/bcrypt`。
- JWT：`github.com/golang-jwt/jwt/v5`。
- UI：Go embed 静态文件，无前端构建链。

## 2. 项目结构

```text
.
├── cmd/auth-service/main.go
├── internal/auth
├── internal/config
├── internal/httpapi
├── internal/limiter
├── internal/service
├── internal/store
├── internal/token
├── web
├── docs/product.md
├── docs/development.md
└── README.md
```

模块职责：

- `config`：读取 `.env` 与系统环境变量，解析 TTL、数据库、Redis、开发模式配置。
- `store`：PostgreSQL 持久化、建表迁移、服务 CRUD、token 事务更新。
- `token`：JWT 生成、解析、token version 校验辅助。
- `limiter`：Redis QPS/QPM 限流。
- `auth`：管理员密码校验、管理员会话 JWT。
- `service`：组合业务逻辑，确保 PG 成功后再同步 Redis。
- `httpapi`：路由、请求响应、Cookie、来源地址校验、UI 静态资源。

## 3. 配置项

`.env` 至少包含：

```env
SERVER_PORT=8080

ADMIN_USER=admin
ADMIN_PASSWORD_HASH=$2a$10$...

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

dev_model=false
DEV_IP=127.0.0.1,::1
```

兼容大小写读取 `dev_model` 与 `DEV_MODEL`。

## 4. PostgreSQL 设计

启动时执行幂等迁移：

```sql
CREATE TABLE IF NOT EXISTS services (
  id BIGSERIAL PRIMARY KEY,
  service_name TEXT NOT NULL UNIQUE,
  service_url TEXT NOT NULL UNIQUE,
  authorization_code_hash TEXT NOT NULL,
  authorization_code_masked TEXT NOT NULL,
  qps INTEGER NOT NULL DEFAULT 0,
  qpm INTEGER NOT NULL DEFAULT 0,
  access_token TEXT NOT NULL DEFAULT '',
  refresh_token TEXT NOT NULL DEFAULT '',
  access_token_expires_at BIGINT,
  refresh_token_expires_at BIGINT,
  token_version BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
  updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);
```

约束：

- `service_name` 唯一。
- `service_url` 保存规范化 origin，唯一。
- 永久授权码只保存 bcrypt hash 和 masked 值。
- 当前 access/refresh JWT 明文保存。
- 所有时间字段保存 Unix 秒级时间戳；接口响应同时返回时间戳和北京时间字符串。

## 5. Redis 设计

key 前缀来自 `REDIS_KEY_PREFIX`。

- `service:name:{serviceName}`：缓存服务记录 JSON，TTL 5 分钟。
- `service:id:{id}`：缓存服务记录 JSON，TTL 5 分钟。
- `rate:{serviceID}:sec:{unixSecond}`：QPS 计数，TTL 2 秒。
- `rate:{serviceID}:min:{unixMinute}`：QPM 计数，TTL 70 秒。

token 刷新顺序：

1. 生成新 access/refresh JWT。
2. 在 PG 事务中更新 token、过期时间、`token_version = token_version + 1`。
3. PG 成功后删除旧 Redis 缓存并写入新缓存。
4. 如果 Redis 写失败，接口返回成功但记录错误日志；下一次校验可从 PG 回源。

token 校验、token 刷新和管理端刷新 token 必须强制读取 PG 最新记录，不能只信任 Redis 缓存，以保证旧 token 在 PG 更新成功后立即失效。

## 6. Token 与会话

### 管理员会话

管理员登录成功后签发 `authSession` JWT：

- `sub=admin`
- `typ=admin`
- 默认有效期 12 小时。
- Cookie 为 HttpOnly、SameSite=Lax、Path=/。

### 服务 token

access token claims：

- `sub=<serviceID>`
- `serviceName=<serviceName>`
- `typ=access`
- `tokenVersion=<tokenVersion>`
- 标准 `iat`、`exp`

refresh token claims：

- `sub=<serviceID>`
- `serviceName=<serviceName>`
- `typ=refresh`
- `tokenVersion=<tokenVersion>`
- 标准 `iat`、`exp`

刷新时 tokenVersion 递增。校验时 JWT 合法且未过期后，还必须和 PG/Redis 当前 token 字符串、tokenVersion 同时匹配。

## 7. 来源地址校验

注册和修改服务时，将 `serviceUrl` 规范化为 origin：

- 输入必须是绝对 URL。
- 只允许 `http` 与 `https`。
- 保存 `scheme://host[:port]`。
- host 小写。

服务 API 来源识别：

1. 优先读取 `Origin`。
2. 没有 `Origin` 时读取 `Referer` 并提取 origin。
3. 与服务的 `serviceUrl` 完全相等才允许。

开发模式：

- 请求头 `model` 等于 `dev`。
- 配置 `dev_model=true`。
- 请求来源 IP 命中 `DEV_IP`。
- 满足三项时跳过来源地址校验。

## 8. API 实现细节

统一响应：

```json
{
  "error": "错误说明"
}
```

或业务成功 JSON。

路由：

- `/`：UI 首页。
- `/api/public/usage`
- `/api/admin/login`
- `/api/admin/logout`
- `/api/admin/services`
- `/api/admin/services/{id}`
- `/api/admin/services/{id}/tokens/refresh`
- `/api/token/exchange`
- `/api/token/refresh`
- `/api/auth/verify`

所有 JSON 请求体限制大小为 1 MiB。

`serviceName` 支持中文。所有进入业务层的服务名都先执行 URL 百分号解码再 trim、保存或比对；用于请求头 `Service-Name` 时，调用方建议传递 `encodeURIComponent(serviceName)` 的结果。

公共说明接口 `/api/public/usage` 无需登录，返回机器可读 JSON，覆盖可用服务 API、请求头、请求体、返回字段、错误码、中文服务名编码规则和开发模式提醒。

## 9. 测试策略

单元测试：

- 授权码生成、长度校验、masked 展示。
- bcrypt 授权码校验。
- URL origin 规范化。
- 开发模式 IP 判断。
- JWT 生成和 claim 校验。
- Redis 限流算法可以用抽象接口或最小集成测试覆盖。

API 测试：

- 管理员登录失败和成功。
- 注册服务重复名称、重复地址返回 `409`。
- 永久授权码换 token 成功和失败。
- refresh 后旧 access/refresh token 失效。
- 地址不匹配返回 `403`。
- 开发模式跳过地址校验但不跳过 token 校验。
- QPS/QPM 超限返回 `429`。

手动测试：

- 通过 UI 完成登录、注册、编辑、展示 token、刷新 token。
- 用 curl 调用 exchange、refresh、verify。
- 重启服务后 PG 数据仍存在，Redis 缓存可自动回源恢复。
