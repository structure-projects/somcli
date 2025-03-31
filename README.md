# somcli 容器管理工具 - 完整文档

## 目录

- [项目概述](#项目概述)
- [功能特性](#功能特性)
- [快速开始](#快速开始)
- [核心模块](#核心模块)
  - [Docker 管理](#docker-管理)
  - [Docker Compose 管理](#docker-compose-管理)
  - [镜像管理](#镜像管理)
  - [Registry 管理](#registry-管理)
  - [集群管理](#集群管理)
  - [离线管理](#离线管理)
  - [Swarm 管理](#swarm-管理)
  - [Kubernetes 管理](#kubernetes-管理)
- [配置参考](#配置参考)
- [开发指南](#开发指南)
- [常见问题](#常见问题)
- [设计架构](#设计架构)

## 项目概述

somcli (structure-ops-cli) 是一个统一的容器管理工具，提供从基础设施到应用部署的全生命周期管理。它整合了 Docker、Docker Compose、Harbor、Swarm 和 Kubernetes 等主流容器技术，通过一致的命令行界面简化运维工作。

## 功能特性

- **全栈支持**：统一管理 Docker、Compose、Swarm 和 Kubernetes
- **一键部署**：自动化安装和配置容器环境
- **镜像全生命周期**：拉取、推送、导出、导入一站式操作
- **企业级仓库**：内置 Harbor 仓库管理
- **离线支持**：完整离线部署解决方案
- **代理加速**：内置 GitHub 代理支持
- **灵活配置**：支持配置文件和变量

## 快速开始

### 安装 somcli

```bash
# 二进制安装
curl -L https://github.com/structure-projects/somcli/releases/latest/download/somcli-$(uname -s)-$(uname -m) -o /usr/local/bin/somcli
chmod +x /usr/local/bin/somcli

# 验证安装
somcli version
```

### 基础工作流

```bash
# 1. 安装 Docker 环境
somcli docker install -v 20.10.12

# 2. 部署 Harbor 仓库
somcli registry install -h harbor.example.com

# 3. 部署 Kubernetes 集群
somcli cluster create -f cluster.yaml
```

## 核心模块

### Docker 管理

[完整文档](./doc/docker.md)

```bash
# 安装指定版本
somcli docker install -v 24.0.6

# 容器管理
somcli docker ps -a
somcli docker logs [容器ID]

# 镜像操作
somcli docker pull nginx:latest
somcli docker rmi nginx:latest
```

### Docker Compose 管理

[完整文档](./doc/docker-compose.md)

```bash
# 安装最新版
somcli docker-compose install

# 应用管理
somcli docker-compose -f stack.yml up -d
somcli docker-compose logs -f
```

### 镜像管理

[完整文档](./doc/images.md)

```bash
# 批量操作
somcli docker-images pull -s k8s
somcli docker-images export -o images.tar.gz

# 仓库同步
somcli docker-images push -r harbor.example.com
```

### Registry 管理

[完整文档](./doc/registry.md)

```bash
# Harbor 安装
somcli registry install -v v2.5.0 -h harbor.example.com

# 镜像同步
somcli registry sync -s docker.io -t harbor.example.com
```

### 集群管理

[完整文档](./doc/cluster.md)

```yaml
# cluster.yaml 示例
cluster:
  type: "k8s"
  nodes:
    - host: "master1"
      ip: "192.168.1.100"
      role: "master"
```

```bash
# 集群操作
somcli cluster create -f cluster.yaml
somcli get nodes
```

### 离线管理

[完整文档](./doc/offline.md)

```yaml
# 离线配置示例
download:
  resources:
    - name: "docker"
      version: "20.10.12"
      urls:
        [
          "https://download.docker.com/linux/static/stable/x86_64/docker-20.10.12.tgz"
        ]
```

```bash
# 离线包操作
somcli offline download -f offline.yaml
somcli offline install -p ./doc/offline-packages
```

### Swarm 管理

```bash
# Swarm 集群初始化
somcli swarm init --advertise-addr 192.168.1.100

# 节点管理
somcli swarm join --token [TOKEN] 192.168.1.100:2377
```

### Kubernetes 管理

```bash
# K8s 集群操作
somcli k8s install -f cluster.yaml
somcli k8s get pods -A
```

## 配置参考

### 全局配置 (~/.somcli.yaml)

```yaml
github_proxy: "https://gh-proxy.com/"
docker:
  default_version: "20.10.12"
registries:
  main:
    url: "harbor.example.com"
    username: "admin"
```

### 集群配置

```yaml
# Swarm 配置
swarmConfig:
  advertiseAddr: "192.168.1.200"
  listenAddr: "0.0.0.0:2377"
  defaultAddrPool:
    - "10.20.0.0/16"
  subnetSize: 24

# K8s 配置
k8sConfig:
  version: "1.25.0"
  podNetworkCidr: "10.244.0.0/16"
  serviceCidr: "10.96.0.0/12"
```

## 开发指南

### 项目结构

```
somcli/
├── cmd/          # CLI 入口
├── pkg/          # 功能实现
│   ├── cluster/  # 集群逻辑
│   ├── docker/   # Docker 封装
│   └── ...       # 其他模块
├── internal/     # 内部定义
├── configs/      # 示例配置
└── main.go       # 程序入口
```

### 添加新命令

1. 在 `cmd/` 下创建命令文件
2. 实现 Cobra 命令结构
3. 注册到根命令

## 常见问题

### 安装问题

```bash
# 静默安装
somcli docker install --force

# 彻底卸载
somcli docker uninstall --force
```

## 设计架构

### 模块关系图

```
+----------------+
|    CLI 入口    |
+----------------+
        |
        v
+----------------+    +----------------+
|  命令解析层     |--->|  功能实现层     |
+----------------+    +----------------+
        |                     |
        v                     v
+----------------+    +----------------+
| 配置管理系统    |    | 第三方工具集成  |
+----------------+    +----------------+
```

### 核心设计原则

1. **一致性**：统一的操作体验
2. **可扩展**：模块化设计
3. **灵活性**：支持多种配置方式
4. **可靠性**：完善的错误处理

---

通过 `somcli --help` 获取完整帮助，各子模块帮助可通过 `somcli [module] --help` 查看。
