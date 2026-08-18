# Gin 错误处理

> 💡 通用约定见 wiki/_common/coding-conventions.md，本文档只保留栈特有约束。

> 适用场景：Gin 项目中错误定义、包装、转换为 HTTP 响应时生效。

## 硬约束

- 错误 MUST 被处理，禁止 `_ = someFunc()` 吞错误。
- 业务错误 MUST 用预定义哨兵错误或 `AppError`，禁止裸 `errors.New`。
- Service 层错误 MUST 用 `fmt.Errorf("...: %w", err)` 包装保留调用链。
- Handler 层 MUST 捕获所有错误并转换为 `c.JSON` 响应，禁止向上抛。
- **禁止** 在 handler 中 `panic` 处理请求级错误（交给 Recovery）。
- **禁止** 错误消息含敏感信息（SQL、堆栈、密钥）。

## 错误模型分层

| 层 | 职责 | 形式 |
|---|---|---|
| Repository | 将 `gorm.ErrRecordNotFound` 等映射为业务错误 | 哨兵错误 |
| Service | 业务规则校验、错误包装（`%w`） | `AppError` / 哨兵 |
| Handler | 错误 → HTTP 状态码 + JSON | `c.JSON(code, gin.H{...})` |
| Recovery | 兜底 panic | 500 + 通用消息 |

## 哨兵错误与自定义错误码

```go
// model/errors.go
var (
    ErrNotFound     = errors.New("resource not found")
    ErrDuplicate    = errors.New("resource already exists")
    ErrUnauthorized = errors.New("unauthorized")
    ErrForbidden    = errors.New("forbidden")
)

// 自定义业务错误（携带业务码）
type AppError struct {
    Code    int    `json:"code"`    // 业务错误码
    Message string `json:"message"`
    Err     error  `json:"-"`       // 包装的底层错误
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func NewAppError(code int, msg string, err error) *AppError {
    return &AppError{Code: code, Message: msg, Err: err}
}
```

- **MUST** 哨兵错误集中在 `model/errors.go` 统一管理。
- **SHOULD** 定义 `AppError` 携带业务错误码，便于前端区分场景。

## 错误包装（Service 层）

```go
func (s *userServiceImpl) GetByID(ctx context.Context, id int64) (*model.User, error) {
    user, err := s.userRepo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get user by id %d: %w", id, err)
    }
    return user, nil
}
```

- **MUST** 包装时保留上下文（操作 + 主键），并用 `%w` 保留原始错误。
- **MUST** 使用 `errors.Is(err, ErrNotFound)` 判断哨兵错误。

## Handler 层错误转响应

```go
func (h *UserHandler) GetUser(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(400, gin.H{"code": 400, "message": "invalid id"})
        return
    }
    user, err := h.userService.GetByID(c.Request.Context(), id)
    switch {
    case errors.Is(err, model.ErrNotFound):
        c.JSON(404, gin.H{"code": 40401, "message": "用户不存在"})
    case err != nil:
        log.Printf("get user failed: %v", err)
        c.JSON(500, gin.H{"code": 500, "message": "internal error"})
        return
    }
    c.JSON(200, user)
}
```

| 错误类型 | HTTP 状态码 | 处理方式 |
|---|---|---|
| 参数绑定失败 | 400 | `c.ShouldBindJSON` 返回 err |
| 未授权 | 401 | 鉴权中间件 `AbortWithStatusJSON` |
| 资源不存在 | 404 | `errors.Is(err, ErrNotFound)` |
| 冲突 / 重复 | 409 | `errors.Is(err, ErrDuplicate)` |
| 内部错误 | 500 | 兜底日志 + 通用消息 |

## 统一错误响应中间件

**SHOULD** 用 `c.Error()` 收集错误 + 统一错误处理中间件，避免每个 handler 重复写 `c.JSON`：

```go
// handler 只收集错误
func (h *UserHandler) GetUser(c *gin.Context) {
    user, err := h.userService.GetByID(c.Request.Context(), id)
    if err != nil {
        _ = c.Error(err)        // 收集，不立即响应
        return
    }
    c.JSON(200, user)
}

// 统一错误处理中间件（最后注册）
func ErrorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        if len(c.Errors) == 0 {
            return
        }
        err := c.Errors.Last().Err
        switch {
        case errors.Is(err, model.ErrNotFound):
            c.JSON(404, gin.H{"code": 40401, "message": "资源不存在"})
        default:
            log.Printf("unhandled error: %v", err)
            c.JSON(500, gin.H{"code": 500, "message": "internal error"})
        }
    }
}
```

## Recovery 兜底

- **MUST** 全局注册 `gin.Recovery()` 或自定义 Recovery，保证 panic 不崩进程。
- panic 时 MUST 返回 500 + 通用消息，**禁止** 把 `recover()` 内容回写客户端。

## 提交前自检

- [ ] 哨兵错误是否集中在 `model/errors.go`？
- [ ] Service 是否用 `fmt.Errorf(": %w", err)` 包装错误？
- [ ] Handler 是否捕获所有错误并转为 `c.JSON`？
- [ ] 错误消息是否不含敏感信息？
- [ ] `gin.Recovery()` 是否全局注册？

## 关联

- Wiki：`wiki/gin/middleware-design.md` `wiki/gin/developer.md`
- 通用：`wiki/_common/error-handling.md`
