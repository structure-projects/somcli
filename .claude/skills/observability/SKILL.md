---
name: observability
description: |
  当用户要求"配置监控/接入 Prometheus/配置告警/接入 Grafana"或"分析日志/查日志/日志排查"时触发。
  可观测全流程：指标暴露与告警配置（Prometheus / Grafana）→ 日志检索与问题定位。
  告警阈值 MUST 用户确认；生产环境 MUST NOT 100% trace 采样。

triggers:
  - 配置监控
  - 接入 Prometheus
  - 配置告警
  - Grafana
  - 监控接入
  - 接入可观测
  - 分析日志
  - 查日志
  - 日志排查
  - log analysis
  - 看日志

role: devops
phase: support

allowed-tools: Bash, Read, Write, Edit, Grep

related-rules:
  - common-observability
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/observability.md
  - wiki/_common/logging.md

produces:
  - Prometheus 配置 + 告警规则 + Grafana Dashboard
  - 日志分析报告与问题定位结论

requires:
  - skill: deployment-verification
    condition: 配置监控（第一章）时服务已部署
    error: 服务未部署，监控无抓取目标

human-in-the-loop:
  - id: confirm-alert-threshold
    action: 告警阈值 MUST 用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 阈值不当只影响告警噪声，改配置重载即可

on-failure: |
  抓取目标 DOWN → 检查端点是否暴露、网络是否可达，MUST NOT 直接删除该 job
  告警不触发 → 用 Prometheus 表达式浏览器验证 expr，MUST NOT 靠调低阈值掩盖
  日志无关键信息 → 按 traceId 串联上下游，MUST NOT 凭单条日志下结论

category: support
stack: _common
priority: medium
---

# 可观测

> 指标与告警（Prometheus / Grafana）→ 日志检索与定位。**告警阈值 MUST 用户确认**。

按任务选章：接监控 → 一；查问题 → 二。

---

# 一、监控接入

## 第 1 步：暴露 Prometheus 端点

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

## 第 2 步：配置 Prometheus 抓取

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'user-service'
    metrics_path: '/actuator/prometheus'
    static_configs:
      - targets: ['user-service:8080']
```

## 第 3 步：配置告警规则（阈值 MUST 用户确认）

```yaml
# alert-rules.yml
groups:
- name: service
  rules:
  - alert: ServiceDown
    expr: up == 0
    for: 1m
    labels:
      severity: critical
```

## 第 4 步：Grafana Dashboard

导入或创建 Dashboard。

## 第 5 步：验证

```bash
curl http://localhost:8080/actuator/prometheus       # 端点
curl http://prometheus:9090/api/v1/targets           # 抓取状态
```

## 关键约束

- ✅ **MUST** 暴露 `/actuator/prometheus`
- ✅ **MUST** 配置关键告警（存活 / 错误率 / 延迟）
- ❌ **MUST NOT** 在生产环境 100% trace 采样

---

# 二、日志分析

## 实时跟随

```bash
tail -f logs/application.log      # 本地
kubectl logs -f <pod> -n <ns>     # K8s
docker logs -f <container>        # Docker
```

## 搜索

```bash
grep "ERROR" logs/application.log
grep "userId=123" logs/application.log
grep "2026-08-13 10:" logs/application.log     # 按时间
grep "traceId=abc123" logs/application.log     # 按链路
grep -c "ERROR" logs/application.log           # 统计
```

## ELK / Loki 查询

```
# Loki LogQL
{app="user-service"} |= "ERROR" |~ "userId=\\d+"
```

## 常见问题模式

| 模式 | 命令 |
|---|---|
| NPE / 空指针 | `grep "NullPointerException" logs/application.log -A 20` |
| SQL 慢查询 | `grep "slow query" logs/application.log` |
| OOM | `grep "OutOfMemoryError" logs/application.log` |

## 关键约束

- ✅ **MUST** 用 traceId 串联跨服务调用再下结论
- ❌ **MUST NOT** 凭单条日志断定根因

---

## 完成标准

- 监控：抓取目标 UP、关键告警规则生效、Dashboard 可见核心指标
- 日志：定位结论有日志证据（时间 + traceId + 堆栈），已写入分析报告

## 关联

- 前置：`deployment-verification`（监控场景）
- 相关：`debug-issue` / `infra-ops` / `release-ops`
- Wiki：`wiki/_common/observability.md`、`wiki/_common/logging.md`
