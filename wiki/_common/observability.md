# 可观测性规范

> 本文档是可观测性（Logging + Metrics + Tracing）的参考手册。

## 三位一体

```
Logging（日志）+ Metrics（指标）+ Tracing（追踪）= 可观测性
```

## Logging（日志）

### 级别

| 级别 | 用途 | 示例 |
|---|---|---|
| `DEBUG` | 调试细节 | 方法参数 / 中间结果 |
| `INFO` | 关键流程 | 用户登录 / 订单创建 |
| `WARN` | 业务异常 | 参数校验失败 / 业务规则不满足 |
| `ERROR` | 系统异常 | NPE / DB 连接失败 |

### 关键约束

- ✅ **MUST** 用 slf4j（`log.info` / `log.warn` / `log.error`）
- ✅ **MUST** 关键流程含 `traceId`
- ❌ **MUST NOT** 用 `System.out.println`
- ❌ **MUST NOT** 打印敏感信息

### MDC（含 traceId）

```java
// 在 Filter / Interceptor 设置
MDC.put("traceId", UUID.randomUUID().toString());

// 在日志输出
logging:
  pattern:
    console: "%d{yyyy-MM-dd HH:mm:ss} [%thread] [%X{traceId}] %-5level %logger{36} - %msg%n"
```

## Metrics（指标）

### 四类黄金指标

| 类型 | 指标 | 说明 |
|---|---|---|
| **延迟**（Latency） | `http_server_requests_seconds` | P50 / P95 / P99 |
| **流量**（Traffic） | `http_server_requests_total` | QPS |
| **错误**（Errors） | `http_server_requests_total{status=~"5.."}` | 错误率 |
| **饱和度**（Saturation） | `jvm_memory_used_bytes` / `process_cpu_usage` | 资源使用 |

### 业务指标

```java
@Component
public class OrderMetrics {
    private final Counter orderCreated = Metrics.counter("order_created_total");
    private final Timer orderProcessTime = Metrics.timer("order_process_seconds");
    private final Gauge orderInProgress = Metrics.gauge("order_in_progress", new AtomicInteger(0));
}
```

### Prometheus 端点

```yaml
management:
  endpoints:
    web:
      exposure:
        include: health,info,metrics,prometheus
  endpoint:
    prometheus:
      enabled: true
```

## Tracing（追踪）

### Spring Cloud Sleuth + Micrometer Tracing

```yaml
management:
  tracing:
    sampling:
      probability: 1.0  # 生产 0.1（10% 采样）
```

### 关键概念

- **Trace**：一次完整请求链路
- **Span**：链路中的一个操作
- **TraceId**：一次请求的唯一标识
- **SpanId**：一个 Span 的唯一标识

### 手动创建 Span

```java
@Observed(name = "user.login", contextualName = "user-login")
public UserVO login(LoginDTO dto) {
    // ...
}
```

## 告警（Alerting）

### 关键告警

| 告警 | 条件 | 严重度 |
|---|---|---|
| 服务 Down | `up == 0` 持续 1 分钟 | 🔴 P0 |
| 高错误率 | 5xx 错误率 > 1% 持续 5 分钟 | 🔴 P0 |
| 高延迟 | P99 > 2s 持续 5 分钟 | 🟠 P1 |
| 高 CPU | CPU > 80% 持续 10 分钟 | 🟠 P1 |
| 高内存 | 内存 > 90% 持续 10 分钟 | 🟠 P1 |
| 磁盘满 | 磁盘 > 90% | 🟠 P1 |
| 慢查询 | 慢查询数 > 10 / 分钟 | 🟡 P2 |

### Prometheus 告警规则

```yaml
groups:
- name: service-alerts
  rules:
  - alert: ServiceDown
    expr: up == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Service {{ $labels.job }} is down"

  - alert: HighErrorRate
    expr: rate(http_server_requests_total{status=~"5.."}[5m]) / rate(http_server_requests_total[5m]) > 0.01
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "High error rate on {{ $labels.job }}"
```

## 关键约束

- ✅ **MUST** 所有服务暴露 `/actuator/health` 和 `/actuator/prometheus`
- ✅ **MUST** 日志含 `traceId`
- ✅ **MUST** 关键指标有告警
- ❌ **MUST NOT** 在生产环境用 100% trace 采样

## 关联

- Wiki：`wiki/_common/logging.md` `wiki/_common/performance.md`
- 规则：`common-observability`
- 技能：`observability` / `debug-issue` / `performance`
