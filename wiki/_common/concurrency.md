# 并发编程规范

> 本文档是并发编程的参考手册。

## 共享可变状态

### 禁止（MUST NOT）

- ❌ **禁止** 在单例 Bean 中用可变实例字段
- ❌ **禁止** 用 `static` 可变字段共享状态
- ❌ **禁止** 在 Controller / Service 用成员变量存请求级状态

### 推荐

- ✅ **MUST** 用 `final` 字段
- ✅ **MUST** 用不可变对象
- ✅ **MUST** 请求级状态用 `ThreadLocal` 或参数传递
- ✅ **MUST** 共享计数用 `AtomicInteger` / `LongAdder`

## 线程池

### 必须用线程池（MUST）

❌ **禁止** `new Thread().start()`
✅ **MUST** 用线程池

### 线程池配置

```java
@Bean
public ThreadPoolTaskExecutor bizExecutor() {
    ThreadPoolTaskExecutor executor = new ThreadPoolTaskExecutor();
    executor.setCorePoolSize(10);
    executor.setMaxPoolSize(20);
    executor.setQueueCapacity(100);
    executor.setThreadNamePrefix("biz-");
    executor.setRejectedExecutionHandler(new ThreadPoolExecutor.CallerRunsPolicy());
    executor.setWaitForTasksToCompleteOnShutdown(true);
    executor.initialize();
    return executor;
}
```

**关键**：
- ✅ **MUST** 设置合理的 core / max / queue
- ✅ **MUST** 设置线程名前缀（便于排查）
- ✅ **MUST** 设置拒绝策略
- ❌ **MUST NOT** 用无界队列（`LinkedBlockingQueue` 默认无界）

### 常见拒绝策略

| 策略 | 行为 | 适用 |
|---|---|---|
| `CallerRunsPolicy` ⭐ | 调用方线程执行 | 默认推荐 |
| `AbortPolicy` | 抛异常 | 严格场景 |
| `DiscardPolicy` | 静默丢弃 | 不推荐 |
| `DiscardOldestPolicy` | 丢弃最老任务 | 不推荐 |

## 异步处理

### @Async

```java
@Async("bizExecutor")
public CompletableFuture<Result> processAsync(...) {
    // ...
}
```

**关键**：
- ✅ **MUST** 指定线程池（`@Async("bizExecutor")`）
- ❌ **MUST NOT** 在 `@Transactional` 方法内调用 `@Async`（事务失效）

## 幂等性设计（MUST）

并发场景 MUST 幂等：

### 数据库层

```sql
-- 唯一约束防重复
UNIQUE KEY `uk_tenant_username` (`tenant_id`, `username`)
```

### 缓存层

```java
String key = "idem:" + requestId;
Boolean first = redisTemplate.opsForValue()
    .setIfAbsent(key, "1", 24, TimeUnit.HOURS);
if (Boolean.FALSE.equals(first)) {
    return cachedResult;
}
```

### 接口层

```
POST /api/v1/orders
Idempotency-Key: <uuid>
```

## 并发集合

| 场景 | 推荐 |
|---|---|
| 读多写少 Map | `ConcurrentHashMap` |
| 读多写少 List | `CopyOnWriteArrayList` |
| 阻塞队列 | `LinkedBlockingQueue`（有界） |
| 计数器 | `LongAdder`（高并发）/ `AtomicInteger` |

## 锁

### 本地锁

```java
// 推荐：ConcurrentHashMap 替代
private final Map<String, Object> locks = new ConcurrentHashMap<>();

public void process(String key) {
    Object lock = locks.computeIfAbsent(key, k -> new Object());
    synchronized (lock) {
        // ...
    }
}
```

### 分布式锁

详见 `wiki/_common/cache-design.md`。

## 关联

- Wiki：`wiki/_common/cache-design.md` `wiki/_common/performance.md`
- 规则：`common-runtime`
- 技能：`coding` / `debug-issue`
