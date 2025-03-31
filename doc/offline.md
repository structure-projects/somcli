## 离线下载

### 离线配置参考

```yaml

```

### **somcli 离线下载功能文档**

---

## **1. 功能概述**

`somcli offline download` 提供离线资源下载能力，支持：

- **多资源批量下载**（Docker、Kubernetes、Harbor 等）
- **代理支持**（HTTP/HTTPS 代理）
- **离线模式**（仅检查文件是否存在，不实际下载）
- **校验和验证**（SHA256/MD5）
- **路径模板化**（支持动态生成目标路径）

---

## **2. 使用方式**

### **2.1 基本命令**

```bash
# 下载配置文件指定的所有资源
somcli offline download -f <config-file>

# 静默模式（不显示下载详情）
somcli offline download -f <config-file> -q

# 离线模式（仅检查文件是否已存在）
export SOMCLI_OFFLINE=true
somcli offline download -f <config-file>
```

### **2.2 配置文件示例**

```yaml
# config.yaml
download:
    resources:
    - name: "docker"
        version: "20.10.12"
        urls:
        - "https://download.docker.com/linux/static/stable/x86_64/docker-20.10.12.tgz"
        target: "{{.Name}}-{{.Version}}.tgz"
        checksum: "sha256:abc123..."

    - name: "kubeadm"
        version: "1.25.0"
        urls:
        - "https://storage.googleapis.com/kubernetes-release/release/v1.25.0/bin/linux/amd64/kubeadm"
        target: "{{.Filename}}"
```

#### **配置字段说明**

| 字段                   | 说明                                                 |
| ---------------------- | ---------------------------------------------------- |
| `cache_dir`            | 自定义缓存目录（默认 `/var/cache/somcli/downloads`） |
| `resources[].name`     | 资源名称（仅用于日志）                               |
| `resources[].version`  | 资源版本                                             |
| `resources[].urls`     | 下载地址（支持多个镜像源）                           |
| `resources[].target`   | 目标路径（支持模板变量）                             |
| `resources[].checksum` | 文件校验和（可选）                                   |

#### **模板变量**

| 变量            | 说明                                |
| --------------- | ----------------------------------- |
| `{{.Name}}`     | 资源名称（如 `docker`）             |
| `{{.Version}}`  | 资源版本（如 `20.10.12`）           |
| `{{.Filename}}` | 从 URL 提取的文件名（如 `kubeadm`） |

---

## **3. 输出示例**

### **3.1 成功下载**

```bash
$ somcli offline download -f config.yaml

Download results:
  ✓ docker-20.10.12: docker/docker-20.10.12.tgz
  ✓ kubeadm-1.25.0: kubernetes/1.25.0/kubeadm

Summary: 2/2 succeeded
```

### **3.2 部分失败**

```bash
$ somcli offline download -f config.yaml

Download results:
  ✓ docker-20.10.12: docker/docker-20.10.12.tgz
  ✗ kubeadm-1.25.0: connection refused

Summary: 1/2 succeeded
```

### **3.3 离线模式**

```bash
$ export SOMCLI_OFFLINE=true
$ somcli offline download -f config.yaml

Download results:
  ✓ docker-20.10.12: docker/docker-20.10.12.tgz (cached)
  ✗ kubeadm-1.25.0: file not found in offline mode

Summary: 1/2 succeeded
```

---

## **4. 高级功能**

### **4.1 校验和验证**

如果配置了 `checksum`，下载完成后会自动验证文件完整性：

```yaml
resources:
  - name: "docker"
    urls: ["https://.../docker.tgz"]
    target: "docker.tgz"
    checksum: "sha256:abc123..." # 格式: <算法>:<哈希值>
```

### **4.2 代理支持**

- **全局代理**：在配置文件中设置 `proxy`
- **环境变量代理**：支持 `HTTP_PROXY`/`HTTPS_PROXY`
- **GitHub 镜像代理**：自动替换 `github.com` 为代理地址（如 `https://ghproxy.com`）

---

## **5. 其他模块调用**

### **5.1 获取已下载文件**

```go
import "github.com/structure-projects/somcli/pkg/offline"

func main() {
    // 从缓存目录获取文件绝对路径
    filePath, err := offline.GetCachedFilePath("docker/docker-20.10.12.tgz", "")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Docker package path:", filePath)
}
```

### **5.2 自定义缓存目录**

```go
// 通过环境变量覆盖默认目录
os.Setenv("SOMCLI_DOWNLOAD_CACHE_DIR", "/tmp/custom-cache")
```

---

## **6. 注意事项**

1. **离线模式**：需提前下载所有资源到缓存目录。
2. **权限问题**：确保对缓存目录有读写权限。
3. **代理问题**：如果使用代理，确保代理服务器可访问目标 URL。
4. **校验和失败**：会自动删除无效文件。

---

## **7. 示例配置文件**

完整示例见 [config.example.yaml](./config.example.yaml)。

---

**总结**：`somcli offline download` 提供了一种统一的方式管理离线资源，适用于无外网环境的集群部署。
