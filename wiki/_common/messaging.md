# 消息队列规范

> 本文档是消息队列使用的参考手册。
> 所有 MQ 使用 MUST 遵循本文档。

> 📌 **栈归属说明**：本文档的**示例代码**以 **structure-boot** 栈为主（含具体类名如 `ResResultVO` / `DataScopeRedisTemplate` / `structure-security` 等）。
> **规则部分**（MUST / MUST NOT）适用于所有技术栈。
> 其他栈的使用者请参考对应栈级 Wiki（`wiki/<stack>/developer.md`）获取具体类名与组件。

## 生态标准

- **MUST** 跨服务消息经 `DataScopeStreamBridge`（数据权限包装）
- **MUST** 用 `EventManager.publish(event)` 发布事件
- **MUST** 事件实现 `cn.structure.infra.event.Event` 接口

## 消费模式

### Spring 事件（本 JVM）

```java
@EventListener
public void handleUserCreated(UserCreatedEvent event) {
    // 本 JVM 内处理
}

@TransactionalEventListener  // 事务提交后才触发
public void handleUserCreated(UserCreatedEvent event) {
    // ...
}
```

### Binding 模型（跨服务推荐）

```java
@Bean
public Consumer<Message<UserEvent>> userEventConsumer() {
    return message -> dispatch(message);  // Consumer 内只 dispatch
}

@Bean
@StreamEventListener(bindingName = "userEventConsumer")
public void handleUserCreated(UserEvent event) {
    // 业务处理
}

// 多状态用 condition SpEL
@Bean
@StreamEventListener(bindingName = "userEventConsumer", condition = "#event.status == 'ACTIVE'")
public void handleUserActive(UserEvent event) {
    // 仅处理 ACTIVE 状态
}
```

### Router 模型（复杂路由）

```java
@StreamRouteHandler(eventType = "user", businessType = "create", condition = "#event.vip == true")
public void handleVipUserCreated(UserEvent event, `StreamEvent`<UserEvent> streamEvent) {
    // 处理 VIP 用户
}
```

## 消息幂等（MUST）

**所有消费端 MUST 幂等**：

```java
public void handleEvent(UserEvent event) {
    String idempotencyKey = "event:" + event.getEventId();
    Boolean first = redisTemplate.opsForValue()
        .setIfAbsent(idempotencyKey, "1", 24, TimeUnit.HOURS);
    if (Boolean.FALSE.equals(first)) {
        log.info("重复事件，跳过: {}", event.getEventId());
        return;
    }
    // 业务处理
}
```

## 死信队列（DLQ）

**MUST 配置**：

```yaml
spring:
  cloud:
    stream:
      bindings:
        userEventConsumer-in-0:
          consumer:
            max-attempts: 3
            back-off-initial-interval: 1000
            back-off-multiplier: 2
          dead-letter-queue: user-events-dlq
```

**MUST 监听 DLQ 并告警**：

```java
@StreamListener("user-events-dlq")
public void handleDeadLetter(Message<UserEvent> message) {
    alertService.send("DLQ 消息: " + message);
}
```

## 延迟消息

```java
// RocketMQ 延迟级别
Message<UserEvent> message = MessageBuilder.withPayload(event)
    .setHeader("delayTimeLevel", 3)  // 10s
    .build();
streamBridge.send("userEvent-out-0", message);
```

## 事务消息

```java
// 本地事务 + 消息最终一致性
@Transactional
public void createOrder(OrderDTO dto) {
    Order order = orderRepository.save(...);
    // 事务消息：DB 提交后才发消息
    streamBridge.send("orderCreated-out-0", OrderCreatedEvent.of(order));
}
```

## 关键约束（MUST）

- ✅ **MUST** 消费端幂等
- ✅ **MUST** 配置 DLQ + 告警
- ✅ **MUST** 跨服务经 `DataScopeStreamBridge`
- ✅ **MUST** 用 `EventManager.publish`（禁止直接 streamBridge.send）
- ❌ **MUST NOT** 在 Consumer 里写业务逻辑（应 dispatch 给 handler）
- ❌ **MUST NOT** 跳过幂等设计

## 关联

- Wiki：`wiki/_common/error-handling.md` `wiki/_common/cache-design.md`
- 规则：`common-runtime`
- 技能：`coding` / `debug-issue`
