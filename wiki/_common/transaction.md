# 本地事务规范

> 本文档是本地事务（单服务单数据库）的参考手册。

## 事务边界

### 最小化原则（MUST）

**事务边界 MUST 最小化**：

```java
// ❌ 错误：大事务
@Transactional
public void processOrder(OrderDTO dto) {
    validateOrder(dto);              // 校验（不需要事务）
    Order order = saveOrder(dto);    // 需要事务
    sendNotification(dto);           // 发消息（不需要事务）
    updateStatistics(dto);           // 统计（不需要事务）
}

// ✅ 正确：只把需要的放事务里
public void processOrder(OrderDTO dto) {
    validateOrder(dto);              // 事务外
    Order order = saveOrderInTx(dto); // 事务内
    sendNotification(dto);           // 事务外
    updateStatistics(dto);           // 事务外
}

@Transactional
protected Order saveOrderInTx(OrderDTO dto) {
    return orderRepository.save(...);
}
```

## 禁止事项（MUST NOT）

- ❌ **禁止**在事务内做远程调用（Feign / HTTP）
- ❌ **禁止**在事务内做长时间操作（文件 IO / 大计算）
- ❌ **禁止**在事务内发消息（用 `@TransactionalEventListener`）
- ❌ **禁止**长事务（> 5s）

## 事务传播

| 传播级别 | 适用 |
|---|---|
| `REQUIRED`（默认） | 大多数场景 |
| `REQUIRES_NEW` | 需要独立事务（如审计日志） |
| `NESTED` | 嵌套事务（部分回滚） |
| `SUPPORTS` | 可有可无（如查询） |
| `NOT_SUPPORTED` | 不需要事务 |
| `NEVER` | 禁止事务 |

### 常见用法

```java
// 审计日志（独立事务，主事务失败不影响审计）
@Transactional(propagation = Propagation.REQUIRES_NEW)
public void auditLog(AuditEvent event) {
    auditRepository.save(event);
}
```

## 只读事务

```java
@Transactional(readOnly = true)
public UserVO findById(Long id) {
    // 只读查询
}
```

**优势**：
- 数据库优化（不开启写锁）
- MyBatis 优化（不 flush）

## 事务失效陷阱

### 陷阱 1：自调用

```java
@Service
public class UserService {
    public void method1() {
        this.method2();  // ❌ 自调用，事务失效
    }
    
    @Transactional
    public void method2() {
        // ...
    }
}
```

**修复**：注入自身代理或用 `AopContext.currentProxy()`。

### 陷阱 2：异常被吞

```java
@Transactional
public void method() {
    try {
        // ...
    } catch (Exception e) {
        // ❌ 吞异常，事务不回滚
    }
}
```

**修复**：让异常抛出或手动 `TransactionAspectSupport.currentTransactionStatus().setRollbackOnly()`。

### 陷阱 3：非 public 方法

```java
@Transactional
private void method() {  // ❌ private 方法，事务失效
    // ...
}
```

**修复**：MUST 为 `public`。

### 陷阱 4：多线程

```java
@Transactional
public void method() {
    CompletableFuture.runAsync(() -> {
        // ❌ 异步线程内事务失效
    });
}
```

## 关联

- Wiki：`wiki/_common/distributed-transaction.md`
- 规则：`common-data`
- 技能：`coding` / `debug-issue`
