# 性能优化规范

> 本文档是性能优化的参考手册。

## 性能优化原则

1. **先测量，后优化** —— 不优化未测量的代码
2. **找到瓶颈** —— 用 profiler 定位，不猜
3. **优化瓶颈** —— 20% 的代码占 80% 的时间
4. **验证效果** —— 优化后再次测量

## 常见性能问题与优化

### N+1 查询（最常见）

**问题**：
```java
// ❌ 每个 user 一次查询
List<User> users = userRepository.findAll();
for (User user : users) {
    user.getOrders().size();  // 每个 user 触发一次订单查询
}
```

**修复**：
```java
// ✅ 一次性 JOIN 查询
List<User> users = userRepository.findAllWithOrders();
```

### 慢查询

**排查**：
```sql
-- 开启慢查询日志
SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 1;  -- 1 秒

-- 用 EXPLAIN 分析
EXPLAIN SELECT * FROM user WHERE username = 'xxx';
```

**优化**：
- 加索引
- 重写 SQL
- 分库分表

### 内存泄漏

**常见原因**：
- 静态集合持有对象
- 未关闭的资源（Stream / Connection）
- ThreadLocal 未清理
- 监听器未反注册

**排查**：
```bash
# JVM 堆分析
jmap -dump:live,format=b,file=heap.hprof <pid>

# 用 MAT 分析 heap.hprof
```

### 线程池耗尽

**症状**：任务队列堆积 / 拒绝

**优化**：
- 调整线程池大小
- 用有界队列 + 拒绝策略
- 拆分慢任务到独立线程池

## JVM 调优

### 堆大小

```bash
-Xms2g -Xmx2g  # 初始 = 最大，避免动态扩容
```

### GC 选择

| JDK | 推荐 GC | 说明 |
|---|---|---|
| JDK 8 | G1 | 默认 |
| JDK 11+ | G1 | 默认 |
| JDK 17+ | G1 或 ZGC | ZGC 适合大堆（> 32G） |

### GC 日志

```bash
-Xlog:gc*:file=gc.log:time,uptime,level,tags
```

## SQL 优化

### 索引

- ✅ **MUST** 高频查询字段有索引
- ✅ **MUST** 联合索引用最左前缀
- ❌ **MUST NOT** 在低选择性字段建索引

### 分页

```sql
-- ❌ 慢：OFFSET 大时分页慢
SELECT * FROM user LIMIT 1000000, 10;

-- ✅ 快：用主键范围
SELECT * FROM user WHERE id > 1000000 LIMIT 10;
```

### 批量操作

```sql
-- ❌ 慢：单条插入
INSERT INTO user VALUES (...);
INSERT INTO user VALUES (...);

-- ✅ 快：批量插入
INSERT INTO user VALUES (...), (...), ...;
```

## 缓存优化

详见 `wiki/_common/cache-design.md`。

## 前端性能

### 资源优化

- ✅ **MUST** 开启 gzip / brotli
- ✅ **MUST** 静态资源 CDN
- ✅ **MUST** 图片懒加载
- ✅ **MUST** 代码分割

### 渲染优化

- ✅ **MUST** 虚拟滚动（大列表）
- ✅ **MUST** 组件懒加载
- ❌ **MUST NOT** 在 `watch` 里做重计算

## 监控指标（MUST 关注）

| 指标 | 阈值 | 说明 |
|---|---|---|
| **响应时间 P99** | < 500ms | 99% 请求在 500ms 内 |
| **QPS** | 按业务 | 每秒查询数 |
| **错误率** | < 0.1% | HTTP 5xx |
| **CPU** | < 70% | |
| **内存** | < 80% | |
| **GC 停顿** | < 100ms | |
| **慢查询数** | 0 | 每日统计 |

## 关联

- Wiki：`wiki/_common/cache-design.md` `wiki/_common/database-design.md` `wiki/_common/observability.md`
- 规则：`common-runtime`
- 技能：`debug-issue` / `performance`
