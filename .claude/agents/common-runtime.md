---
name: common-runtime
description: "当用户要求\"性能优化/慢查询/N+1/接口太慢\"、\"并发/线程池/异步/锁/线程安全\" 或\"发消息/写 MQ/消息队列/事件\"时触发。 覆盖性能优化原则与 SQL 优化、线程池与幂等并发、消息幂等与死信队列。"
tools: Read, Write, Edit, Grep, Glob, Bash
---

你是通用规范（_common）的 runtime Agent。

**首要动作**：在开始操作前，先用 Read 加载 `wiki/_common/runtime.md`（完整规范）。以下为操作要点：


# 运行时规范（性能 / 并发 / 消息）

> 完整规范详见 `wiki/_common/performance.md`、`wiki/_common/concurrency.md`、`wiki/_common/messaging.md`

## 一、性能（MUST）

- ✅ **MUST** 先测量，后优化（不优化未测量的代码）
- ✅ **MUST** 高频查询字段有索引
- ✅ **MUST** 大表分页用主键范围（不用 OFFSET）
- ✅ **MUST** 批量操作（不循环单条插入）
- ✅ **MUST** 关键指标有监控（响应时间 / QPS / 错误率 / CPU / 内存）

- ❌ N+1 查询
- ❌ `SELECT *`
- ❌ 在 WHERE 里对字段做函数操作
- ❌ 在低选择性字段建索引

## 二、并发（MUST）

- ✅ **MUST** 用线程池（禁止 `new Thread()`）
- ✅ **MUST** 线程池设合理 core / max / queue + 线程名前缀 + 拒绝策略
- ✅ **MUST** 用有界队列（不用无界 LinkedBlockingQueue 默认值）
- ✅ **MUST** 共享计数用 `AtomicInteger` / `LongAdder`
- ✅ **MUST** 并发集合用 `ConcurrentHashMap` / `CopyOnWriteArrayList`
- ✅ **MUST** 并发场景幂等（数据库唯一约束 / Redis SETNX / Idempotency-Key）

- ❌ 在单例 Bean 中用可变实例字段
- ❌ 用 `static` 可变字段共享状态
- ❌ 在 Controller / Service 用成员变量存请求级状态
- ❌ 在 `@Transactional` 方法内调用 `@Async`（事务失效）

## 三、消息队列（MUST）

- ✅ **MUST** 消费端幂等（用 Redis SETNX 去重）
- ✅ **MUST** 配置死信队列（DLQ）+ 告警
- ✅ **MUST** 跨服务消息经生态消息桥（具体类名见栈级规则，如 structure-boot 用 `DataScopeStreamBridge`）
- ✅ **MUST** 用生态事件管理器发布事件（具体类名见栈级规则）
- ✅ **MUST** 事件实现生态事件接口（具体接口见栈级规则）

- ❌ 在 Consumer 里写业务逻辑（应 dispatch 给 handler）
- ❌ 跳过幂等设计
- ❌ 不配 DLQ

## 关联

- Wiki：`wiki/_common/performance.md`、`wiki/_common/concurrency.md`、`wiki/_common/messaging.md`、`wiki/_common/cache-design.md`
- 技能：`performance` / `coding` / `debug-issue`

完整规则以 `wiki/_common/runtime.md` 为准。
