# Kubernetes 规范

> 本文档是 structure-projects 生态 K8s 部署的参考手册。

## 部署方式选择

| 方式 | 适用 | 推荐度 |
|---|---|---|
| **Helm Chart** ⭐ | 生产环境 | 生态标准（见 structure-pro/helm/） |
| **原生 YAML** | 简单场景 / 学习 | 入门 / 测试 |
| **Kustomize** | 多环境覆盖 | 备选 |

**生态标准 MUST 用 Helm Chart**。

## Helm Chart 双 workload 模板

### Chart 结构

```
structure-<X>-center/
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
    └── tests/
        └── test-connection.yaml
```

### Chart.yaml

```yaml
apiVersion: v2
name: structure-user-center
description: structure-projects 用户中心
type: application
version: 1.2.0
appVersion: "1.2.0"
```

### values.yaml（双 workload）

```yaml
# 后端
backend:
  enabled: true
  name: user-service
  image:
    repository: registry.cn-hangzhou.aliyuncs.com/structured/user-service
    tag: "1.2.0"
    pullPolicy: Always
  service:
    type: ClusterIP
    port: 8080
  env:
    APP_PATH: /app/boot/app.jar
    JAVA_OPTS: -Xms256m -Xmx1024m
    PARAMS: -Dspring.profiles.active=pro
  replicaCount: 1
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "1Gi"
      cpu: "1000m"

# 前端
frontend:
  enabled: true
  name: user-ui
  image:
    repository: registry.cn-hangzhou.aliyuncs.com/structured/user-ui
    tag: "1.2.0"
  service:
    port: 80
  env:
    SCHEME: https
    SERVER_HOST: api.prod.structured.cn
    SERVER_PORT: "443"
  replicaCount: 1

# HPA
autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 3
  targetCPUUtilizationPercentage: 80

# Ingress
ingress:
  enabled: false
  className: "nginx"
  hosts:
    - host: user.example.com
      paths:
        - path: /
          pathType: Prefix
```

### 双 workload 渲染（templates/deployment.yaml）

```yaml
{{- range $key, $svc := dict "backend" .Values.backend "frontend" .Values.frontend }}
{{- if $svc.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $svc.name }}
  namespace: {{ $.Release.Namespace }}
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
        imagePullPolicy: {{ $svc.image.pullPolicy }}
        ports:
        - containerPort: {{ $svc.service.port }}
        env:
        {{- range $k, $v := $svc.env }}
        - name: {{ $k }}
          value: {{ $v | quote }}
        {{- end }}
        {{- if $svc.resources }}
        resources:
          {{- toYaml $svc.resources | nindent 10 }}
        {{- end }}
---
{{- end }}
{{- end }}
```

## Probe 规范

### 后端（Spring Boot Actuator）

```yaml
livenessProbe:
  httpGet:
    path: /actuator/health
    port: 7777  # 生态约定
  initialDelaySeconds: 60
  periodSeconds: 30
  timeoutSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /actuator/health
    port: 7777
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

### 前端（Nginx）

```yaml
livenessProbe:
  httpGet:
    path: /
    port: 80
  initialDelaySeconds: 10
  periodSeconds: 30
```

## Resources 规范

**MUST 设置** `requests` 和 `limits`：

| 服务类型 | requests.memory | limits.memory | requests.cpu | limits.cpu |
|---|---|---|---|---|
| 小型后端 | 256Mi | 512Mi | 250m | 500m |
| 中型后端 | 512Mi | 1Gi | 500m | 1000m |
| 大型后端 | 1Gi | 2Gi | 1000m | 2000m |
| 前端 | 64Mi | 128Mi | 50m | 100m |

## HPA 规范

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ $svc.name }}-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ $svc.name }}
  minReplicas: 1
  maxReplicas: 3
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 80
```

## Secrets 管理

**禁止**：
- ❌ 在 YAML 硬编码 Secrets
- ❌ 提交 Secrets 到 Git

**推荐**：
- ✅ External Secrets Operator
- ✅ Sealed Secrets
- ✅ Vault Agent Injector

## 关联

- 技能：`infra-ops` / `deployment-verification`
- Wiki：`wiki/_common/docker.md` `wiki/_common/ci-cd-pipeline.md`
- 参考实现：`structure-pro/helm/` / `structure-iam/.../deploy/helm/`
