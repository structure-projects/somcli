# Gin 测试规则

> 适用场景：编写 Gin 项目的单元测试和集成测试。

## 测试工作流（MUST）

- 每开发一个功能 **立即** 写单元测试，**单测通过才能做下一个功能**。
- 功能有修改时 **同步修改测试** 并通过。
- 业务完成后写 **业务流程集成测试**，通过才算交付。
- 覆盖正常 + 异常 + 边界；断言验证行为与数据（**禁止** 僵尸断言）。
- **提交前**：`go test -race ./...` 全部通过 + `go build ./cmd/server` 编译通过。

## 测试分层与命名

| 类型 | 命名 | 说明 |
|---|---|---|
| 单元测试 | `xxx_test.go`（同包） | 不启动外部依赖 |
| 集成测试 | `xxx_integration_test.go` + `//go:build integration` | 使用 Testcontainers 或 docker-compose |
| HTTP 测试 | 使用 `httptest` | 测试 handler |

## 单元测试

```go
package service

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// Mock Repository
type mockUserRepo struct {
    mock.Mock
}

func (m *mockUserRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*model.User), args.Error(1)
}

func TestUserService_GetByID_Success(t *testing.T) {
    // Arrange
    mockRepo := new(mockUserRepo)
    svc := NewUserService(mockRepo)
    expected := &model.User{ID: 1, Username: "test"}
    mockRepo.On("FindByID", mock.Anything, int64(1)).Return(expected, nil)

    // Act
    user, err := svc.GetByID(context.Background(), 1)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected.ID, user.ID)
    assert.Equal(t, expected.Username, user.Username)
    mockRepo.AssertExpectations(t)
}

func TestUserService_GetByID_NotFound(t *testing.T) {
    mockRepo := new(mockUserRepo)
    svc := NewUserService(mockRepo)
    mockRepo.On("FindByID", mock.Anything, int64(999)).Return(nil, ErrNotFound)

    user, err := svc.GetByID(context.Background(), 999)

    assert.Nil(t, user)
    assert.ErrorIs(t, err, ErrNotFound)
}
```

## Handler 测试（httptest）

```go
func TestUserHandler_GetByID(t *testing.T) {
    // Setup
    gin.SetMode(gin.TestMode)
    mockSvc := new(mockUserService)
    handler := NewUserHandler(mockSvc)

    r := gin.New()
    r.GET("/users/:id", handler.GetByID)

    t.Run("success", func(t *testing.T) {
        mockSvc.On("GetByID", mock.Anything, int64(1)).
            Return(&model.User{ID: 1, Username: "test"}, nil).Once()

        w := httptest.NewRecorder()
        req, _ := http.NewRequest("GET", "/users/1", nil)
        r.ServeHTTP(w, req)

        assert.Equal(t, 200, w.Code)
        mockSvc.AssertExpectations(t)
    })

    t.Run("invalid id", func(t *testing.T) {
        w := httptest.NewRecorder()
        req, _ := http.NewRequest("GET", "/users/abc", nil)
        r.ServeHTTP(w, req)

        assert.Equal(t, 400, w.Code)
    })

    t.Run("not found", func(t *testing.T) {
        mockSvc.On("GetByID", mock.Anything, int64(999)).
            Return(nil, ErrNotFound).Once()

        w := httptest.NewRecorder()
        req, _ := http.NewRequest("GET", "/users/999", nil)
        r.ServeHTTP(w, req)

        assert.Equal(t, 404, w.Code)
    })
}
```

## 集成测试

```go
//go:build integration

package repository_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/testcontainers/testcontainers-go"
)

func TestUserRepo_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // 启动 Testcontainer
    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "postgres:16-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER": "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB": "testdb",
        },
    }
    container, _ := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    defer container.Terminate(ctx)

    // 获取连接信息
    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "5432")
    dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=testdb sslmode=disable", host, port.Port())

    db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    db.AutoMigrate(&model.User{}) // 集成测试中可用 AutoMigrate

    repo := gormrepo.NewUserRepoGorm(db)

    t.Run("create and find user", func(t *testing.T) {
        user := &model.User{Username: "test", Email: "test@example.com"}
        err := repo.Save(ctx, user)
        assert.NoError(t, err)
        assert.NotZero(t, user.ID)

        found, err := repo.FindByID(ctx, user.ID)
        assert.NoError(t, err)
        assert.Equal(t, "test", found.Username)
    })
}
```

## Mock 边界

- **只允许** Mock 进程边界（外部 HTTP API、第三方 SDK）。
- **允许** Mock 自己项目的 Repository 接口（在 Service 单测中）。
- **允许** Mock 自己项目的 Service 接口（在 Handler 单测中）。
- **禁止** Mock 标准库或框架核心类型（如 `*gorm.DB`）。

## 必须覆盖的场景

- 正常路径：主流程成功。
- 异常路径：资源不存在、参数无效、权限不足。
- 边界条件：空值、零值、最大值、并发冲突。
- 数据一致性：事务回滚、重复创建。

## 禁止事项

- **禁止** 僵尸断言（只 `assert.NotNil` / 只看 HTTP 200）。
- **禁止** `time.Sleep` 等待异步（用 `sync.WaitGroup` 或 channel）。
- **禁止** 无 `t.Parallel()` 的测试函数间相互依赖。
- **禁止** 集成测试 Mock 数据库/Redis（必须用真实中间件）。
- **禁止** 跳过测试的 `t.Skip` 无注释说明原因。
