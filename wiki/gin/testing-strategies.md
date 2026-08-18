# Gin 测试策略

> 💡 通用约定见 wiki/_common/coding-conventions.md，本文档只保留栈特有约束。

> 适用场景：Gin 项目单元测试、Handler 测试、集成测试时生效。

## 硬约束

- 测试文件 MUST 命名 `xxx_test.go`，与被测代码同包。
- 测试函数 MUST 命名 `TestXxx(t *testing.T)`，参数 MUST 为 `*testing.T`。
- **MUST** 使用 `net/http/httptest` 测试 Handler，禁止起真实端口。
- **MUST** 使用接口 + mock 隔离 Service / Repository，禁止测试连真实数据库。
- **禁止** 测试 / 编译失败仍提交。
- 集成测试 MUST 加 `//go:build integration` 构建标签。

## 测试分层

| 层级 | 工具 | 目标 | 是否 mock |
|---|---|---|---|
| Service 单测 | testify + mock | 业务逻辑 | mock Repository |
| Handler 测试 | httptest + gin.TestMode | 路由 / 绑定 / 响应 | mock Service |
| 集成测试 | Testcontainers | 真实 DB + 全链路 | 不 mock |
| 基准测试 | `testing.B` | 热点性能 | 视情况 |

## 表驱动测试（MUST 优先）

```go
func TestUserService_Create(t *testing.T) {
    cases := []struct {
        name    string
        req     *CreateUserReq
        wantErr error
    }{
        {"正常创建", &CreateUserReq{Name: "alice"}, nil},
        {"名称为空", &CreateUserReq{Name: ""}, ErrValidation},
        {"名称超长", &CreateUserReq{Name: strings.Repeat("a", 256)}, ErrValidation},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            svc := NewUserService(mockRepo)
            _, err := svc.Create(context.Background(), tc.req)
            assert.ErrorIs(t, err, tc.wantErr)
        })
    }
}
```

- **MUST** 用 `t.Run(tc.name, ...)` 子测试，失败定位清晰。
- **MUST** 用 `assert` / `require`（testify），禁止手写 `if got != want { t.Fatal() }`。

## Handler 测试（httptest）

```go
func TestUserHandler_GetUser(t *testing.T) {
    gin.SetMode(gin.TestMode)
    mockSvc := new(mocks.UserService)
    mockSvc.On("GetByID", mock.Anything, int64(1)).
        Return(&model.User{ID: 1, Name: "alice"}, nil)

    r := gin.New()
    h := NewUserHandler(mockSvc)
    r.GET("/users/:id", h.GetUser)

    req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    require.Equal(t, http.StatusOK, w.Code)
    mockSvc.AssertExpectations(t)
}
```

| 要点 | 约束 |
|---|---|
| `gin.SetMode(gin.TestMode)` | MUST 关闭冗余日志 |
| `httptest.NewRequest` + `NewRecorder` | MUST 用 httptest，禁止真实端口 |
| 路径参数 | 用 `:id` 路由 + 真实 URL |
| 断言 | `require.Equal` 状态码 + `assert.JSONEq` 响应体 |
| Mock 校验 | `mockSvc.AssertExpectations(t)` 验证调用 |

## Mock 定义

- **MUST** 用接口定义依赖，mock 实现 interface（`mockgen` 生成或手写）。
- **SHOULD** 使用 `github.com/stretchr/testify/mock` + `go.uber.org/mock/mockgen` 生成。
- **禁止** 在单测中连真实数据库验证业务逻辑。

```go
//go:generate mockgen -source=user_service.go -destination=mocks/user_service_mock.go
type UserService interface {
    GetByID(ctx context.Context, id int64) (*model.User, error)
}
```

## 集成测试

```go
//go:build integration

func TestUserRepo_Integration(t *testing.T) {
    db := setupTestDB(t)   // Testcontainers 启动 PG
    repo := NewUserRepoGorm(db)
    // ... 真实 DB 操作
}
```

- **MUST** 加 `//go:build integration` 标签，CI 默认只跑单测。
- **MUST** 用 Testcontainers 启动一次性 DB，测试结束自动清理。

## 测试工作流（MUST）

- 每开发一个功能 **立即** 写单测，单测通过才能做下一个功能。
- 功能修改时 **同步修改测试** 并通过。
- 业务完成后写集成测试，通过才算交付。
- **提交前**：`go test ./...` 全绿 + `go build ./cmd/server` 通过。
- **SHOULD** 提交前跑 `go test -race ./...` 检测数据竞争。

## 覆盖率

- **SHOULD** 核心业务覆盖率 >= 80%，整体 >= 60%。
- **MAY** 用 `go test -coverprofile=cover.out` + `go tool cover -html=cover.out` 查看。

## 提交前自检

- [ ] 测试文件命名 `xxx_test.go` 且与被测同包？
- [ ] Handler 测试用 `httptest` + `gin.TestMode`？
- [ ] 依赖通过接口 mock，未连真实 DB？
- [ ] 表驱动测试覆盖正常 + 边界 + 异常路径？
- [ ] `go test ./...` 全部通过？

## 关联

- Wiki：`wiki/gin/developer.md` `wiki/gin/middleware-design.md`
- 通用：`wiki/_common/testing-strategies.md`
