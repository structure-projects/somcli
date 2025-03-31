# somcli docker-images 使用文档

## 1. 功能概述

- **多仓库支持**：跨 Docker Hub/Harbor/私有仓库操作
- **批量处理**：基于作用域或文件列表批量操作镜像
- **离线迁移**：支持镜像打包为压缩文件
- **代理加速**：通过 GitHub 代理加速下载

## 2. 命令结构

```bash
somcli docker-images [command] [flags]
```

## 3. 核心命令

### 3.1 镜像拉取

```bash
# 从默认仓库拉取所有镜像
somcli docker-images pull

# 从Harbor拉取指定镜像
somcli docker-images pull \
  --scope harbor \
  --registry harbor.example.com

# 从自定义文件拉取
somcli docker-images pull \
  --file image-list.txt
```

### image-list.txt 参考

```txt
nginx:latest
goharbor/nginx-photon:v2.5.0
registry.cn-beijing.aliyuncs.com/structured/structure-admin:1.0.2

```

### 3.2 镜像推送

```bash
# 推送所有镜像到私有仓库
somcli docker-images push \
  --registry harbor.example.com/project

# 带认证的推送
somcli docker-images push \
  --username admin \
  --password Harbor12345
```

### 3.3 镜像导出

```bash
# 导出全部镜像到压缩包
somcli docker-images export \
  --output /backup/images.tar.gz

# 导出指定作用域镜像
somcli docker-images export \
  --scope k8s \
  --output k8s-images.tar
```

### 3.4 镜像导入

```bash
# 从压缩包导入镜像
somcli docker-images import \
  --input /backup/images.tar.gz
```

## 4. 参数说明

### 通用参数

| 参数         | 缩写 | 默认值 | 说明                     |
| ------------ | ---- | ------ | ------------------------ |
| `--scope`    | `-s` | `all`  | 作用域（all/harbor/k8s） |
| `--registry` | `-r` | -      | 自定义仓库地址           |

### 命令专属参数

| 命令     | 参数                    | 说明                 |
| -------- | ----------------------- | -------------------- |
| `pull`   | `--file`                | 自定义镜像列表文件   |
| `export` | `--output`              | 导出文件路径（必需） |
| `import` | `--input`               | 导入文件路径（必需） |
| `push`   | `--username/--password` | 仓库认证信息         |

## 5. 使用示例

### 5.1 典型工作流

```bash
# 1. 从生产Harbor拉取镜像
somcli docker-images pull \
  -s harbor \
  -r harbor.prod.example.com

# 2. 导出为离线包
somcli docker-images export \
  -o /backup/prod-images-$(date +%Y%m%d).tar.gz

# 3. 推送到测试环境
somcli docker-images push \
  -r harbor.test.example.com/dev \
  -u tester -p Test@123
```

### 5.2 高级用法

```bash
# 使用代理加速拉取
somcli --github-proxy "https://gh-proxy.com/" \
  docker-images pull

# 同步特定架构镜像
grep "linux/arm64" images.txt > arm64-images.txt
somcli docker-images pull -f arm64-images.txt

# 批量重打标签
while read img; do
  docker tag $img new-registry.example.com/${img#*/}
done < images.txt
```

## 6. 配置文件

`~/.somcli.yaml` 示例：

```yaml
docker_images:
  default_registry: "harbor.example.com"
  scopes:
    harbor:
      - nginx:latest
      - redis:alpine
    k8s:
      - pause:3.7
      - coredns:1.8.0
```

## 7. 注意事项

1. **认证安全**：

   - 密码建议通过环境变量传递 `export SOMCLI_REGISTRY_PASSWORD=xxx`
   - 避免在命令行直接暴露密码

2. **存储空间**：

   - 导出大镜像需确保磁盘空间充足
   - 工作目录建议使用高速存储

3. **网络要求**：
   - 推送/拉取需开放仓库端口（通常 443 或 5000）
   - 离线环境只需 import/export 功能

## 8. 常见问题

### Q1: 如何查看支持的镜像列表？

```bash
# 查看配置文件定义的镜像
cat ~/.somcli.yaml | grep -A10 "docker_images:"

# 列出已下载的镜像
docker images
```

### Q2: 导出文件太大如何分割？

```bash
# 分割为2GB每份
split -b 2G images.tar.gz images-part-

# 合并还原
cat images-part-* > images.tar.gz
```

### Q3: 如何清理残留镜像？

```bash
# 清理所有临时镜像
docker images | grep "^<none>" | awk '{print $3}' | xargs docker rmi
```

---

通过 `somcli docker-images --help` 可查看实时帮助信息，建议结合 `--dry-run` 参数测试命令效果后再实际执行。
