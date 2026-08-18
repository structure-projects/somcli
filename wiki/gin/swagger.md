# Gin Swagger/OpenAPI 规则

> 适用场景：Gin 项目的 API 文档生成（swaggo/swag）。

## 硬约束

- **MUST** 使用 `swaggo/swag` + `swaggo/gin-swagger` 生成 OpenAPI 文档。
- **MUST** 每个公开 API handler 有完整的 Swagger 注解。
- **MUST** 在 CI 中验证 `swag init` 无错误（文档生成不失败）。
- **SHOULD** 生产环境可关闭 Swagger UI（通过配置开关）。

## 主入口注解（`cmd/server/main.go`）

```go
// @title           My Service API
// @version         1.0.0
// @description     用户管理微服务
// @termsOfService  https://example.com/terms
//
// @contact.name   API Support
// @contact.email  support@example.com
//
// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT
//
// @host      localhost:8080
// @BasePath  /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token，格式："Bearer {token}"
func main() {
    // ...
}
```

## Handler 注解

```go
// @Summary      创建用户
// @Description  创建新用户并返回用户信息
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        body  body      CreateUserRequest  true  "创建用户请求"
// @Success      201   {object}  UserResponse        "用户创建成功"
// @Failure      400   {object}  ErrorResponse       "参数校验失败"
// @Failure      409   {object}  ErrorResponse       "用户已存在"
// @Failure      500   {object}  ErrorResponse       "服务器内部错误"
// @Security     BearerAuth
// @Router       /users [post]
func (h *UserHandler) Create(c *gin.Context) {
    // ...
}

// @Summary      获取用户详情
// @Tags         用户管理
// @Param        id   path      int64  true  "用户ID"
// @Success      200  {object}  UserResponse
// @Failure      404  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
    // ...
}

// @Summary      分页查询用户
// @Tags         用户管理
// @Param        page      query     int     false  "页码"   default(1)
// @Param        pageSize  query     int     false  "每页条数" default(20)
// @Param        keyword   query     string  false  "搜索关键词"
// @Success      200  {object}  PageResponse{data=[]UserResponse}
// @Security     BearerAuth
// @Router       /users [get]
func (h *UserHandler) List(c *gin.Context) {
    // ...
}
```

## 通用响应结构

```go
// 统一错误响应
type ErrorResponse struct {
    Code    int    `json:"code" example:"40001"`
    Message string `json:"message" example:"参数校验失败"`
}

// 分页响应
type PageResponse struct {
    Total    int64       `json:"total" example:"100"`
    Page     int         `json:"page" example:"1"`
    PageSize int         `json:"pageSize" example:"20"`
    Data     interface{} `json:"data"`
}
```

## 模型注解

```go
type CreateUserRequest struct {
    Username string `json:"username" binding:"required" example:"zhangsan"`
    Email    string `json:"email" binding:"required,email" example:"zhangsan@example.com"`
    Age      int    `json:"age" binding:"gte=0,lte=150" example:"25"`
}

type UserResponse struct {
    ID        int64     `json:"id" example:"1"`
    Username  string    `json:"username" example:"zhangsan"`
    Email     string    `json:"email" example:"zhangsan@example.com"`
    CreatedAt time.Time `json:"createdAt" example:"2024-01-01T00:00:00Z"`
}
```

## 路由注册

```go
import (
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"
    _ "my-service/docs" // swag init 生成的 docs 包
)

func SetupRouter() *gin.Engine {
    r := gin.New()
    // Swagger 路由（可选：通过配置控制是否注册）
    if cfg.Swagger.Enabled {
        r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
    }
    return r
}
```

## CI 集成

```makefile
# Makefile
swagger:
	swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

check-swagger:
	swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
	git diff --exit-code docs/ || (echo "Swagger docs are out of date. Run 'make swagger'." && exit 1)
```

## 禁止事项

- **禁止** 在生产环境默认暴露 Swagger UI（通过配置开关控制）。
- **禁止** 注解中的 `@Param` 类型与实际 handler 参数不一致。
- **禁止** 在公共 API 中省略 `@Security` 注解（需认证的接口）。
- **禁止** `swag init` 生成的 `docs/` 目录加入 `.gitignore`（必须版本管理）。
