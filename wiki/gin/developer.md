# Gin 开发规则

> 💡 **通用编码约定**：命名 / 注释 / 异常 / 日志 / 统一响应 / 参数校验 / 依赖注入 / 空值 / 日期时间 / 集合等通用约定见 `wiki/_common/coding-conventions.md`。
> 本文档只保留 **栈特有** 的约束。

> 适用场景：编写 Go/Gin 代码时始终生效。

## 硬约束

- Go 版本 MUST >= 1.21，使用 `go.mod` 管理依赖。
- 包名 MUST 全部小写、单数、无下划线、无驼峰。
- 导出符号 MUST 有文档注释（`// FuncName does X.`）。
- 错误 MUST 被处理（`errcheck` 零容忍），禁止 `_ = someFunc()` 吞错误。
- 函数参数 MUST <= 5，超过用 struct 封装。

## 代码风格

- **MUST** 使用 `gofmt` / `goimports` 格式化代码。
- **MUST** 变量命名：驼峰式，缩写全大写（`userID`、`HTTPServer`、`parseURL`）。
- **SHOULD** 函数体不超过 50 行，复杂逻辑拆分辅助函数。
- **SHOULD** 避免裸 `return`（Named Return 仅用于 defer 场景）。

## Handler 层（HTTP 层）

```go
type UserHandler struct {
    userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
    return &UserHandler{userService: userService}
}

func (h *UserHandler) GetUser(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(400, gin.H{"error": "invalid id"})
        return
    }
    user, err := h.userService.GetByID(c.Request.Context(), id)
    if err != nil {
        // 统一错误处理
        c.JSON(500, gin.H{"error": "internal error"})
        return
    }
    c.JSON(200, user)
}
```

**Handler 约束**：
- **MUST** 只做三件事：绑定参数 → 调用 service → 渲染响应。
- **MUST** 使用 `c.Request.Context()` 传递上下文给 service 层。
- **禁止** 在 handler 中编写业务逻辑、直接操作数据库。
- **禁止** 使用 `panic` 处理请求级错误。

## Service 层（业务逻辑）

```go
// 接口定义
type UserService interface {
    GetByID(ctx context.Context, id int64) (*model.User, error)
    Create(ctx context.Context, req *CreateUserReq) (*model.User, error)
}

// 实现
type userServiceImpl struct {
    userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
    return &userServiceImpl{userRepo: userRepo}
}

func (s *userServiceImpl) GetByID(ctx context.Context, id int64) (*model.User, error) {
    user, err := s.userRepo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get user by id %d: %w", id, err)
    }
    return user, nil
}
```

**Service 约束**：
- **MUST** 定义接口（interface），实现放独立 struct。
- **MUST** 使用 `context.Context` 作为第一个参数。
- **MUST** 使用 `fmt.Errorf("...: %w", err)` 包装错误，保留调用链。
- **禁止** 引用 `gin.Context` 或任何 HTTP 框架类型。

## Repository 层（数据访问）

```go
// 接口（在 repository 包）
type UserRepository interface {
    FindByID(ctx context.Context, id int64) (*model.User, error)
    Save(ctx context.Context, user *model.User) error
    Delete(ctx context.Context, id int64) error
}

// GORM 实现（在 repository/gorm 包）
type userRepoGorm struct {
    db *gorm.DB
}

func NewUserRepoGorm(db *gorm.DB) UserRepository {
    return &userRepoGorm{db: db}
}

func (r *userRepoGorm) FindByID(ctx context.Context, id int64) (*model.User, error) {
    var user model.User
    err := r.db.WithContext(ctx).First(&user, id).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, ErrNotFound
    }
    return &user, err
}
```

**Repository 约束**：
- **MUST** 接口在 `repository/` 包，实现在 `repository/gorm/` 等子包。
- **MUST** 使用 `context.Context` 作为第一个参数。
- **MUST** 将 `gorm.ErrRecordNotFound` 映射为业务错误（`ErrNotFound`）。
- **禁止** 在 repository 中编写业务逻辑。

## 错误处理

```go
// model/errors.go
var (
    ErrNotFound       = errors.New("resource not found")
    ErrDuplicate      = errors.New("resource already exists")
    ErrUnauthorized   = errors.New("unauthorized")
    ErrForbidden      = errors.New("forbidden")
)

// 自定义业务错误
type AppError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Err     error  `json:"-"`
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }
```

**错误约束**：
- **MUST** 在 `model/errors.go` 统一定义哨兵错误。
- **MUST** handler 层捕获所有错误，转换为 HTTP 状态码。
- **SHOULD** 定义 `AppError` 结构体，携带业务错误码。
- **禁止** 返回裸 `errors.New("something wrong")`，应使用预定义错误。

## 依赖注入

- **MUST** 使用构造器注入（`NewXxx(dep1, dep2) *Xxx`）。
- **SHOULD** 使用 Wire 编译时生成依赖注入代码。
- **禁止** 使用全局变量持有依赖。

## 数据库迁移

- **MUST** 使用版本化迁移工具（`golang-migrate` 或 `atlas`）。
- **禁止** 使用 GORM `AutoMigrate` 用于生产环境。
- **MUST** 迁移脚本放 `migrations/` 目录，命名 `000001_create_users.up.sql` / `.down.sql`。

## 测试工作流（MUST）

- 每开发一个功能 **立即** 写单元测试，**单测通过才能做下一个功能**。
- 功能有修改时 **同步修改测试** 并通过。
- 业务完成后写 **业务流程集成测试**，通过才算交付。
- **提交前**：`go test ./...` 全部通过 + `go build ./cmd/server` 编译通过。
- **禁止** 测试/编译失败仍提交。

## 提交前自检

- [ ] 所有导出符号是否有文档注释？
- [ ] handler 是否只做绑定→调用→渲染？
- [ ] service 接口是否定义？实现是否使用 context.Context？
- [ ] 错误是否使用预定义哨兵错误或自定义 AppError？
- [ ] 依赖是否通过构造器注入？
- [ ] `go test -race ./...` 是否全部通过？
- [ ] `golangci-lint run` 是否无错误？
