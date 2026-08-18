# Gin 架构与设计规则

> 适用场景：Gin 项目架构设计、模块划分、分层决策、技术选型。

## 硬约束

- Go 版本 MUST >= 1.21（推荐 1.23+），使用 `go.mod` 管理依赖。
- 项目 MUST 遵循 Go 标准项目布局：`cmd/`、`internal/`、`pkg/`。
- 包名 MUST 使用小写、单数、简洁命名，禁止下划线和驼峰。
- 模块路径 SHOULD 使用 `github.com/<org>/<repo>` 格式。

## 分层架构（Clean Architecture）

依赖方向（内层不依赖外层）：
```
handler（HTTP 层） → service（业务层） → repository（数据层）
           ↓                    ↓
        model（领域模型，无外部依赖）
```

- **handler/**：Gin handler，负责请求绑定、参数校验、响应渲染。**禁止**在 handler 中写业务逻辑。
- **service/**：业务逻辑层，接口定义在 `internal/service/`，实现可放同包或 `internal/service/impl/`。
- **repository/**：数据访问接口在 `internal/repository/`，实现放 `internal/repository/gorm/` 等子包。
- **model/**：领域实体、值对象、枚举。**禁止**引用任何框架层类型。

## 目录结构（推荐）

```
project/
├── cmd/server/main.go           # 入口，组装依赖
├── internal/
│   ├── handler/                 # Gin handlers
│   │   ├── user_handler.go
│   │   └── middleware/
│   │       ├── auth.go
│   │       ├── cors.go
│   │       └── logger.go
│   ├── service/                 # 业务逻辑
│   │   ├── user_service.go      # 接口
│   │   └── impl/
│   │       └── user_service_impl.go
│   ├── repository/              # 数据访问接口
│   │   ├── user_repo.go
│   │   └── gorm/
│   │       └── user_repo_gorm.go
│   ├── model/                   # 领域模型
│   │   ├── user.go
│   │   └── errors.go
│   ├── config/                  # 配置结构体与加载
│   │   └── config.go
│   └── router/                  # 路由注册
│       └── router.go
├── pkg/                         # 可复用的公共库
├── migrations/                  # 数据库迁移脚本
├── configs/                     # 配置文件模板（YAML）
├── Makefile
├── Dockerfile
└── go.mod
```

## 技术选型（推荐组合）

| 层次 | 推荐方案 | 替代方案 |
|---|---|---|
| Web 框架 | Gin | — |
| ORM | GORM | sqlx / ent |
| 配置管理 | Viper | — |
| 日志 | Zap / zerolog | logrus |
| 校验 | go-playground/validator | ozzo-validation |
| 依赖注入 | Wire（编译时） | dig / fx |
| API 文档 | swaggo/swag | go-swagger |
| 测试 | testify + httptest | ginkgo |
| 热重载 | Air | realize |

## Gin 路由设计

- **MUST** 使用路由分组（`router.Group()`）组织 API。
- **MUST** 中间件按优先级排列：Recovery → Logger → CORS → Auth → RateLimit。
- **SHOULD** API 版本化通过路径前缀：`/api/v1/`。
- **SHOULD** 健康检查与指标端点独立分组，不受 Auth 中间件影响。

```go
// router/router.go
func SetupRouter() *gin.Engine {
    r := gin.New()
    r.Use(gin.Recovery())
    r.Use(middleware.Logger())

    v1 := r.Group("/api/v1")
    {
        v1.Use(middleware.Auth())
        // 注册业务路由
    }
    return r
}
```

## 配置管理

- **MUST** 使用 Viper 统一管理配置，支持 YAML 文件 + 环境变量覆盖。
- **MUST** 定义配置结构体，类型安全加载（禁止 `viper.GetString` 散落各处）。
- **SHOULD** 按环境拆分配置文件：`config.dev.yaml`、`config.prod.yaml`。

## 错误处理

- **MUST** 定义自定义错误类型，包含错误码（code）、消息（message）、HTTP 状态码。
- **MUST** handler 层统一处理错误，转换为标准 JSON 响应。
- **禁止** 在 service/repository 层直接使用 `gin.Context` 或 HTTP 相关类型。

## 安全与中间件

- **MUST** 启用 CORS 中间件，白名单配置允许的源。
- **MUST** JWT 认证中间件放在独立的路由组上。
- **SHOULD** 启用 Rate Limiting（`gin-contrib/limit` 或自定义）。
- **SHOULD** 启用 Request ID 中间件，用于链路追踪。

## 性能约束

- **MUST** 数据库连接池参数（MaxOpenConns、MaxIdleConns、ConnMaxLifetime）在生产环境显式配置。
- **SHOULD** 使用 `sync.Pool` 复用频繁分配的临时对象。
- **SHOULD** 对热点查询使用 Redis 缓存，设置合理 TTL。

## 构建与部署

- **MUST** 使用多阶段 Docker 构建（golang:alpine → scratch/alpine）。
- **MUST** Makefile 提供 `build`、`test`、`lint`、`run` 目标。
- **SHOULD** 使用 `CGO_ENABLED=0` 编译静态链接二进制文件。
