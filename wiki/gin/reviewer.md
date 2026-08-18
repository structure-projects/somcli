# Gin 评审规则

> 适用场景：审查 Gin 项目的 PR、diff、设计文档。

## 评审顺序（按优先级）

1. 包名与目录结构 → 2. 分层依赖方向 → 3. Handler 职责 → 4. Service 接口定义 → 5. Repository 数据访问 → 6. 错误处理 → 7. 依赖注入 → 8. 配置管理 → 9. 中间件 → 10. 测试 → 11. 文档

## 硬性驳回项（MUST-FIX）

### 包名与结构
- 包名使用大写、下划线、复数形式。
- `main` 函数放在非 `cmd/` 目录下。
- 业务代码放在 `cmd/` 或 `pkg/` 中。

### 分层破坏
- Handler 直接操作数据库（`c.MustGet("db").(*gorm.DB)`）。
- Handler 包含业务逻辑（超过参数绑定 + 调用 + 渲染）。
- Service 引用 `gin.Context` 或 HTTP 相关类型。
- Repository 接口放在实现所在的子包中（应在 `repository/` 包）。

### 错误处理
- 使用 `panic` 处理请求级业务错误。
- 忽略错误（`_ = fn()` 无注释说明）。
- 使用 `log.Fatal` 在非初始化阶段退出进程。
- 未将 `gorm.ErrRecordNotFound` 映射为业务哨兵错误。

### 依赖注入
- 使用全局变量持有 DB、Logger 等依赖。
- 使用 `init()` 函数初始化依赖。

### 配置管理
- 硬编码数据库连接串、端口号、密钥等。
- 配置文件未通过 Viper 或环境变量加载。

### 数据库
- 生产环境使用 `AutoMigrate`。
- 未设置数据库连接池参数（MaxOpenConns、MaxIdleConns、ConnMaxLifetime）。

### 测试
- 新功能无单元测试。
- 修改功能未同步修改测试。
- Mock 自己项目的 Repository/Service。
- 使用 `time.Sleep` 等待异步操作。
- 僵尸断言（只 `assert.NotNil` 或只检查 200）。

### 中间件
- Recovery 中间件未注册（会导致 panic 时进程崩溃）。
- CORS 配置使用 `*` 通配符（生产环境）。

## 建议性反馈（SHOULD-FIX）

- 函数体超过 50 行，建议拆分。
- 函数参数超过 5 个，建议用 struct 封装。
- 导出符号缺少文档注释。
- 使用裸 `return` 而非显式返回值（named return 场景）。
- `interface{}` 类型使用，应用 `any` 或具体类型。
- 字符串拼接使用 `+` 而非 `fmt.Sprintf` 或 `strings.Builder`（循环场景）。
- 使用 `go func()` 启动 goroutine 但未 recover。

## NIT（不影响功能，建议改进）

- 变量命名不符合 Go 惯例（如 `user_id` 应为 `userID`）。
- 注释中的拼写错误。
- import 分组不规范（标准库、第三方、本地 应分三组空行分隔）。

## 已知常见陷阱（识别但不一定驳回）

- `gin.Context` 在 goroutine 中使用（应使用 `c.Copy()`）。
- GORM 的 `Updates` 对零值不生效（应使用 `Select` 或 map）。
- Viper 的 `AutomaticEnv` 会将 `.` 替换为 `_`（如 `database.host` → `DATABASE_HOST`）。
- Wire 注入循环依赖导致编译失败。

## 反馈格式

每条反馈含：
- **位置**：`file:line`
- **级别**：`MUST-FIX` / `SHOULD-FIX` / `NIT` / `QUESTION`
- **依据**：引用规则条目
- **建议**：可落地的修改方案
