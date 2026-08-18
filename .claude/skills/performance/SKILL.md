---
name: performance
description: |
  当用户要求"性能测试/压测/JMeter/K6/Gatling"或"性能调优/性能优化/接口太慢/系统太慢"时触发。
  先测量后优化：用压测建立基线 → 定位瓶颈 → 优化 → 复测验证。
  生产环境压测与调优 MUST 用户确认。

triggers:
  - 性能测试
  - 压力测试
  - 压测
  - 负载测试
  - JMeter
  - K6
  - Gatling
  - performance test
  - load test
  - 性能调优
  - 性能优化
  - 优化性能
  - 接口太慢
  - 系统太慢
  - 响应太慢
  - performance tuning

role: devops
phase: support

allowed-tools: Bash, Read, Write, Edit, Grep

related-rules:
  - common-core
  - common-runtime
  - common-testing

reads-before-action:
  - wiki/_common/performance.md
  - wiki/_common/cache-design.md
  - wiki/_common/database-design.md
  - wiki/_common/testing-strategies.md

produces:
  - 性能测试脚本（K6 / JMeter / Gatling）
  - 性能基线报告（QPS / P95 / P99 / 资源使用）
  - 性能诊断报告 + 优化实施
  - 优化后复测对比报告

requires:
  - skill: testing
    condition: 集成测试已通过

human-in-the-loop:
  - id: confirm-production-load-test
    action: 对生产环境压测 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-production-tuning
    action: 对生产环境调整参数（堆 / GC / 线程池 / 索引）MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: select-optimization-approach
    action: 多个优化手段可选时 MUST 与用户确认取舍
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 优化未达预期可按 git 回退单项改动，逐项复测

on-failure: |
  压测未达阈值 → MUST NOT 放宽 thresholds 蒙混过关，进入调优流程定位瓶颈
  优化后指标未改善 → 回退该项改动，回到定位阶段，MUST NOT 叠加多个未验证的改动
  无压测环境 → 明说"未执行，原因 X"，MUST NOT 用生产环境替代

category: support
stack: _common
priority: medium
---

# 性能

> 压测建立基线 → 定位瓶颈 → 优化 → 复测验证。**先测量，后优化**；**生产操作 MUST 用户确认（L0）**。

---

# 一、性能测试（建立基线）

## 工具选择

| 工具 | 适用 | 推荐度 |
|---|---|---|
| **K6** ⭐ | 现代化 / 易编写 | 推荐 |
| JMeter | 传统 / 功能全 | 备选 |
| Gatling | 高性能 / Scala | 备选 |

## K6 示例

```javascript
import http from 'k6/http'
import { check, sleep } from 'k6'

export const options = {
  stages: [
    { duration: '1m', target: 100 },  // 1 分钟爬到 100 并发
    { duration: '3m', target: 100 },  // 保持 3 分钟
    { duration: '1m', target: 0 },    // 1 分钟降到 0
  ],
  thresholds: {
    http_req_duration: ['p(99)<500'],  // P99 < 500ms
    http_req_failed: ['rate<0.01'],    // 错误率 < 1%
  },
}

export default function () {
  const res = http.get('http://localhost:8080/api/v1/users/1')
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  })
  sleep(1)
}
```

## 关键指标

| 指标 | 目标 |
|---|---|
| **P50** | < 100ms |
| **P95** | < 300ms |
| **P99** | < 500ms |
| **错误率** | < 0.1% |
| **QPS** | 按业务需求 |

## 关键约束

- ✅ **MUST** 用 `stages` 渐进加压
- ✅ **MUST** 设阈值（thresholds），未达标视为失败
- ✅ **MUST** 压测前确认目标环境
- ❌ **MUST NOT** 压生产环境（除非走 L0 确认）
- ❌ **MUST NOT** 为了"通过"而放宽 thresholds

---

# 二、性能调优（消除瓶颈）

## 第 1 步：测量

```bash
# 接口延迟
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8080/api/v1/users/1

# JVM 状态
jstat -gc <pid> 1000

# 线程
jstack <pid>

# 堆 dump
jmap -dump:live,format=b,file=heap.hprof <pid>
```

## 第 2 步：定位瓶颈

按层次排查：

1. **网络**：延迟 / 带宽
2. **应用**：慢方法 / N+1 / 锁竞争
3. **数据**：慢查询 / 索引缺失
4. **缓存**：命中率低 / 穿透
5. **JVM**：GC 频繁 / 内存不足

## 第 3 步：优化

| 问题 | 优化 |
|---|---|
| N+1 查询 | JOIN / 批量查询 |
| 慢查询 | 索引 / 重写 SQL |
| 缓存穿透 | 缓存空值 / 布隆过滤 |
| 线程池耗尽 | 调整大小 / 拆分池 |
| GC 频繁 | 调堆 / 换 G1 / ZGC |
| 大对象 | 分页 / 流式处理 |

**MUST 一次只改一项**，否则无法归因。

## 第 4 步：复测验证

用第一部分的同一套压测脚本与阈值复跑，贴出优化前后指标对比。指标未改善 → 回退该项改动。

## 报告模板

```markdown
# 性能测试报告

## 测试环境
- 硬件：16C 32G SSD ｜ 版本：v1.2.0 ｜ 数据量：100 万用户

## 核心接口性能
| 接口 | QPS | P99 | 错误率 | 状态 |
|---|---|---|---|---|
| 查询用户列表 | 1500 | 120ms | 0.001% | ✅ |
| 导出报表 | 50 | 2000ms | 0% | ⚠️ 需优化 |

## 瓶颈分析
1. **导出报表 P99 = 2s**
   - 原因：全表扫描 + 无分页
   - 建议：分页导出 + 异步处理
   - 预期收益：P99 → 200ms

## 建议（按优先级，每项 MUST 给出预期收益与实施成本）
- [P0] 优化导出报表（影响用户体验）
- [P1] 解决缓存穿透问题
```

## 完成标准

- 有优化前基线报告与优化后复测报告，两份用同一脚本同一阈值
- 每项改动可单独归因
- thresholds 未被放宽

## 关联

- 前置：`testing`
- 相关：`debug-issue` / `observability`
- Wiki：`wiki/_common/performance.md`
