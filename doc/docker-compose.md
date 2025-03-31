# `somcli docker-compose` 使用文档

## 1. 功能概述

- **Compose 环境管理**：一键安装/卸载 Docker Compose
- **应用编排**：简化多容器应用生命周期管理
- **版本控制**：支持指定版本安装
- **代理支持**：通过 GitHub 代理加速安装

## 2. 命令结构

```bash
somcli docker-compose [command] [flags]
```

## 3. 核心命令

### 3.1 安装与卸载

```bash
# 安装最新稳定版
somcli docker-compose install

# 安装指定版本 (示例：v2.12.2)
somcli docker-compose install --version v2.12.2

# 使用代理安装（国内加速）
somcli docker-compose install --github-proxy "https://gh-proxy.com/"

# 卸载 Compose
somcli docker-compose uninstall
```

### 3.2 应用管理

```bash
# 启动所有服务（后台模式）
somcli docker-compose up -d

# 停止并移除所有容器
somcli docker-compose down

# 查看服务状态
somcli docker-compose ps

# 查看服务日志
somcli docker-compose logs -f
```

### 3.3 原生命令透传

```bash
# 透传任意 compose 命令（参数需加 -- 分隔）
somcli docker-compose -- build
somcli docker-compose -- config
```

## 4. 参数说明

| 参数             | 缩写 | 说明              |
| ---------------- | ---- | ----------------- |
| `--version`      | `-v` | 指定 Compose 版本 |
| `--github-proxy` | `-p` | GitHub 代理地址   |
| `--path`         | -    | 自定义安装路径    |
| `--`             | -    | 透传命令分隔符    |

## 5. 使用示例

### 5.1 典型工作流

```bash
# 1. 安装指定版本
somcli docker-compose install -v v2.12.2

# 2. 启动应用栈
somcli docker-compose -f docker-compose.prod.yml up -d

# 3. 监控服务状态
somcli docker-compose ps
```

### 5.2 高级用法

```bash
# 多文件组合部署
somcli docker-compose -f docker-compose.yml -f override.yml up

# 指定环境变量文件
somcli docker-compose --env-file .env.prod up

# 重建单个服务
somcli docker-compose up -d --no-deps --build web
```

## 6. 配置文件

`~/.somcli.yaml` 示例配置：

```yaml
docker_compose:
  default_version: "v2.12.2"
  install_path: "/usr/local/bin"
  github_proxy: "https://gh-proxy.com/"
```

## 7. 注意事项

1. **依赖要求**：

   - 需先安装 Docker（可通过 `somcli docker install` 完成）
   - Linux 系统需确保 `/usr/local/bin` 在 `PATH` 中

2. **版本兼容性**：

   - Compose V2 需要 Docker Engine 20.10+
   - 旧版项目可使用兼容模式：`somcli docker-compose --compatibility up`

3. **网络问题**：
   ```bash
   # 通过代理下载（企业内网环境）
   somcli docker-compose install -p "http://internal-proxy:3128"
   ```

## 8. 常见问题

### Q1: 安装后命令找不到？

```bash
# 手动刷新 PATH
export PATH=$PATH:/usr/local/bin
# 或指定安装路径
somcli docker-compose install --path ~/bin
```

### Q2: 如何迁移旧版 Compose 项目？

```bash
# 1. 转换旧版语法
somcli docker-compose -- convert

# 2. 使用兼容模式运行
somcli docker-compose --compatibility up
```

### Q3: 如何查看当前版本？

```bash
somcli docker-compose -- version
```

---

通过 `somcli docker-compose --help` 获取实时帮助。建议使用 `--dry-run` 测试危险操作（如 `down`）。
