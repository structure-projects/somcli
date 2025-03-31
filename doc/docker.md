# `somcli docker` 使用文档

## 1. 功能概述

- **Docker 环境管理**：一键安装/卸载 Docker 环境
- **容器操作**：简化容器生命周期管理
- **兼容性**：支持 Linux 和 macOS 系统
- **版本控制**：支持指定版本安装

## 2. 命令结构

```bash
somcli docker [command] [flags]
```

## 3. 核心命令

### 3.1 安装与卸载

```bash
# 安装最新版 Docker
somcli docker install

# 安装指定版本 (示例：20.10.12)
somcli docker install --version 20.10.12

# 静默安装（跳过确认提示）
somcli docker install --yes

# 卸载 Docker
somcli docker uninstall

# 强制卸载（不提示确认）
somcli docker uninstall --force
```

### 3.2 容器管理

```bash
# 查看运行中的容器
somcli docker ps

# 查看所有容器（包括已停止）
somcli docker ps --all

# 查看容器日志
somcli docker logs [容器ID]

# 启动/停止容器
somcli docker start/stop [容器ID]
```

### 3.3 镜像管理

```bash
# 列出本地镜像
somcli docker images

# 拉取镜像
somcli docker pull nginx:latest

# 删除镜像
somcli docker rmi nginx:latest
```

### 3.4 原生命令透传

```bash
# 透传任意 docker 命令（参数需加 -- 分隔）
somcli docker -- ps -a
somcli docker -- logs -f my_container
```

## 4. 参数说明

| 参数        | 缩写 | 说明             |
| ----------- | ---- | ---------------- |
| `--version` | `-v` | 指定 Docker 版本 |
| `--yes`     | `-y` | 跳过确认提示     |
| `--force`   | `-f` | 强制操作不提示   |
| `--`        | -    | 透传命令分隔符   |

## 5. 使用示例

### 5.1 典型工作流

```bash
# 1. 安装 Docker
somcli docker install -v 20.10.12

# 2. 运行测试容器
somcli docker -- run -d -p 80:80 --name webserver nginx

# 3. 查看运行状态
somcli docker ps

# 4. 检查日志
somcli docker logs webserver
```

### 5.2 高级用法

```bash
# 批量清理无用镜像
somcli docker -- image prune -a

# 使用代理安装（企业内网环境）
export ALL_PROXY=http://proxy.example.com:8080
somcli docker install

# 查看 Docker 系统信息
somcli docker -- info
```

## 6. 配置文件

`~/.somcli.yaml` 可配置默认参数：

```yaml
docker:
  default_version: "20.10.12"
  proxy: "http://internal-proxy:3128"
```

## 7. 注意事项

1. **权限要求**：

   - 安装/卸载需要 root 权限
   - 日常操作建议将用户加入 `docker` 用户组

2. **版本兼容性**：

   - Linux 推荐使用 Docker CE 20.10+
   - macOS 推荐通过 Docker Desktop 管理

3. **网络问题**：
   - 安装失败时可尝试 `--github-proxy` 参数
   ```bash
   somcli --github-proxy "https://gh-proxy.com/" docker install
   ```

## 8. 常见问题

### Q1: 如何查看已安装版本？

```bash
somcli docker -- version
# 或
docker version
```

### Q2: 如何彻底卸载 Docker？

```bash
somcli docker uninstall --force
# 补充清理残留文件（Linux 示例）：
sudo rm -rf /var/lib/docker
```

---

通过 `somcli docker --help` 可查看实时帮助信息。建议配合 `--dry-run` 参数测试危险操作（如卸载）。
