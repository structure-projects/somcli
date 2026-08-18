---
name: gin-new-handler
description: |
  当用户要求"新建 Handler/新增接口/写 API/加路由"时触发（gin 栈）。
  按分层规范在 handler 包创建 Handler struct + 方法，并在 router 注册路由。
  MUST 用 c.ShouldBindJSON 绑定参数、c.JSON 渲染响应、c.Request.Context() 传递上下文。

triggers:
  - 新建 Handler
  - 新增 Handler
  - 写 Handler
  - 新增接口
  - 写 API
  - 加路由
  - new handler
  - add api

role: developer
phase: support
delegates-to: coding


allowed-tools: Bash, Read, Write, Edit

related-rules:
  - gin-developer
  - common-core
  - common-api-design
  - common-project-stack-detection

reads-before-action:
  - wiki/gin/developer.md
  - wiki/gin/middleware-design.md
  - wiki/gin/error-handling.md
  - wiki/_common/api-design.md

stack-constraints:
  gin:
    package: "internal/handler"
    struct-naming: "{X}Handler"
    constructor: "New{X}Handler(dep) *{X}Handler"
    bind: "c.ShouldBindJSON(&req)"
    context: "c.Request.Context()"
    response: "c.JSON(code, gin.H{...})"
    forbidden:
      - "在 Handler 编写业务逻辑"
      - "在 Handler 直接操作数据库"
      - "在 Handler 注入 *gorm.DB"
      - "使用 panic 处理请求级错误"

produces:
  - "{X}Handler 结构体与方法（internal/handler/{x}_handler.go）"
  - 路由注册（internal/router/router.go 或对应 router 文件）
  - 对应单元测试（{x}_handler_test.go）

requires:
  - skill: coding
    condition: 对应 Service 已存在
    error: 无 Service 层，MUST 先创建 Service（interface + impl）再写 Handler

human-in-the-loop:
  - API 路径 MUST 与用户确认
  - 请求 / 响应字段 MUST 与用户确认

mode: auto

category: coding
stack: gin
priority: high
---

# gin 新建 Handler

> 在 `internal/handler` 创建 Handler struct + 方法并在 router 注册路由。**MUST 只做绑定 → 调 Service → 渲染响应**。

## 前置条件

- 已识别为 gin 项目（存在 `gin-gonic/gin` 依赖）
- 对应 `{X}Service` 接口 + 实现已存在

## 执行步骤

### 第 1 步：确认 Handler 位置

- 文件：`internal/handler/{x}_handler.go`
- 结构体：`{X}Handler`，构造器：`New{X}Handler(svc {X}Service) *{X}Handler`

### 第 2 步：生成 Handler

```go
package handler

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"

    "yourapp/internal/model"
    "yourapp/internal/service"
)

type UserHandler struct {
    userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
    return &UserHandler{userService: userService}
}

// GetUser 根据ID查询用户
func (h *UserHandler) GetUser(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id"})
        return
    }
    user, err := h.userService.GetByID(c.Request.Context(), id)
    switch {
    case errors.Is(err, model.ErrNotFound):
        c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "用户不存在"})
    case err != nil:
        c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "internal error"})
    default:
        c.JSON(http.StatusOK, user)
    }
}

// CreateUser 创建用户
func (h *UserHandler) CreateUser(c *gin.Context) {
    var req CreateUserReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
        return
    }
    id, err := h.userService.Create(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "internal error"})
        return
    }
    c.JSON(http.StatusCreated, gin.H{"id": id})
}
```

### 第 3 步：注册路由

```go
// internal/router/router.go
func Register(r *gin.Engine, h *handler.UserHandler) {
    api := r.Group("/api/v1")
    {
        api.GET("/users/:id", h.GetUser)
        api.POST("/users", h.CreateUser)
    }
}
```

### 第 4 步：关键约束

| 约束 | 说明 |
|---|---|
| **包名** | `internal/handler` |
| **命名** | `{X}Handler` + `New{X}Handler` |
| **绑定** | MUST `c.ShouldBindJSON(&req)`，校验 err |
| **上下文** | MUST 传 `c.Request.Context()` 给 Service |
| **响应** | MUST `c.JSON(code, ...)`，错误转 HTTP 状态码 |
| **禁止** | 业务逻辑、直接操作 DB、panic 处理请求错误 |

## 产出物

- `{x}_handler.go`
- 路由注册（`router.go`）
- `{x}_handler_test.go`（httptest + mock Service）

## 下一步

完成本技能后 MUST 按以下顺序继续：

1. **本层组件完成** → 调用 `testing` 补全 Handler 测试
2. **全部代码完成** → 调用 `expert-review` 评审
3. **评审通过** → 调用 `ci-gate` 提交
4. **多人协作** → 调用 `ci-pipeline` 提 PR

**推荐下一技能**：`testing`

## 关联

- 前置：Service 层已存在
- 相关：`gin-developer` / `gin-error-handling`
- Wiki：`wiki/gin/developer.md` `wiki/gin/middleware-design.md`
