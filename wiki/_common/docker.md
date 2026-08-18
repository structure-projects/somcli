# Docker 规范

> 本文档是 structure-projects 生态 Docker 编写的单一来源。
> 所有 Dockerfile 和 docker-compose.yml MUST 遵循本文档。

## Dockerfile 双模板

### 模板 A：后端 Spring Boot（生产标准）

**适用**：所有 Java / Spring Boot 后端服务

```dockerfile
# 后端 Dockerfile（生产标准）
# 位置：<module>-boot/Dockerfile

FROM eclipse-temurin:21-jdk-alpine

# 安装必要工具 + 时区
RUN apk update && \
    apk add --no-cache curl tzdata && \
    rm -rf /var/cache/apk/*

# 时区与环境变量
ENV TZ=Asia/Shanghai
ENV JAVA_OPTS=""
ENV PARAMS=""
ENV APP_PATH=/app/boot/app.jar

# 创建目录
RUN mkdir -p /app/logs /app/boot

# 健康检查脚本
COPY ./liveness.sh /app/
RUN chmod +x /app/liveness.sh

# 复制 JAR（由 CI 外部 mvn package 产出）
COPY ./target/*.jar /app/boot/app.jar

# 健康检查（actuator 端口约定 7777）
HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=60s \
  CMD /app/liveness.sh || exit 1

EXPOSE 8080

ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar $APP_PATH $PARAMS"]
```

**配套 `liveness.sh`**（与 Dockerfile 同目录）：

```bash
#!/bin/sh
# 健康检查脚本（actuator 端口约定 7777）
wget -q -O- http://localhost:7777/actuator/health | grep '"status":"UP"' || exit 1
```

**关键点**：
- ✅ base 镜像：`eclipse-temurin:21-jdk-alpine`（JDK 17+ 用 `17-jdk-alpine`）
- ✅ 时区：`Asia/Shanghai`
- ✅ 三件套：`JAVA_OPTS` / `PARAMS` / `APP_PATH`
- ✅ 健康检查：`HEALTHCHECK` + `liveness.sh`
- ✅ 多阶段构建：**不在 Dockerfile 里做**，由 CI 外部 `mvn package` 产出 JAR

### 模板 B：前端 Nginx

**适用**：所有 Vue / React / 静态前端

```dockerfile
# 前端 Dockerfile（Nginx + envsubst）
# 位置：<ui-module>/Dockerfile

FROM nginx:alpine

# 环境变量（运行时可覆盖）
ENV SCHEME=https
ENV SERVER_HOST=api.prod.structured.cn
ENV SERVER_PORT=443

# 静态文件
ADD ./dist /usr/share/nginx/html

# Nginx 配置模板（用 envsubst 在启动时渲染）
ADD ./nginx.template /etc/nginx/conf.d/source.template

# 启动时渲染模板
ENTRYPOINT ["sh", "-c", "envsubst '$${SERVER_HOST}$${SERVER_PORT}$${SCHEME}' < /etc/nginx/conf.d/source.template > /etc/nginx/nginx.conf && exec nginx -g 'daemon off;'"]

EXPOSE 80
```

**配套 `nginx.template`**：

```nginx
server {
    listen 80;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass ${SCHEME}://${SERVER_HOST}:${SERVER_PORT};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**关键点**：
- ✅ base 镜像：`nginx:alpine`
- ✅ 环境变量渲染：`envsubst`（运行时切换 upstream）
- ✅ SPA 路由：`try_files ... /index.html`

## Dockerfile 编写规则

### MUST

- ✅ **MUST** 使用 `alpine` 变体（减小镜像体积）
- ✅ **MUST** 设置时区 `TZ=Asia/Shanghai`
- ✅ **MUST** 配置 `HEALTHCHECK`
- ✅ **MUST** 用 `ENTRYPOINT` 而非 `CMD`（保持启动参数可注入）
- ✅ **MUST** 清理包管理器缓存（`rm -rf /var/cache/apk/*`）
- ✅ **MUST** 暴露正确端口（后端 8080，前端 80）
- ✅ **MUST** 使用 `.dockerignore`

### MUST NOT

- ❌ **MUST NOT** 在 Dockerfile 里做 `mvn package` / `npm install`（由 CI 外部完成）
- ❌ **MUST NOT** 硬编码环境配置（用环境变量）
- ❌ **MUST NOT** 以 root 运行（生产环境，可用非 root 用户）
- ❌ **MUST NOT** 在镜像里留 Secrets / 密钥

### `.dockerignore` 模板

```
.git
.gitignore
.github/
node_modules/
dist/
target/
*.md
.idea/
.vscode/
.vs/
.DS_Store
*.log
coverage/
docs/
changes/
wiki/
```

## docker-compose.yml 编排规范

### 单服务模板（后端）

```yaml
version: "3.8"

services:
  user-service:
    image: registry.cn-hangzhou.aliyuncs.com/structured/user-service:1.2.0
    restart: always
    hostname: user-service
    container_name: user-service
    env_file:
      - .env
    deploy:
      restart_policy:
        condition: on-failure
      replicas: 1
    networks:
      - structure-cloud-work
    environment:
      - APP_PATH=/app/boot/app.jar
      - JAVA_OPTS=-Xms256m -Xmx1024m
      - PARAMS=-Dfile.encoding=UTF-8 -Dspring.profiles.active=pro -Djava.security.egd=file:/dev/./urandom -Duser.timezone=Asia/Shanghai
    healthcheck:
      test: ["CMD", "/bin/sh", "/app/liveness.sh"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    ports:
      - "8080:8080"

networks:
  structure-cloud-work:
    external: true
```

### 单服务模板（前端）

```yaml
  user-ui:
    image: registry.cn-hangzhou.aliyuncs.com/structured/user-ui:1.2.0
    restart: always
    hostname: user-ui
    container_name: user-ui
    env_file:
      - .env
    networks:
      - structure-cloud-work
    environment:
      - SCHEME=https
      - SERVER_HOST=api.prod.structured.cn
      - SERVER_PORT=443
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    ports:
      - "80:80"
```

### 编排规则

**MUST**：
- ✅ **MUST** 使用 `version: "3.8"`
- ✅ **MUST** 所有服务接入 `structure-cloud-work` 网络（`external: true`）
- ✅ **MUST** 后端用 `liveness.sh` 健康检查
- ✅ **MUST** 前端用 `wget --spider` 健康检查
- ✅ **MUST** 后端传 `JAVA_OPTS` / `PARAMS` / `APP_PATH` 三件套
- ✅ **MUST** 前端传 `SCHEME` / `SERVER_HOST` / `SERVER_PORT`
- ✅ **MUST** 使用 `env_file: .env` 管理环境变量
- ✅ **MUST** 配置 `restart: always` + `deploy.restart_policy`

**MUST NOT**：
- ❌ **MUST NOT** 在 docker-compose 里硬编码 Secrets
- ❌ **MUST NOT** 用 `latest` tag（生产环境）
- ❌ **MUST NOT** 暴露不必要的端口

## 镜像命名与 Tag 规范

### 命名

```
registry.cn-hangzhou.aliyuncs.com/structured/<service-name>:<version>
```

### Tag 策略

| Tag | 用途 | 示例 |
|---|---|---|
| `<version>` | 具体版本（生产） | `1.2.0` |
| `latest` | 最新（仅测试） | `latest` |
| `<version>-rc<N>` | 预发布 | `1.2.0-rc1` |
| `<version>-snapshot` | 快照 | `1.2.0-snapshot` |

**MUST**：
- ✅ 同时打 `<version>` 和 `latest` 两个 tag
- ✅ 生产环境 MUST 用具体版本号

## 关联

- 技能：`infra-ops` / `ci-pipeline`
- Wiki：`wiki/_common/ci-cd-pipeline.md` / `wiki/_common/kubernetes.md`
- 参考实现：`structure-iam/structure-user-center/structure-user/structure-user-boot/Dockerfile`
