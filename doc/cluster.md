# somcli 集群管理工具使用手册

## 1. 工具简介

somcli 是一个容器集群管理工具，支持 Docker Swarm 和 Kubernetes 两种主流容器编排系统的部署与管理。

## 2. 快速入门

### 2.1 安装工具

```bash
# 下载并安装 somcli
curl -L https://github.com/structure-projects/somcli/install.sh | bash
```

### 2.2 创建第一个集群

1. 准备配置文件 `my-cluster.yaml` (参考第 4 章配置模板)
2. 执行部署命令：

```bash
somcli cluster create -f my-cluster.yaml
```

## 3. 核心功能

### 3.1 集群生命周期管理

| 命令             | 功能描述   | 常用参数                                  |
| ---------------- | ---------- | ----------------------------------------- |
| `cluster deploy` | 部署新集群 | `-f` 指定配置文件<br>`--offline` 离线模式 |
| `cluster remove` | 销毁集群   | `-f` 指定配置文件<br>`--force` 强制删除   |

## 4. 配置参考

### 4.1 Swarm 集群配置模板

```yaml
cluster:
  type: "swarm"
  name: "my-swarm"
  nodes:
    - host: "swarm-mgr-01"
      ip: "192.168.1.100"
      role: "manager"
      sshKey: "~/.ssh/id_rsa"

  swarmConfig:
    advertiseAddr: "192.168.1.100" # 管理节点广播地址
    listenAddr: "0.0.0.0:2377" # 监听地址
    defaultAddrPool: # 地址池配置
      - "10.20.0.0/16"
    subnetSize: 24 # 子网大小
    dataPathPort: 4789 # 数据通道端口
```

### 4.2 Kubernetes 集群配置模板

```yaml
cluster:
  type: "k8s"
  name: "my-k8s"
  nodes:
    - host: "k8s-master"
      ip: "192.168.1.200"
      role: "master"
      sshKey: "~/.ssh/id_rsa"
  k8sConfig:
    version: "1.25.0" # Kubernetes版本
    podNetworkCidr: "10.244.0.0/16" # Pod网络CIDR
    serviceCidr: "10.96.0.0/12" # Service网络CIDR
```

## 5. 最佳实践

### 5.1 生产环境建议

1. 为 manager/master 节点配置高可用（至少 3 节点）
2. 使用离线模式部署保障稳定性 (`--offline`)
3. 提前进行环境预检查 (`precheck`)

### 5.2 配置技巧

```yaml
# 使用SSH别名简化配置
nodes:
  - host: "mgr01" # 对应 ~/.ssh/config 中的主机别名
    role: "manager"

# 复用SSH密钥
sshKey: "/shared/ssh/cluster-key" # 所有节点共用密钥
```

## 6. 常见问题处理

### 6.1 部署失败排查步骤

1. 检查节点 SSH 连通性

### 6.2 典型错误解决方案

| 错误现象                | 解决方案                      |
| ----------------------- | ----------------------------- |
| "SSH connection failed" | 检查 sshKey 路径和权限        |
| "Port already in use"   | 修改 swarmConfig 中的端口配置 |
| "Insufficient memory"   | 增加节点资源或调整配置        |

## 附录

### A. 版本兼容性

| somcli 版本 | Docker 版本 | Kubernetes 版本 |
| ----------- | ----------- | --------------- |
| v1.0+       | 20.10+      | 1.23-1.25       |

### B. 获取帮助

```bash
# 查看完整帮助
somcli --help

# 获取特定命令帮助
somcli cluster --help
```
