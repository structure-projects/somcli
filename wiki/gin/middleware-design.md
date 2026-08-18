# Gin 中间件设计

> 💡 通用约定见 wiki/_common/coding-conventions.md，本文档只保留栈特有约束。

> 适用场景：设计与编写 Gin 中间件（middleware）时生效。

## 硬约束

- 中间件签名 MUST 为 `func(c *gin.Context)`，符合 `gin.HandlerFunc`。
- 链路继续 MUST 调用 `c.Next()`，链路中断 MUST 调用 `c.Abort()`。
- **禁止** 在中间件中 `panic` 处理请求级错误（交给 Recovery 中间件）。
- **禁止** 在中间件中编写业务逻辑，只做横切关注点（鉴权、日志、限流、CORS）。
- 中间件 MUST 无状态或线程安全，禁止持有请求级可变状态。

## 中间件链执行模型

Gin 采用线性链模型，`c.Next()` 之前的代码在请求阶段执行，之后的代码在响应阶段执行：

```go
func Logger() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()                       // 进入下一个中间件 / handler
        latency := time.Since(start)
        log.Printf("%s %s %d %v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
    }
}
```

| 行为 | API | 说明 |
|---|---|---|
| 继续链路 | `c.Next()` | 同步执行后续中间件，阻塞返回后才继续 |
| 中断链路 | `c.Abort()` | 阻止后续 handler 执行，已写入的响应保留 |
| 中断并写响应 | `c.AbortWithStatusJSON(code, obj)` | 原子操作，避免竞态 |
| 跳过本中间件 | `c.Skip()`（自定义 return） | 提前 return 即可 |

**MUST** 鉴权失败时使用 `c.AbortWithStatusJSON`，禁止仅 `c.Abort()` 后继续写响应。

## 注册方式

```go
r := gin.New()
r.Use(gin.Recovery())        // panic 兜底，MUST 第一个注册
r.Use(gin.Logger())          // 请求日志
r.Use(middleware.Cors())     // 跨域
r.Use(middleware.Auth())     // 鉴权

// 路由级中间件
r.GET("/admin", middleware.RequireAdmin(), adminHandler)
```

- **MUST** `gin.Recovery()` 全局注册且排在最前，保证 panic 不崩进程。
- **SHOULD** 全局中间件用 `r.Use()`，路由级中间件作为 `r.GET()` 的中间参数。
- **MAY** 按 RouterGroup 分组注册以缩小中间件作用域。

## Context 值传递

```go
// 设置
c.Set("userID", userID)
c.Set("requestID", uuid.NewString())

// 读取（handler 层）
userID, exists := c.Get("userID")
uid, ok := userID.(int64)
```

| 规则 | 级别 |
|---|---|
| 跨层传递数据 MUST 用 `c.Set` / `c.Get` | MUST |
| 类型断言 MUST 检查 `ok`，禁止裸断言 | MUST |
| **禁止** 在 service 层读取 `gin.Context` 的值 | MUST |
| requestID 等 trace 字段 SHOULD 在入口中间件注入 | SHOULD |

## 常见中间件实现

### Recovery（MUST 全局）

```go
func Recovery() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if rec := recover(); rec != nil {
                c.AbortWithStatusJSON(500, gin.H{"code": 500, "message": "internal error"})
                log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
            }
        }()
        c.Next()
    }
}
```

### CORS

```go
func Cors() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*")
        c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}
```

- **SHOULD** 生产环境使用 `github.com/gin-contrib/cors` 并配置白名单，禁止 `*`。

### 鉴权（JWT）

```go
func Auth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        claims, err := jwt.Parse(token)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "unauthorized"})
            return
        }
        c.Set("userID", claims.UserID)
        c.Next()
    }
}
```

## 中间件顺序约定

| 顺序 | 中间件 | 原因 |
|---|---|---|
| 1 | Recovery | 兜底所有 panic |
| 2 | RequestID / Trace | 后续日志可关联 |
| 3 | Logger | 记录完整请求 |
| 4 | CORS | 预检请求需早返回 |
| 5 | RateLimiter | 鉴权前挡流量 |
| 6 | Auth | 注入用户身份 |

## 提交前自检

- [ ] `gin.Recovery()` 是否全局注册且首位？
- [ ] 鉴权失败是否用 `c.AbortWithStatusJSON`？
- [ ] `c.Get` 返回值是否做了类型断言与 `ok` 检查？
- [ ] 中间件是否无业务逻辑、无请求级可变状态？
- [ ] CORS 生产环境是否避免 `*`？

## 关联

- Wiki：`wiki/gin/error-handling.md` `wiki/gin/testing-strategies.md`
- 通用：`wiki/_common/logging.md` `wiki/_common/security.md`
