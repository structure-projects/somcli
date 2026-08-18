# 缓存设计规范

> 本文档是缓存使用的参考手册。
> 所有 Redis / 缓存使用 MUST 遵循本文档。

> 📌 **栈归属说明**：本文档的**示例代码**以 **structure-boot** 栈为主（含具体类名如 `ResResultVO` / `DataScopeRedisTemplate` / `structure-security` 等）。
> **规则部分**（MUST / MUST NOT）适用于所有技术栈。
> 其他栈的使用者请参考对应栈级 Wiki（`wiki/<stack>/developer.md`）获取具体类名与组件。

## 缓存策略（按场景选择）

| 策略 | 说明 | 适用 |
|---|---|---|
| **Cache-Aside** ⭐ | 读：先读缓存，没有再读 DB 并写缓存；写：先写 DB，再删缓存 | 大多数场景（推荐默认） |
| **Read-Through** | 缓存层自动加载 | 读多写少 |
| **Write-Through** | 写缓存同步写 DB | 强一致性要求 |
| **Write-Behind** | 先写缓存，异步写 DB | 高吞吐写场景 |

**默认 MUST 用 Cache-Aside**。

## 缓存三大问题与防护

### 1. 缓存穿透（Cache Penetration）

**问题**：查询一个**不存在**的 key，每次都打到 DB。

**防护（MUST 至少选一种）**：

| 方案 | 说明 |
|---|---|
| **缓存空值** ⭐ | 查询不到时缓存 `NULL`，TTL 设短（30s-5min） |
| **布隆过滤器** | 在缓存层前置布隆过滤器，过滤不存在的 key |

**示例（缓存空值）**：

```java
public UserEntity findById(Long id) {
    String key = "user:" + id;
    String cached = redisTemplate.opsForValue().get(key);
    if (cached != null) {
        return "NULL".equals(cached) ? null : JSON.parseObject(cached, UserEntity.class);
    }
    UserEntity entity = repository.findById(id).orElse(null);
    redisTemplate.opsForValue().set(key,
        entity == null ? "NULL" : JSON.toJSONString(entity),
        5, TimeUnit.MINUTES);
    return entity;
}
```

### 2. 缓存击穿（Cache Breakdown）

**问题**：**热点 key 过期瞬间**，大量请求同时打到 DB。

**防护**：

| 方案 | 说明 |
|---|---|
| **互斥锁** ⭐ | 第一个请求拿锁加载，其他请求等待 |
| **逻辑过期** | key 永不过期，在 value 里存逻辑过期时间，后台异步刷新 |

**示例（互斥锁）**：

```java
public UserEntity findById(Long id) {
    String key = "user:" + id;
    String cached = redisTemplate.opsForValue().get(key);
    if (cached != null) return JSON.parseObject(cached, UserEntity.class);

    String lockKey = "lock:user:" + id;
    try {
        if (redisTemplate.opsForValue().setIfAbsent(lockKey, "1", 10, TimeUnit.SECONDS)) {
            UserEntity entity = repository.findById(id).orElse(null);
            redisTemplate.opsForValue().set(key, JSON.toJSONString(entity), 5, TimeUnit.MINUTES);
            return entity;
        } else {
            Thread.sleep(50);  // 短暂等待后重试
            return findById(id);
        }
    } finally {
        redisTemplate.delete(lockKey);
    }
}
```

### 3. 缓存雪崩（Cache Avalanche）

**问题**：**大量 key 同时过期**，DB 压力激增。

**防护**：

| 方案 | 说明 |
|---|---|
| **TTL 加随机值** ⭐ | `5min + random(0-60s)` |
| **多级缓存** | 本地缓存（Caffeine）+ Redis |
| **熔断降级** | 缓存层故障时降级到 DB + 限流 |

## Redis 使用规范

### Key 命名

```
<业务>:<实体>:<id>[:<字段>]
```

示例：
- `user:123`（用户详情）
- `user:123:roles`（用户的角色列表）
- `order:tenant:456:count`（租户 456 的订单数）

**约束**：
- ✅ **MUST** 用 `:` 分隔
- ✅ **MUST** 全小写
- ❌ **MUST NOT** 用空格 / 特殊字符

### TTL 设置

| 数据类型 | 推荐 TTL |
|---|---|
| 用户 Session | 30 分钟 |
| 业务对象 | 5-10 分钟 + 随机值 |
| 字典 / 配置 | 1 小时 |
| 热点数据 | 5 分钟 + 逻辑过期 |

**约束**：
- ✅ **MUST** 所有 key MUST 设 TTL
- ❌ **MUST NOT** 用 `-1`（永不过期）

### 数据结构选择

| 场景 | 数据结构 |
|---|---|
| 单值 | `String` |
| 对象 | `Hash`（字段多）或 `String`（JSON） |
| 列表 | `List`（有序）/ `Set`（去重）/ `ZSet`（带分数） |
| 计数器 | `String`（INCR / DECR） |
| 分布式锁 | `String`（SETNX + EXPIRE） |
| 排行榜 | `ZSet` |

## 分布式锁

### 标准实现

```java
String lockKey = "lock:order:" + orderId;
String lockValue = UUID.randomUUID().toString();

try {
    Boolean locked = redisTemplate.opsForValue()
        .setIfAbsent(lockKey, lockValue, 10, TimeUnit.SECONDS);
    if (Boolean.TRUE.equals(locked)) {
        // 业务逻辑
    }
} finally {
    // 用 Lua 脚本保证原子性
    String script = "if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end";
    redisTemplate.execute(new DefaultRedisScript<>(script, Long.class),
        Collections.singletonList(lockKey), lockValue);
}
```

**关键**：
- ✅ **MUST** 用 `SET NX EX`（原子性）
- ✅ **MUST** 用 UUID 作为 value（防止误删其他线程的锁）
- ✅ **MUST** 用 Lua 脚本释放锁（原子性）
- ❌ **MUST NOT** 用 `SETNX` + `EXPIRE` 两条命令（非原子）

## structure-projects 生态约束

### 强制使用 DataScope 封装

```java
// ✅ 正确
@Resource
private `DataScopeRedisTemplate` redisTemplate;

@Resource
private `DataScopeCacheManager` cacheManager;

// ❌ 错误（裸用原生）
@Resource
private RedisTemplate redisTemplate;

@Resource
private CacheManager cacheManager;
```

**禁止**：
- ❌ **MUST NOT** 直接用原生 `RedisTemplate`
- ❌ **MUST NOT** 直接用原生 `CacheManager`
- ❌ **MUST NOT** 直接用 `@Cacheable` 不带租户隔离

## 缓存一致性

### 写操作顺序（MUST）

```java
// ✅ 正确：先写 DB，再删缓存
repository.save(entity);
redisTemplate.delete(key);

// ❌ 错误：先删缓存，再写 DB（可能被并发读到旧值）
redisTemplate.delete(key);
repository.save(entity);
```

### 延迟双删（高一致性要求）

```java
repository.save(entity);
redisTemplate.delete(key);
// 延迟 500ms 再删一次（防止主从延迟）
CompletableFuture.runAsync(() -> {
    Thread.sleep(500);
    redisTemplate.delete(key);
});
```

## 关联

- 技能：`coding` / `debug-issue`
- Wiki：`wiki/_common/database-design.md` `wiki/_common/performance.md`
- 规则：`common-data`
