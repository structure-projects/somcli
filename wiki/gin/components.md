# Gin 组件与依赖速查

> 适用场景：Gin 项目中使用生态组件时的配置参考、最佳实践。

## 核心框架

### Gin

```go
import "github.com/gin-gonic/gin"

// 创建引擎（生产用 gin.New()，不加默认中间件）
r := gin.New()
r.Use(gin.Recovery(), middleware.Logger())

// 启动
r.Run(":8080")  // 开发
// 生产用 http.Server + Graceful Shutdown
```

### GORM

```go
import "gorm.io/gorm"
import "gorm.io/driver/postgres"

// 连接
dsn := "host=localhost user=postgres dbname=mydb port=5432 sslmode=disable"
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    Logger: logger.Default.LogMode(logger.Info),
})

// 连接池配置（生产 MUST 显式设置）
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(100)
sqlDB.SetMaxIdleConns(10)
sqlDB.SetConnMaxLifetime(time.Hour)
```

### Viper（配置管理）

```go
import "github.com/spf13/viper"

viper.SetConfigName("config")    // 文件名（不含扩展名）
viper.SetConfigType("yaml")
viper.AddConfigPath("./configs") // 搜索路径
viper.AutomaticEnv()             // 环境变量覆盖

if err := viper.ReadInConfig(); err != nil {
    log.Fatalf("config error: %v", err)
}

// 结构体反序列化（推荐）
type Config struct {
    Server   ServerConfig   `mapstructure:"server"`
    Database DatabaseConfig `mapstructure:"database"`
}
var cfg Config
viper.Unmarshal(&cfg)
```

### Zap（结构化日志）

```go
import "go.uber.org/zap"

// 生产配置
logger, _ := zap.NewProduction()
defer logger.Sync()

// 开发配置（可读性更好）
logger, _ := zap.NewDevelopment()

// 带字段的结构化日志
logger.Info("user created",
    zap.Int64("user_id", user.ID),
    zap.String("username", user.Username),
)
```

### go-playground/validator

```go
import "github.com/go-playground/validator/v10"

type CreateUserReq struct {
    Username string `json:"username" validate:"required,min=3,max=32"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"gte=0,lte=150"`
}

validate := validator.New()
if err := validate.Struct(req); err != nil {
    // 处理校验错误
}
```

### Wire（依赖注入）

```go
// wire.go (go:build wireinject)
//go:build wireinject

func InitializeServer() (*Server, error) {
    wire.Build(
        NewConfig,
        NewDB,
        repository.NewUserRepo,
        service.NewUserService,
        handler.NewUserHandler,
        NewRouter,
    )
    return &Server{}, nil
}

// 生成代码：wire gen ./internal/...
```

### swaggo/swag（API 文档）

```go
import "github.com/swaggo/gin-swagger"
import "github.com/swaggo/files"

// main.go 头部注解
// @title My API
// @version 1.0
// @host localhost:8080
// @BasePath /api/v1

// handler 注解
// @Summary 获取用户
// @Tags users
// @Param id path int true "用户ID"
// @Success 200 {object} UserVO
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) { ... }

// 注册 Swagger 路由
r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

// 生成：swag init -g cmd/server/main.go
```

## 中间件推荐

| 中间件 | 包 | 用途 |
|---|---|---|
| CORS | `github.com/gin-contrib/cors` | 跨域配置 |
| Rate Limit | `github.com/ulule/limiter/v3` | 限流 |
| Request ID | 自定义或 `github.com/google/uuid` | 链路追踪 |
| JWT Auth | `github.com/golang-jwt/jwt/v5` | 认证 |
| Gzip | `github.com/gin-contrib/gzip` | 压缩 |

## 测试组件

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "net/http/httptest"
)

// httptest 测试 handler
func TestGetUser(t *testing.T) {
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/users/1", nil)
    router.ServeHTTP(w, req)
    assert.Equal(t, 200, w.Code)
}
```

## 禁止事项

- **禁止** 在 handler 中直接使用 `db` 实例（必须通过 service/repository 层）。
- **禁止** 使用 `panic` 处理业务错误（仅用于不可恢复的初始化失败）。
- **禁止** GORM 的 `AutoMigrate` 用于生产环境（用版本化迁移工具）。
- **禁止** 硬编码配置值（必须通过 Viper 或环境变量）。
