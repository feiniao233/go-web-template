# go-web-template

可裁剪的 Go Web 服务脚手架：Gin、GORM、PostgreSQL/MySQL、ClickHouse/TDengine、Redis、MQTT、Redis Stream、可选前端嵌入、显式 SQL 迁移和优雅退出。脚手架仓库保留全部适配器，创建项目时只留下所选实现。

## 创建项目

复制脚手架后，在新项目目录执行一次初始化：

```sh
go run ./cmd/scaffold \
  -module github.com/example/device-service \
  -name device-service \
  -database mysql \
  -telemetry clickhouse \
  -redis=true \
  -mqtt=true \
  -redis-stream=true \
  -frontend=embed
```

支持的选择：

```text
-database  postgres | mysql | none
-telemetry clickhouse | tdengine | none
-redis     true | false
-mqtt      true | false
-redis-stream true | false（要求 -redis=true）
-frontend  embed | none
```

初始化器会先打印裁剪清单；可加 `-dry-run` 只预览。正式执行会替换 module、生成存储装配和 Compose，删除未选择的 Adapter、迁移和初始化器自身，最后执行 `gofmt`、`go mod tidy` 和 `go test ./...`。它不会初始化 Git 或提交代码。

## 运行

要求 Go 1.26。未初始化的模板默认使用 PostgreSQL 和可选 Redis；生成项目会要求已选择的业务库和采集库提供 DSN，并把它们注册到 `/ready`。

```sh
go run ./cmd/server
curl http://localhost:8080/health
curl 'http://localhost:8080/api/v1/notes?page=1&page_size=20'
```

启用 PostgreSQL：

```sh
docker compose up -d
export DATABASE_DSN='postgres://postgres:postgres@localhost:5432/app?sslmode=disable'
export REDIS_URL='redis://localhost:6379/0'
go run ./cmd/server
```

YAML 配置按 `http`、`database`、`telemetry`、`redis`、`mqtt`、`cors` 和 `log` 两级分组，参考 `config.example.yaml`。环境变量仍使用 `HTTP_ADDR`、`DATABASE_DSN` 这类扁平名称。

`config.yaml` 是主配置源，进程环境变量用于部署覆盖。优先级为：进程环境变量 > `config.yaml` > 代码内安全默认值。程序不读取 `.env` 文件。

部署时可显式指定文件路径：

```sh
server -config /etc/my-service/config.yaml
```

也可以使用 `CONFIG_FILE` 设置默认路径。所有 YAML 配置均可通过对应的系统环境变量覆盖；流式接口可将 `HTTP_WRITE_TIMEOUT` 设为 `0s`。

如需区分开发和交付环境，可维护外部的 `configs/dev.yaml`、`configs/release.yaml`，启动时通过 `-config` 明确选择。脚手架不把环境配置嵌入二进制：程序本身已有安全默认值，地址、账号和密钥继续外置，避免修改配置必须重新构建以及敏感值进入二进制。

日志默认以 JSON 输出到 stdout；设置 `LOG_FILE` 后会同时写入 stdout 和自动轮转的日志文件。调用方传入的合法 `X-Request-ID` 会被保留并写入请求上下文，否则服务生成 UUID；业务代码可通过 `requestid.FromContext(ctx)` 读取。

`HTTP_PREFIX` 默认 `/api/v1`，只作用于业务路由；`/health`、`/ready`、`/version` 始终位于根路径。CORS 默认允许所有来源且不携带凭据；生产环境如开启凭据，必须配置明确来源。数据库连接池和 GORM 慢查询日志可通过 `DB_*` 环境变量调整。

## 目录结构

```text
cmd/server                 进程入口与信号处理
internal/bootstrap         手工依赖装配、启动、关闭和资源清理
internal/config            系统环境变量与 YAML 配置
internal/httpapi           Gin 总路由
internal/httpapi/middleware 通用 HTTP 中间件
internal/httpapi/response  统一响应
internal/httpapi/system    健康检查与版本接口
internal/httpapi/note      Notes Handler 与路由
internal/note              Notes 的模型、数据访问和业务服务
internal/platform/database GORM 公共层及 PostgreSQL/MySQL Adapter
internal/platform/clickhouse ClickHouse Native Client
internal/platform/tdengine TDengine WebSocket Client
internal/platform/redis    可选 Redis 客户端，可用于缓存和 Redis Stream
internal/platform/mqtt     MQTT 3.1.1 客户端、重连、发布和订阅
internal/platform/redisstream Redis Stream Consumer Group 消费器
internal/platform/logging  JSON 日志和文件轮转
internal/integration       MQTT 等外部集成的启动、就绪检查和关闭
internal/httpapi/frontend  可选的前端静态资源和 SPA fallback
internal/requestid         Request ID 上下文传递
internal/worker            后台 Worker 启动、异常联动和优雅停止
migrations/postgres        PostgreSQL 显式 SQL 迁移
migrations/mysql           MySQL 显式 SQL 迁移
migrations/clickhouse      ClickHouse 独立迁移目录
migrations/tdengine        TDengine 独立迁移目录
```

## 添加业务模块

1. 在 `internal/<业务名>` 添加模型、具体 Repository 和 Service。
2. 在 `internal/httpapi/<业务名>` 添加该模块的 Handler 和路由注册。
3. 在 `internal/bootstrap` 手工创建 Repository、Service，并通过 `httpapi.Dependencies` 注入。
4. 关系库表变更放入对应的 `migrations/<数据库>`，不要使用 `AutoMigrate`。
5. ClickHouse/TDengine 查询集中放在业务 Adapter 的 `queries/*.sql`，Handler 和 Service 不写 SQL。

MQTT 订阅、Redis Stream Consumer 等常驻任务通过 bootstrap 中的 `workers.Add("名称", func(ctx context.Context) error { ... })` 注册。任一 Worker 返回错误会停止 HTTP 服务并取消其他 Worker；进程退出时 Worker 必须响应 `ctx.Done()`。

## API

```text
GET    /health
GET    /ready
GET    /version
GET    /api/v1/notes?page=1&page_size=20
POST   /api/v1/notes       {"title":"hello","content":"world"}
GET    /api/v1/notes/{id}
PUT    /api/v1/notes/{id}  {"title":"hello","content":"updated"}
DELETE /api/v1/notes/{id}
```

普通成功响应为 `{"code":200,"msg":"ok","data":...}`；分页响应额外包含 `total`、`page`、`page_size`。错误响应使用真实 HTTP 状态，body 为 `{"code":状态码,"msg":"...","data":null}`。创建返回 HTTP 201 但 body `code` 仍为 200，删除返回 HTTP 200。`page_size` 默认 20，最大 100。

## 检查

```sh
make test
make build
make release VERSION=1.0.0 COMMIT=abc123
go run ./cmd/migrate -command up
go run ./cmd/migrate -command down -steps 1
go run ./cmd/migrate -command version
```

`VERSION`、`COMMIT` 通过 Make 变量写入 `/version`；`make release` 默认生成 Linux amd64 的 `dist/server` 和 `dist/server.sha256`，可用 `RELEASE_GOOS`、`RELEASE_GOARCH` 覆盖。`MIGRATE_ON_START` 控制服务启动时是否执行所选关系库迁移。

ClickHouse 运行时使用官方 Native Client；TDengine 只使用 WebSocket Driver。采集表结构依赖具体项目的数据模型，因此脚手架只提供独立迁移目录，不预置通用遥测表。

选择 MQTT 后，服务启动时连接 Broker、掉线自动重连、恢复订阅，并把连接状态加入 `/ready`；业务模块在 `internal/bootstrap` 中通过 `integrations.MQTT.Subscribe` 注册主题。消息回调应尽快写入有界 channel 后返回。

选择 Redis Stream 后会保留 Consumer Group 消费器，业务模块负责传入 stream、group、consumer 和消息处理函数；处理成功后才 ACK，重启时先读取当前 consumer 的 pending 消息。

选择 `-frontend=embed` 后，`internal/httpapi/frontend/dist` 会编译进二进制，未知的非 API 路由回退到 `index.html`。将前端构建产物输出或复制到该目录即可。

## 容器运行

默认 Compose 只启动 PostgreSQL 和 Redis：

```sh
docker compose up -d
```

带应用一起构建和启动：

```sh
docker compose --profile app up -d --build
```

应用镜像使用非 root 用户运行。离线环境可先导出构建后的镜像，部署时不需要 Go 工具链。

定时任务、其他消息队列和认证未加入默认基础设施。
