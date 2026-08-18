---
name: infra-ops
description: |
  当用户要求"写 Dockerfile/容器化"、"写 docker-compose/编排服务"、"docker 命令/构建镜像/看容器日志"、
  "写 K8s 部署/manifest/Deployment/Service/Ingress"、"kubectl 操作/查看 pod/pod 日志"、
  "helm 安装/升级/回滚/chart"、"验证 K8s 部署/检查部署状态"或"Terraform/IaC/云资源编排"时触发。
  基础设施全流程：容器化（Dockerfile + compose）→ K8s 部署与运维（manifest / Helm / kubectl）→ Terraform 云资源。
  Dockerfile MUST 含健康检查、时区、三件套；生产写操作、delete、prune、rollback、context 切换、
  terraform apply/destroy MUST 用户确认（L0）。

triggers:
  - 写 Dockerfile
  - Docker 化
  - 容器化
  - dockerize
  - 容器化部署
  - 写 docker-compose
  - docker compose
  - 编排服务
  - 多服务部署
  - 本地编排
  - docker
  - docker 命令
  - docker 构建
  - docker 运行
  - docker 日志
  - docker push
  - docker pull
  - 容器操作
  - K8s
  - k8s 部署
  - 编写 manifest
  - Deployment
  - Service
  - Ingress
  - K8s YAML
  - kubectl
  - kubectl apply
  - kubectl get
  - kubectl logs
  - 查看 pod
  - pod 日志
  - k8s 操作
  - helm
  - helm chart
  - helm install
  - helm upgrade
  - helm rollback
  - 验证 K8s
  - K8s 状态
  - 检查部署
  - 验证部署
  - Terraform
  - IaC
  - 基础设施即代码
  - 云资源编排
  - terraform
  - 基础设施

role: devops
phase: deployment

allowed-tools: Bash, Read, Write, Edit

related-rules:
  - common-core
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/docker.md
  - wiki/_common/kubernetes.md
  - wiki/_common/ci-cd-pipeline.md

produces:
  - Dockerfile + .dockerignore + liveness.sh（后端）/ nginx.template（前端）
  - docker-compose.yml + .env.example
  - K8s manifest（Deployment / Service / Ingress / ConfigMap / Secret）或 Helm Chart
  - Terraform 配置（*.tf）+ 状态后端 + 变量与输出定义
  - 完成的 docker / kubectl / helm / terraform 操作与集群状态输出
  - 部署验证报告（changes/proposals/<current>/deployment.md）

requires: []

human-in-the-loop:
  - id: confirm-service-type
    action: 服务类型（后端 Spring Boot / 前端静态站点）与基础镜像版本 MUST 询问用户
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 类型判断错误只需重写 Dockerfile，无副作用
  - id: confirm-compose-topology
    action: 服务清单 / 端口映射 / 环境变量 MUST 用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 编排错误在本地 up 阶段即暴露，改文件重启即可
  - id: select-deploy-mode
    action: 部署方式（原生 YAML / Helm Chart）与目标 namespace MUST 询问用户
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 方式判断错误只影响未 apply 的文件，可重写
  - id: confirm-production-write
    action: 对生产环境执行写操作（docker run / apply / upgrade / scale / restart）MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-delete
    action: 执行 kubectl delete / helm uninstall / drain MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-prune
    action: 执行 docker system prune / image prune / volume 清理 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-rollback
    action: 执行 kubectl rollout undo / helm rollback MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-context-switch
    action: 切换 kube context 或默认 namespace MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-terraform-write
    action: 执行 terraform apply / destroy 或变更状态后端配置 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A

on-failure: |
  镜像构建失败 → 分析构建日志，修复后重试
  健康检查失败 → 检查 liveness.sh（后端）或 wget --spider（前端）
  服务启动失败 → docker logs / docker inspect 排查，MUST NOT 直接删容器重来
  kubectl / helm 命令失败 → 读 Events / logs 定位，修复后重试；MUST NOT 反复 delete 重建掩盖问题
  Chart 渲染失败 → 用 helm template / helm lint 调试
  terraform apply 失败 → terraform plan 分析差异；状态锁冲突 → 协调后重试
  权限不足 → 引导用户配置 docker 权限 / RBAC，MUST NOT 用 sudo 或提权绕过
  验证任一项失败 → 判定部署失败，MUST NOT 只凭 Pod Running 就报成功

category: deployment
stack: _common
priority: high
---

# 基础设施运维

> 容器化（Dockerfile + compose）→ K8s 部署与运维（manifest / Helm / kubectl）→ Terraform 云资源。
> **生产写操作、delete、prune、rollback、context 切换、terraform apply/destroy 均为 L0，MUST 用户确认**。

按任务选章：容器化 → 一；K8s → 二；云资源 → 三。K8s 前置：Dockerfile 已存在（见第一章）。

---

# 一、容器化（Dockerfile + compose）

## 1.1 Dockerfile 编写

### 第 1 步：确定服务类型（MUST 询问用户）

```
Q1: 服务类型？
    a) 后端 Spring Boot
    b) 前端（Vue / React / 静态站点）

Q2: JDK 版本（后端）？
    默认：21（structure-projects 当前主线）
    备选：17

Q3: 端口？
    默认：后端 8080，前端 80
```

### 第 2 步：按模板生成

按 `wiki/_common/docker.md` 生成：

| 服务类型 | 模板 | 产出 |
|---|---|---|
| 后端 Spring Boot | 模板 A | `Dockerfile` + `liveness.sh` |
| 前端 Nginx | 模板 B | `Dockerfile` + `nginx.template` |

### 第 3 步：生成 .dockerignore

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
.DS_Store
*.log
coverage/
docs/
changes/
wiki/
```

### 关键约束

- ✅ **MUST** 使用 `alpine` 变体
- ✅ **MUST** 设置时区 `TZ=Asia/Shanghai`
- ✅ **MUST** 配置 `HEALTHCHECK`
- ✅ **MUST** 用 `ENTRYPOINT` 而非 `CMD`
- ✅ **MUST** 清理包管理器缓存
- ❌ **MUST NOT** 在 Dockerfile 里做 `mvn package` / `npm install`
- ❌ **MUST NOT** 硬编码环境配置

## 1.2 docker-compose 编排

前置：各服务 Dockerfile 已存在。

### 第 1 步：确定服务清单（MUST 询问用户）

服务名 + 类型（后端 / 前端 / 中间件）+ 端口映射。

### 第 2 步：生成 docker-compose.yml

按 `wiki/_common/docker.md` 模板生成，骨架：

```yaml
version: "3.8"

services:
  user-service:
    image: registry.cn-hangzhou.aliyuncs.com/structured/user-service:1.2.0
    restart: always
    hostname: user-service
    container_name: user-service
    env_file: [.env]
    deploy:
      restart_policy: { condition: on-failure }
      replicas: 1
    networks: [structure-cloud-work]
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

  user-ui:
    image: registry.cn-hangzhou.aliyuncs.com/structured/user-ui:1.2.0
    restart: always
    hostname: user-ui
    container_name: user-ui
    env_file: [.env]
    networks: [structure-cloud-work]
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

networks:
  structure-cloud-work:
    external: true
```

### 第 3 步：生成 .env.example

```bash
TZ=Asia/Shanghai

MYSQL_HOST=mysql
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=<填入>

REDIS_HOST=redis
REDIS_PORT=6379

NACOS_ADDR=nacos:8848
```

### 关键约束

- ✅ **MUST** 使用 `version: "3.8"`
- ✅ **MUST** 所有服务接入 `structure-cloud-work`（`external: true`）
- ✅ **MUST** 后端用 `liveness.sh`、前端用 `wget --spider` 健康检查
- ✅ **MUST** 后端传 `JAVA_OPTS` / `PARAMS` / `APP_PATH` 三件套
- ✅ **MUST** 前端传 `SCHEME` / `SERVER_HOST` / `SERVER_PORT`
- ✅ **MUST** 使用 `env_file: .env`
- ❌ **MUST NOT** 硬编码 Secrets
- ❌ **MUST NOT** 用 `latest` tag（生产环境）

## 1.3 docker CLI 运维

### 镜像操作

```bash
# 构建（MUST 同时打 version + latest 双 tag）
docker build -t registry.cn-hangzhou.aliyuncs.com/structured/<service>:<version> \
             -t registry.cn-hangzhou.aliyuncs.com/structured/<service>:latest .

docker images
docker push <image>:<version>
docker pull <image>:<version>
docker rmi <image>:<version>
docker inspect <image>:<version>

# 导出 / 导入
docker save -o image.tar <image>:<version>
docker load -i image.tar
```

### 容器操作

```bash
docker run -d -p 8080:8080 \
  -e JAVA_OPTS="-Xms256m -Xmx1024m" \
  -e PARAMS="-Dspring.profiles.active=pro" \
  --name <name> <image>:<version>

docker ps                      # 运行中
docker ps -a                   # 含已停止
docker logs -f <name>          # 跟随日志
docker logs --tail 100 <name>
docker exec -it <name> /bin/sh
docker stop/start/restart <name>
docker rm <name>
```

### 清理操作（L0，MUST 用户确认）

```bash
docker container prune
docker image prune
docker system prune
docker system prune -a --volumes   # 破坏性最强
```

### 调试与网络

```bash
docker inspect <name>
docker stats
docker top <name>
docker cp <name>:/path/to/file ./local-path

docker network ls
docker network inspect structure-cloud-work
docker network create structure-cloud-work   # 生态约定的外部网络
docker volume ls
```

### 常见问题

| 现象 | 排查 |
|---|---|
| 容器启动失败 | `docker logs` → `docker inspect` → `docker exec -it <name> sh` |
| 镜像太大 | `docker images --format "{{.Repository}}:{{.Tag}} {{.Size}}"`；改 alpine + 多阶段构建 |
| 网络不通 | `docker network inspect structure-cloud-work`，确认服务在同一网络 |

### 关键约束

- ✅ **MUST** 构建时打 `<version>` + `latest` 双 tag
- ❌ **MUST NOT** 在生产环境用 `latest` tag 运行
- ❌ **MUST NOT** 未确认执行 `docker system prune -a --volumes`

---

# 二、K8s 部署与运维

前置：Dockerfile 已存在（第一章）。MUST Read：`wiki/_common/kubernetes.md`。

## 2.1 部署文件编写

### 第 1 步：确定部署方式（MUST 询问用户）

```
Q1: 部署方式？
    a) 原生 K8s YAML（简单场景）
    b) Helm Chart（推荐，生态标准）

Q2: 目标环境 / namespace？
    test / staging / prod
```

### 第 2 步：按方式生成

#### 方式 A：原生 K8s YAML

```
k8s/
├── namespace.yaml
├── deployment-backend.yaml
├── service-backend.yaml
├── deployment-frontend.yaml
├── service-frontend.yaml
├── ingress.yaml
├── configmap.yaml
└── secret.yaml
```

#### 方式 B：Helm Chart（推荐，双 workload 模板）

```
helm/<chart-name>/
├── Chart.yaml
├── values.yaml
├── .helmignore
└── templates/
    ├── _helpers.tpl
    ├── deployment.yaml      # 双 workload（backend + frontend）
    ├── service.yaml
    ├── ingress.yaml
    ├── hpa.yaml
    ├── serviceaccount.yaml
    ├── NOTES.txt
    └── tests/test-connection.yaml
```

### 第 3 步：关键配置

#### 后端容器要点

```yaml
containers:
- name: user-service
  image: registry.cn-hangzhou.aliyuncs.com/structured/user-service:1.2.0
  env:
  - name: APP_PATH
    value: /app/boot/app.jar
  - name: JAVA_OPTS
    value: -Xms256m -Xmx1024m
  - name: PARAMS
    value: -Dspring.profiles.active=pro
  ports:
  - containerPort: 8080
  livenessProbe:
    httpGet:
      path: /actuator/health
      port: 7777        # 生态约定的 actuator 端口
    initialDelaySeconds: 60
    periodSeconds: 30
  readinessProbe:
    httpGet:
      path: /actuator/health
      port: 7777
    initialDelaySeconds: 30
    periodSeconds: 10
  resources:
    requests: { memory: "256Mi", cpu: "250m" }
    limits:   { memory: "1Gi",   cpu: "1000m" }
```

#### 前端容器要点

```yaml
containers:
- name: user-ui
  image: registry.cn-hangzhou.aliyuncs.com/structured/user-ui:1.2.0
  env:
  - name: SCHEME
    value: https
  - name: SERVER_HOST
    value: api.prod.structured.cn
  - name: SERVER_PORT
    value: "443"
  ports:
  - containerPort: 80
```

#### values.yaml 关键约定（Helm）

```yaml
backend:
  enabled: true
  name: user-service
  image:
    repository: registry.cn-hangzhou.aliyuncs.com/structured/user-service
    tag: "1.2.0"
    pullPolicy: Always
  service: { type: ClusterIP, port: 8080 }
  env:
    APP_PATH: /app/boot/app.jar
    JAVA_OPTS: -Xms256m -Xmx1024m
    PARAMS: -Dspring.profiles.active=pro
  replicaCount: 1

frontend:
  enabled: true
  name: user-ui
  image:
    repository: registry.cn-hangzhou.aliyuncs.com/structured/user-ui
    tag: "1.2.0"
  service: { port: 80 }
  env:
    SCHEME: https
    SERVER_HOST: api.prod.structured.cn
    SERVER_PORT: "443"

autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 3
  targetCPUUtilizationPercentage: 80

ingress:
  enabled: false
  className: ""
  hosts: []
```

双 workload 用 `range` 渲染，避免 backend / frontend 两份重复模板：

```yaml
{{- range $key, $svc := dict "backend" .Values.backend "frontend" .Values.frontend }}
{{- if $svc.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $svc.name }}
spec:
  replicas: {{ $svc.replicaCount }}
  selector:
    matchLabels:
      app.service: {{ $svc.name }}
  template:
    metadata:
      labels:
        app.service: {{ $svc.name }}
    spec:
      containers:
      - name: {{ $svc.name }}
        image: "{{ $svc.image.repository }}:{{ $svc.image.tag }}"
{{- end }}
{{- end }}
```

### 关键约束

- ✅ **MUST** 含 `livenessProbe` + `readinessProbe`（后端 actuator 7777 端口）
- ✅ **MUST** 含 `resources.requests` 与 `resources.limits`
- ✅ **MUST** 镜像 tag 用具体版本号
- ✅ **MUST** 用 namespace 隔离环境
- ❌ **MUST NOT** 用 `latest` tag
- ❌ **MUST NOT** 在 YAML / values.yaml 硬编码 Secrets（用 External Secrets / Sealed Secrets）
- ❌ **MUST NOT** 用 `hostNetwork: true` / `hostPID: true`

## 2.2 kubectl 运维

### 上下文与命名空间

```bash
kubectl config current-context
kubectl config use-context <context>                      # L0，MUST 确认
kubectl config set-context --current --namespace=<ns>     # L0，MUST 确认
kubectl -n <ns> get pods                                  # 推荐：临时指定，不改默认
```

### 查看（只读，安全）

```bash
kubectl get pods -n <ns> -o wide
kubectl get pods -n <ns> --watch
kubectl get deployment/svc/ingress/all -n <ns>
kubectl describe deployment <name> -n <ns>
kubectl get events -n <ns> --sort-by='.lastTimestamp'
kubectl top nodes && kubectl top pods -n <ns>
```

### 日志与调试

```bash
kubectl logs -f <pod> -n <ns>
kubectl logs --tail=100 <pod> -n <ns>
kubectl logs <pod> -c <container> -n <ns>     # 多容器 Pod
kubectl logs <pod> --previous -n <ns>         # 上次崩溃前日志
kubectl exec -it <pod> -n <ns> -- /bin/sh
kubectl port-forward svc/<svc> -n <ns> 8080:80
```

### 写操作（L0，MUST 确认）

```bash
kubectl diff -f deployment.yaml               # MUST 先预览
kubectl apply -f ./k8s/ --dry-run=client      # 再干跑
kubectl apply -f ./k8s/
kubectl delete -f deployment.yaml             # L0
kubectl scale deployment/<name> --replicas=3
kubectl rollout restart deployment/<name>
```

### 滚动更新与回滚

```bash
kubectl rollout status  deployment/<name> -n <ns>
kubectl rollout history deployment/<name> -n <ns>
kubectl rollout undo    deployment/<name> -n <ns>                    # L0
kubectl rollout undo    deployment/<name> --to-revision=<n> -n <ns>  # L0
```

### 配置与节点

```bash
kubectl get configmap -n <ns> && kubectl describe configmap <name> -n <ns>
kubectl get secret <name> -n <ns> -o jsonpath='{.data.password}' | base64 -d
kubectl get nodes && kubectl describe node <node>
kubectl cordon <node>                                  # 维护：标记不可调度
kubectl drain  <node> --ignore-daemonsets              # L0
kubectl uncordon <node>
```

### 关键约束

- ✅ **MUST** 用 `-n <ns>` 显式指定命名空间
- ✅ **MUST** 写操作前 `kubectl diff` 预览
- ❌ **MUST NOT** 在生产 `default` namespace 操作
- ❌ **MUST NOT** 未确认执行 `delete` / `drain`

## 2.3 Helm 运维

### 仓库与查看

```bash
helm repo add <name> <url> && helm repo update
helm search repo <keyword>
helm show chart  <repo>/<chart>
helm show values <repo>/<chart>

helm list -n <ns>
helm status   <release> -n <ns>
helm get values   <release> -n <ns>
helm get manifest <release> -n <ns>
helm history  <release> -n <ns>
```

### 安装与升级

```bash
# 干跑预览（生产 MUST 先跑这条）
helm install <release> <repo>/<chart> --dry-run --debug -n <ns>

# 幂等安装或升级（推荐写法）
helm upgrade --install <release> <repo>/<chart> \
  -n <ns> \
  -f values-prod.yaml \
  --set image.tag=1.2.0
```

### 回滚与卸载（L0，MUST 确认）

```bash
helm rollback  <release> -n <ns>
helm rollback  <release> <revision> -n <ns>
helm uninstall <release> -n <ns>
```

### Chart 调试

```bash
helm template <release> ./<chart-dir> -n <ns> -f values.yaml   # 渲染不写集群
helm lint     ./<chart-dir>
helm package  ./<chart-dir>
```

### 关键约束

- ✅ **MUST** 用 `helm upgrade --install`（幂等）
- ✅ **MUST** 用 `-n <ns>` 显式指定命名空间
- ✅ **MUST** 生产环境先 `--dry-run --debug` 预览
- ✅ **MUST** 用 `-f values-<env>.yaml` 区分环境
- ❌ **MUST NOT** 在 values.yaml 硬编码 Secrets
- ❌ **MUST NOT** 用 `latest` tag

## 2.4 部署验证（MUST 6 项全过）

| # | 检查项 | 命令 | 通过标准 |
|---|---|---|---|
| 1 | Deployment | `kubectl get deployment -n <ns>` | `READY` = `UP-TO-DATE` = `AVAILABLE` = 期望副本数 |
| 2 | Pod | `kubectl get pods -n <ns>` | 全部 `Running`、`RESTARTS` 为 0（或极少）、日志无 ERROR |
| 3 | Service | `kubectl get endpoints -n <ns>` | Endpoints 含所有 Pod IP |
| 4 | Ingress | `kubectl get ingress -n <ns>` | 已分配 `ADDRESS` |
| 5 | HPA（如启用） | `kubectl get hpa -n <ns>` | `TARGETS` 有值、`REPLICAS` 在 min/max 之间 |
| 6 | 应用健康 | `kubectl port-forward svc/<svc> -n <ns> 8080:80` + `curl /actuator/health` | 返回 `{"status":"UP"}` |

写入 `changes/proposals/<current>/deployment.md`：

```markdown
## K8s 部署验证

- [ ] Deployment 就绪
- [ ] Pod 全部 Running
- [ ] Service Endpoints 正常
- [ ] Ingress 已分配地址
- [ ] HPA 正常（如启用）
- [ ] 应用健康检查通过
- [ ] 日志无 ERROR

**结论**：✅ 部署成功 / ❌ 部署失败（原因）
```

### 失败排查

| 现象 | 排查 |
|---|---|
| Pod Pending | `kubectl describe pod` 看 Events（资源不足 / 镜像拉取失败 / 调度限制） |
| Pod CrashLoopBackOff | `kubectl logs --previous` 看上次崩溃日志 |
| Service 无 Endpoints | 检查 Pod 是否 Running + label selector 是否匹配 |
| Ingress 无 ADDRESS | 检查 Ingress Controller 是否运行 |

- ✅ **MUST** 6 项全部通过才判定部署成功
- ❌ **MUST NOT** 任何一项失败仍报成功

---

# 三、Terraform / IaC

> 用 Terraform 管理云资源。**apply / destroy / 状态后端变更均为 L0，MUST 用户确认**。

## 3.1 项目结构

```
terraform/
├── main.tf              # 主入口
├── variables.tf         # 变量定义
├── outputs.tf           # 输出定义
├── terraform.tfvars     # 变量值（不入库）
├── backend.tf           # 状态后端
├── modules/             # 自定义模块
│   ├── vpc/
│   ├── ecs/
│   └── rds/
└── environments/        # 环境区分
    ├── dev/
    ├── staging/
    └── prod/
```

## 3.2 关键文件模板

### backend.tf（状态后端）

```hcl
terraform {
  backend "oss" {
    bucket = "structure-terraform-state"
    key    = "prod/terraform.tfstate"
    region = "cn-hangzhou"
  }
}
```

### variables.tf

```hcl
variable "env" {
  description = "Environment name"
  type        = string
}

variable "region" {
  description = "Cloud region"
  type        = string
  default     = "cn-hangzhou"
}
```

### main.tf

```hcl
provider "alicloud" {
  region = var.region
}

module "vpc" {
  source = "./modules/vpc"
  env    = var.env
}

module "ecs" {
  source = "./modules/ecs"
  env    = var.env
  vpc_id = module.vpc.vpc_id
}
```

### outputs.tf

```hcl
output "vpc_id" {
  value = module.vpc.vpc_id
}

output "ecs_public_ip" {
  value = module.ecs.public_ip
}
```

## 3.3 常用命令

```bash
terraform init          # 初始化
terraform fmt           # 格式化
terraform validate      # 校验
terraform plan          # 预览（不写）
terraform apply         # L0，MUST 用户确认
terraform destroy       # L0，MUST 用户确认
terraform show
terraform state list
terraform output
```

## 关键约束

- ✅ **MUST** 状态远端存储（OSS / S3 / Terraform Cloud）
- ✅ **MUST** 用 `modules/` 复用配置
- ✅ **MUST** 用 `environments/` 区分环境
- ✅ **MUST** `apply` 前先 `plan` 预览
- ❌ **MUST NOT** 在 *.tf 硬编码 Secrets（用变量 + tfvars）
- ❌ **MUST NOT** 直接编辑远端状态

---

## 完成标准

- Dockerfile 语法正确，本地构建成功，容器健康检查通过
- `docker-compose config` 验证通过，`up -d` 后所有服务 `healthy`
- K8s 部署验证 6 项全过（2.4）
- Terraform `plan` 无意外差异，`apply` 后 `output` 符合预期
- 所有生产 / 清理 / 回滚 / apply 操作均有用户确认记录

## 关联

- 后续：`ci-pipeline` / `deployment-verification` / `release-ops`
- 相关：`observability`
- Wiki：`wiki/_common/docker.md`、`wiki/_common/kubernetes.md`、`wiki/_common/ci-cd-pipeline.md`
