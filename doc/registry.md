# somcli registry 使用文档

## 1. 功能概述

- **Harbor 仓库管理**：安装/卸载企业级镜像仓库
- **镜像同步**：在多仓库间批量同步容器镜像
- **代理支持**：通过 GitHub 代理加速资源下载

## 2. 命令结构

```bash
somcli registry [command] [flags]
```

## 3. 核心命令

### 3.1 Harbor 仓库管理

```bash
# 安装Harbor（自动配置TLS证书）
somcli registry install \
  --version v2.5.0 \
  --hostname harbor.example.com \
  --ca-path /etc/certs \
  --github-proxy "https://gh-proxy.com/"

# 卸载Harbor
somcli registry uninstall
```

### 3.2 镜像同步

```bash
# 同步镜像到私有仓库（支持并发控制）
somcli registry sync \
  --source docker.io \
  --target harbor.example.com/library \
  --image-list images.txt \
  --username admin \
  --password Harbor12345 \
  --concurrency 5
```

### 3.3 全局参数

| 参数             | 说明                                              |
| ---------------- | ------------------------------------------------- |
| `--github-proxy` | GitHub 资源代理地址（如 `https://gh-proxy.com/`） |
| `--config`       | 指定配置文件路径（默认 `~/.somcli.yaml`）         |

## 4. 配置文件示例

`~/.somcli.yaml`：

````yaml
github_proxy: "https://gh-proxy.com/"
registries:
  default:
    url: harbor.example.com
    username: admin
    password: Harbor12345

```;

## 5. 使用示例

### 5.1 典型工作流

```bash
# 1. 安装Harbor
somcli registry install -h harbor.example.com

# 2. 同步基础镜像
echo "nginx:latest
goharbor/nginx-photon:v2.5.0
registry.cn-beijing.aliyuncs.com/structured/structure-admin:1.0.2
" > images.txt
somcli registry sync -s docker.io -t harbor.example.com -f images.txt

# 3. 验证
curl -k https://harbor.example.com/api/v2.0/projects
````

### 5.2 高级用法

```bash
# 使用环境变量认证
export SOMCLI_REGISTRY_USERNAME=admin
export SOMCLI_REGISTRY_PASSWORD=Harbor12345
somcli registry sync -s docker.io -t harbor.example.com -f images.txt

# 仅同步特定架构镜像
grep "linux/amd64" image-manifest.txt > images.txt
somcli registry sync -f images.txt
```

## 6. 注意事项

1. **网络要求**：

   - 安装时需要访问 GitHub 下载 Harbor 安装包
   - 同步镜像时需要访问源和目标仓库

2. **权限要求**：

   - Harbor 安装需要 root 权限
   - 镜像同步需要 docker login 权限

3. **性能建议**：
   - 大规模同步时建议设置 `--concurrency`（默认 3）
   - 海外服务器可省略 `--github-proxy`

## 7. 常见问题

### Q1: 如何跳过证书验证？

```bash
# 在目标主机配置
mkdir -p /etc/docker/certs.d/harbor.example.com
cp ca.crt /etc/docker/certs.d/harbor.example.com/
```

### Q2: 同步中断后如何续传？

```bash
# 过滤已同步的镜像
grep -v -x -f synced.txt images.txt > remaining.txt
somcli registry sync -f remaining.txt
```

### Q3: 如何清理残留镜像？

```bash
# 清理所有临时镜像
docker images | awk '/<none>|harbor.example.com/{print $3}' | xargs docker rmi
```

---

该文档可通过 `somcli registry --help` 实时查看最新版本，建议配合实际环境测试验证。
