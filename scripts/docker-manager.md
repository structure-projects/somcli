# Docker 管理脚本文档

一个用于管理 Docker 安装、配置和卸载的 Bash 脚本。

## 功能特点

- 安装指定版本的 Docker
- 配置 Docker 镜像加速器
- 设置 systemd 服务
- 检查 Docker 环境状态
- 完全卸载 Docker
- 支持直接透传命令给 Docker

## 使用说明

```bash
./docker-manager.sh [选项] [docker命令]
```

### 选项参数

| 选项                 | 描述                                      |
| -------------------- | ----------------------------------------- |
| `-v, --version 版本` | 指定安装的 Docker 版本 (默认: 24.0.6)     |
| `-p, --path 路径`    | 指定安装路径 (默认: ./work/docker/版本号) |
| `-d, --download URL` | 指定下载地址                              |
| `-c, --check`        | 检查 Docker 环境                          |
| `-u, --uninstall`    | 完全卸载 Docker                           |
| `-h, --help`         | 显示帮助信息                              |

### 使用示例

```bash
# 直接安装 Docker (使用默认配置)
./docker-manager.sh

# 检查 Docker 环境
./docker-manager.sh -c

# 卸载 Docker
./docker-manager.sh -u

# 安装指定版本
./docker-manager.sh -v 24.0.6

# 透传命令给 Docker (相当于 docker ps -a)
./docker-manager.sh ps -a
```

## 功能详解

### 1. 环境检查功能

- 检测 Docker 是否已安装
- 显示 Docker 版本和运行状态
- 检查 docker-compose 是否安装
- 验证当前用户是否在 docker 用户组
- 检查镜像加速配置

### 2. 安装功能

- 自动下载指定版本的 Docker 二进制包
- 支持断点续传和下载进度显示
- 自动解压并安装到系统目录
- 配置 systemd 服务
- 设置多个国内镜像加速源
- 自动启动 Docker 服务

### 3. 卸载功能

- 完全移除 Docker 相关文件
- 停止并禁用 Docker 服务
- 清理配置和数据目录
- 验证卸载是否彻底

### 4. 镜像加速配置

内置以下国内镜像加速源:

- https://proxy.1panel.live
- https://docker.1panel.top
- https://docker.m.daocloud.io
- https://docker.1ms.run
- https://docker.ketches.cn

## 错误处理

脚本提供详细的错误提示和可能原因分析，包括:

- 下载失败 (网络问题/URL 不可用)
- 安装失败 (权限不足/磁盘空间)
- 服务配置错误
- JSON 格式验证失败
- 服务启动失败

## 安装后提示

成功安装后会显示:

- Docker 版本信息
- 服务运行状态
- 镜像加速配置
- 用户组设置提示 (如需)

## 注意事项

1. 需要 root 权限执行安装/卸载操作
2. 建议安装 jq 工具以获得更好的配置验证体验
3. 卸载后会彻底删除所有 Docker 相关文件
4. 如果已安装 Docker，需要先卸载才能重新安装

## 版本信息

当前脚本版本: v2.2  
默认 Docker 版本: 24.0.6
