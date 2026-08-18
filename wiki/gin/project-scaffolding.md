# Gin 项目脚手架规则

> 适用场景：新建 Gin 项目时的初始化步骤、目录创建、依赖配置。

## 初始化步骤（MUST 按顺序执行）

### 1. 初始化 Go Module

```bash
mkdir my-service && cd my-service
go mod init github.com/myorg/my-service
```

### 2. 创建目录结构

```bash
mkdir -p cmd/server
mkdir -p internal/{handler,service,repository,model,config,router,middleware}
mkdir -p internal/repository/gorm
mkdir -p internal/service/impl
mkdir -p pkg
mkdir -p migrations
mkdir -p configs
mkdir -p scripts
```

### 3. 安装核心依赖

```bash
go get github.com/gin-gonic/gin
go get gorm.io/gorm gorm.io/driver/postgres
go get github.com/spf13/viper
go get go.uber.org/zap
go get github.com/go-playground/validator/v10
go get github.com/swaggo/gin-swagger github.com/swaggo/files
go get github.com/golang-jwt/jwt/v5
go get github.com/stretchr/testify
```

### 4. 创建入口文件 `cmd/server/main.go`

```go
package main

import "my-service/internal/config"

// @title My Service API
// @version 1.0.0
// @host localhost:8080
// @BasePath /api/v1
func main() {
    cfg := config.MustLoad()
    // ... 初始化 DB、依赖、路由
}
```

### 5. 创建配置模块 `internal/config/config.go`

```go
package config

import (
    "github.com/spf13/viper"
    "log"
)

type Config struct {
    Server   ServerConfig   `mapstructure:"server"`
    Database DatabaseConfig `mapstructure:"database"`
    JWT      JWTConfig      `mapstructure:"jwt"`
}

type ServerConfig struct {
    Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    User     string `mapstructure:"user"`
    Password string `mapstructure:"password"`
    DBName   string `mapstructure:"dbname"`
    SSLMode  string `mapstructure:"sslmode"`
}

type JWTConfig struct {
    Secret     string `mapstructure:"secret"`
    Expiration int    `mapstructure:"expiration_hours"`
}

func MustLoad() *Config {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./configs")
    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err != nil {
        log.Fatalf("failed to read config: %v", err)
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        log.Fatalf("failed to unmarshal config: %v", err)
    }
    return &cfg
}
```

### 6. 创建默认配置文件 `configs/config.dev.yaml`

```yaml
server:
  port: 8080

database:
  host: localhost
  port: 5432
  user: postgres
  password: postgres
  dbname: mydb
  sslmode: disable

jwt:
  secret: change-me-in-production
  expiration_hours: 24
```

### 7. 创建 Makefile

```makefile
.PHONY: build run test lint swagger

APP_NAME = my-service
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w -X main.Version=$(VERSION)" \
		-o bin/$(APP_NAME) ./cmd/server

run:
	go run ./cmd/server

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

swagger:
	swag init -g cmd/server/main.go -o docs

docker:
	docker build -t $(APP_NAME):$(VERSION) .
```

### 8. 创建 Dockerfile

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
COPY --from=builder /app/configs /app/configs
USER 1000
EXPOSE 8080
CMD ["/app/server"]
```

### 9. 创建 `.golangci.yml`

```yaml
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - gosimple
    - ineffassign
    - gofmt
    - goimports
    - misspell
    - unconvert
run:
  timeout: 5m
```

### 10. 创建 `.air.toml`（热重载配置）

```toml
root = "."
tmp_dir = "tmp"
[build]
  cmd = "go build -o ./tmp/server ./cmd/server"
  bin = "./tmp/server"
  include_ext = ["go", "yaml", "yml"]
  exclude_dir = ["tmp", "vendor"]
```

## 禁止事项

- **禁止** 手动创建 vendor 目录（用 `go mod vendor`）。
- **禁止** 在 `cmd/` 下放业务逻辑（`cmd/` 只做组装和启动）。
- **禁止** 把接口和实现放在同一个文件中。
