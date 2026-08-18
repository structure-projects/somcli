# Gin 数据校验规则

> 适用场景：Gin 项目的请求参数校验、DTO 校验、业务校验。

## 硬约束

- **MUST** 所有入参 DTO 使用 `go-playground/validator` 标签校验。
- **MUST** 在 handler 层完成参数绑定和基础校验。
- **MUST** 业务规则校验放在 service 层。
- **禁止** 在 handler 中手写 `if req.Name == ""` 逐字段校验。

## 请求绑定与校验

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New()

type CreateUserReq struct {
    Username string `json:"username" binding:"required,min=3,max=32"`
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
    Age      int    `json:"age" binding:"gte=0,lte=150"`
    Phone    string `json:"phone" binding:"omitempty,len=11,numeric"`
    Role     string `json:"role" binding:"required,oneof=admin user guest"`
}

func (h *UserHandler) Create(c *gin.Context) {
    var req CreateUserReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, ErrorResponse{Code: 40001, Message: formatValidationError(err)})
        return
    }
    // 调用 service
}
```

## 常用校验标签

| 标签 | 说明 | 示例 |
|---|---|---|
| `required` | 必填 | `binding:"required"` |
| `min=n` / `max=n` | 最小/最大长度或值 | `binding:"min=3,max=32"` |
| `len=n` | 精确长度 | `binding:"len=11"` |
| `email` | 邮箱格式 | `binding:"email"` |
| `url` | URL 格式 | `binding:"url"` |
| `gte=n` / `lte=n` | 大于等于 / 小于等于 | `binding:"gte=0,lte=150"` |
| `oneof=a b c` | 枚举值 | `binding:"oneof=admin user"` |
| `numeric` | 数字字符串 | `binding:"numeric"` |
| `alpha` / `alphanum` | 字母 / 字母数字 | `binding:"alpha"` |
| `omitempty` | 空值时跳过其他校验 | `binding:"omitempty,email"` |
| `dive` | 校验切片/数组元素 | `binding:"required,dive,min=1"` |

## 自定义校验器

```go
// 注册自定义校验
func init() {
    validate.RegisterValidation("phone", validatePhone)
}

func validatePhone(fl validator.FieldLevel) bool {
    phone := fl.Field().String()
    matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, phone)
    return matched
}

// 使用
type Req struct {
    Phone string `json:"phone" binding:"required,phone"`
}
```

## 校验错误格式化

```go
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

func formatValidationError(err error) []ValidationError {
    var errors []ValidationError
    if ve, ok := err.(validator.ValidationErrors); ok {
        for _, fe := range ve {
            errors = append(errors, ValidationError{
                Field:   fe.Field(),
                Message: getErrorMessage(fe),
            })
        }
    }
    return errors
}

func getErrorMessage(fe validator.FieldError) string {
    switch fe.Tag() {
    case "required":
        return "此字段为必填项"
    case "min":
        return fmt.Sprintf("最小长度为 %s", fe.Param())
    case "max":
        return fmt.Sprintf("最大长度为 %s", fe.Param())
    case "email":
        return "邮箱格式不正确"
    default:
        return fmt.Sprintf("校验失败: %s", fe.Tag())
    }
}
```

## 业务校验（Service 层）

```go
func (s *userServiceImpl) Create(ctx context.Context, req *CreateUserReq) (*model.User, error) {
    // 业务规则校验（非字段级别，无法用 validator 标签表达）
    exists, err := s.userRepo.ExistsByEmail(ctx, req.Email)
    if err != nil {
        return nil, fmt.Errorf("check email exists: %w", err)
    }
    if exists {
        return nil, NewAppError(40901, "邮箱已被注册")
    }

    // 创建用户
    user := &model.User{
        Username: req.Username,
        Email:    req.Email,
    }
    // ...
}
```

## 分页参数校验

```go
type PageRequest struct {
    Page     int `json:"page" form:"page" binding:"gte=1" default:"1"`
    PageSize int `json:"pageSize" form:"pageSize" binding:"gte=1,lte=100" default:"20"`
}

// 默认值处理
func (h *UserHandler) List(c *gin.Context) {
    var req PageRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        c.JSON(400, ErrorResponse{Code: 40001, Message: "参数校验失败"})
        return
    }
    // 默认值
    if req.Page == 0 {
        req.Page = 1
    }
    if req.PageSize == 0 {
        req.PageSize = 20
    }
    // ...
}
```

## 禁止事项

- **禁止** 仅在前端做校验（后端 MUST 独立校验所有入参）。
- **禁止** 在 handler 中手写 `if` 逐字段校验（使用 validator 标签）。
- **禁止** 返回原始校验错误给前端（必须格式化后返回）。
- **禁止** 依赖 `ShouldBindJSON` 的默认错误信息（中文项目须自定义错误消息）。
