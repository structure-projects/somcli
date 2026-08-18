# Gin CI/CD 规则

> 适用场景：Gin 项目的持续集成、持续部署、流水线配置。

## CI 流水线阶段（MUST 顺序执行）

1. **Lint** → 2. **Test** → 3. **Build** → 4. **Image** → 5. **Deploy**

## Lint 阶段

- **MUST** 使用 `golangci-lint` 进行静态分析。
- **MUST** 配置 `.golangci.yml`，至少启用：`govet`、`staticcheck`、`errcheck`、`gosimple`、`ineffassign`。
- **SHOULD** 启用 `gofmt` / `goimports` 检查代码格式。

```yaml
# .golangci.yml 最小配置
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - gosimple
    - ineffassign
    - gofmt
    - goimports
```

## Test 阶段

- **MUST** 运行 `go test -race -coverprofile=coverage.out ./...`。
- **MUST** 单元测试覆盖率门槛 >= 70%（`-covermode=atomic`）。
- **MUST** 集成测试使用 Testcontainers 或 docker-compose 提供真实依赖。
- **SHOULD** 使用 `gotestsum` 输出 JUnit 格式测试报告。

```makefile
test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | grep total | awk '{print $$3}'

test-integration:
	go test -tags=integration -count=1 ./internal/...
```

## Build 阶段

- **MUST** 使用 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` 编译静态二进制。
- **MUST** 通过 `-ldflags="-s -w"` 去除调试信息，减小二进制体积。
- **SHOULD** 注入版本信息：`-ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"`。

```makefile
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		-o bin/server ./cmd/server
```

## Docker 镜像阶段

- **MUST** 使用多阶段构建，编译阶段用 `golang:1.23-alpine`，运行阶段用 `alpine:3.20`。
- **MUST** 在运行阶段添加 `RUN apk --no-cache add ca-certificates tzdata`。
- **SHOULD** 使用非 root 用户运行：`USER 1000`。

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /app/server /app/server
USER 1000
EXPOSE 8080
CMD ["/app/server"]
```

## GitHub Actions 示例

```yaml
name: CI
on: [push, pull_request]
jobs:
  ci:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: testdb
        ports: ["5432:5432"]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.23" }
      - run: go mod download
      - run: go test -race -coverprofile=coverage.out ./...
      - run: make build
```

## 部署阶段

- **MUST** 部署前执行健康检查：`curl -f http://localhost:8080/health`。
- **SHOULD** 使用优雅关闭（Graceful Shutdown）：监听 `SIGTERM`/`SIGINT`，等待进行中请求完成。
- **SHOULD** 使用 Kubernetes Deployment + Service 或 Docker Compose 编排。

## 禁止事项

- **禁止** 跳过 lint 直接构建。
- **禁止** 测试覆盖率低于 70% 时合并 PR。
- **禁止** 在 CI 中使用 `latest` 标签的 Docker 基础镜像（用固定版本）。
- **禁止** 在镜像中保留构建工具和源代码。
