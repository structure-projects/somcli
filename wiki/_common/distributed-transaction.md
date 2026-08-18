# 分布式事务规范

> 本文档是分布式事务的参考手册。

## 生态标准

**MUST 用 Spring Cloud Alibaba Seata** 处理分布式事务。

## 事务模式选择

| 模式 | 适用 | 一致性 | 性能 | 推荐度 |
|---|---|---|---|---|
| **AT** ⭐ | 大多数场景 | 最终一致 | 高 | **默认** |
| **TCC** | 资金 / 库存等强一致 | 强一致 | 中 | 资金场景 |
| **Saga** | 长事务 / 跨多服务 | 最终一致 | 高 | 长流程 |
| **XA** | 强一致 | 强一致 | 低 | 不推荐 |

## AT 模式（默认）

### 使用

```java
@GlobalTransactional
public void createOrder(OrderDTO dto) {
    orderRepository.save(...);          // 本服务
    inventoryFeign.deduct(...);         // 库存服务
    accountFeign.deduct(...);           // 账户服务
}
```

**关键**：
- ✅ **MUST** 在全局事务发起方加 `@GlobalTransactional`
- ✅ **MUST** 分支事务用 `@Transactional`
- ✅ **MUST** 各服务有 `undo_log` 表

### 适用

- 跨服务 DB 操作
- 可接受最终一致

## TCC 模式（资金 / 库存）

### 定义

```java
public interface AccountTccAction {
    @TwoPhaseBusinessAction(name = "deductTcc")
    boolean prepare(BusinessActionContext context, @BusinessActionContextParameter Long userId, @BusinessActionContextParameter BigDecimal amount);
    
    boolean commit(BusinessActionContext context);
    
    boolean rollback(BusinessActionContext context);
}
```

### 实现

```java
@Service
public class AccountTccActionImpl implements AccountTccAction {
    
    @Override
    public boolean prepare(BusinessActionContext context, Long userId, BigDecimal amount) {
        // Try：冻结金额
        return accountService.freeze(userId, amount);
    }
    
    @Override
    public boolean commit(BusinessActionContext context) {
        // Confirm：扣减冻结金额
        return accountService.confirmDeduct(...);
    }
    
    @Override
    public boolean rollback(BusinessActionContext context) {
        // Cancel：解冻金额
        return accountService.unfreeze(...);
    }
}
```

**关键**：
- ✅ **MUST** 实现 Try / Confirm / Cancel 三个方法
- ✅ **MUST** 保证幂等
- ✅ **MUST** 防空转（rollback 时检查是否执行过 prepare）
- ✅ **MUST** 防悬挂（prepare 时检查是否已 rollback）

## Saga 模式（长事务）

### 适用

- 跨多服务的长流程（订单履约、物流跟踪）

### 实现

用状态机管理：

```java
public enum OrderSagaState {
    CREATED → PAID → INVENTORY_DEDUCTED → SHIPPED → COMPLETED
              ↓         ↓                    ↓
           CANCELLED  REFUNDED         INVENTORY_RESTORED
}
```

每个状态对应一个补偿动作。

## 关键约束（MUST）

- ✅ **MUST** 默认用 AT 模式
- ✅ **MUST** 资金 / 库存用 TCC
- ✅ **MUST** 长流程用 Saga + 状态机
- ✅ **MUST** 所有模式 MUST 幂等
- ✅ **MUST** 记录事务日志（用于排查）
- ❌ **MUST NOT** 跨服务用本地 `@Transactional`（无效）

## 关联

- Wiki：`wiki/_common/transaction.md` `wiki/_common/messaging.md`
- 规则：`common-data`
- 技能：`coding` / `debug-issue`
