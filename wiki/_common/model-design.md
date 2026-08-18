# 模型设计规范

> 本文档是 `data-design` 技能的参考手册。
> 所有领域模型 / 数据模型设计 MUST 遵循本文档。

> 📌 **栈归属说明**：本文档的**示例代码**以 **structure-boot** 栈为主（含具体类名如 `ResResultVO` / `DataScopeRedisTemplate` / `structure-security` 等）。
> **规则部分**（MUST / MUST NOT）适用于所有技术栈。
> 其他栈的使用者请参考对应栈级 Wiki（`wiki/<stack>/developer.md`）获取具体类名与组件。

## 模型分层（MUST）

structure-projects 生态 MUST 区分**五层模型**：

| 层 | 命名 | 位置（DDD 7+1） | 职责 | 持久化注解 |
|---|---|---|---|---|
| **Entity** | `{X}Entity` | `domain` | 领域实体（业务视角） | ❌ 不含 |
| **PO** | `{X}PO` | `repository-mybatis` | 持久化对象（DB 视角） | ✅ 含 `@TableName` / `@TableId` / `@TableLogic` |
| **DTO** | `{X}DTO` | `common` | 服务间传输（Feign） | ❌ 不含 |
| **VO** | `{X}VO` | `common` | 返回给前端的视图 | ❌ 不含 |
| **Query** | `{X}Query` | `common` | 分页 / 条件查询 | ❌ 不含 |

**禁止**：
- ❌ **MUST NOT** 在 Entity 上加 `@TableId` / `@TableLogic`
- ❌ **MUST NOT** 在 Service / Controller 直接返回 PO
- ❌ **MUST NOT** 用 Entity 直接做 Controller 返回（应用 VO）
- ❌ **MUST NOT** 混用四层模型（如把 PO 当 DTO 用）

## 审计字段（MUST）

所有数据表 MUST 含以下审计字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | BIGINT | 主键（AUTO_INCREMENT 或雪花算法） |
| `tenant_id` | BIGINT | 多租户 ID |
| `create_time` | DATETIME | 创建时间 |
| `update_time` | DATETIME | 更新时间（ON UPDATE） |
| `is_deleted` | TINYINT | 逻辑删除（0 正常 / 1 已删） |

**对应 Java 字段**：

```java
// PO 层
@TableId(type = IdType.AUTO)
private Long id;
private Long tenantId;
private LocalDateTime createdAt;
private LocalDateTime updatedAt;
@TableLogic
private Integer isDeleted;
```

## 主键策略

| 策略 | 适用 | 说明 |
|---|---|---|
| **AUTO_INCREMENT** ⭐ | 默认，单库单表 | 简单、高效、有序 |
| **雪花算法** | 分库分表 / 分布式 | 全局唯一、趋势递增 |
| **UUID** | 仅特殊场景 | 无序、占用空间大，不推荐 |

**默认 MUST 用 `AUTO_INCREMENT`**，除非明确需要分库分表。

## 命名规范

### Entity / PO / DTO / VO / Query

| 层 | 模式 | 示例 |
|---|---|---|
| Entity | `{X}Entity` | `UserEntity` / `DeptEntity` |
| PO | `{X}PO` | `UserPO` / `DeptPO` |
| DTO | `{X}DTO` | `UserDTO` / `DeptDTO` |
| VO | `{X}VO` | `UserVO` / `DeptVO` |
| Query | `{X}Query` | `UserQuery` / `DeptQuery` |

### 数据表

- 表名 MUST `lower_snake_case`：`user` / `user_role` / `order_item`
- 字段名 MUST `lower_snake_case`：`create_time` / `tenant_id`

### 包名

```
cn.structured.{X}.domain.entity       # Entity
cn.structured.{X}.repository.po       # PO（注意：实际包是 cn.structured.{X}.repository.repository.po，双 repository 是历史遗留）
cn.structured.{X}.common.dto          # DTO
cn.structured.{X}.common.vo           # VO
cn.structured.{X}.common.query        # Query
```

## Entity / PO 转换（MUST 显式）

**MUST 在 `MybatisPlusDelegate` 子类显式实现 `toEntity` / `toPo`**：

```java
@Component
public class UserMybatisPlusDelegate
        extends `MybatisPlusRepositoryDelegate`<UserEntity, UserPO, Long>
        implements UserRepositoryDelegate {

    @Override
    protected UserEntity toEntity(UserPO po) {
        if (po == null) return null;
        UserEntity entity = new UserEntity();
        entity.setId(po.getId());
        entity.setUsername(po.getUsername());
        // ... 显式字段映射
        return entity;
    }

    @Override
    protected UserPO toPo(UserEntity entity) {
        if (entity == null) return null;
        UserPO po = new UserPO();
        po.setId(entity.getId());
        // ... 显式字段映射
        return po;
    }
}
```

**禁止**：
- ❌ **MUST NOT** 依赖 MapStruct 等自动转换（除非团队明确约定）
- ❌ **MUST NOT** 在 Service 层做 Entity ↔ PO 转换

## Lombok 使用规范

### Entity / PO

```java
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class UserEntity {
    // ...
}
```

### DTO / VO / Query

```java
@Data
public class UserDTO {
    // ...
}
```

**关键**：
- Entity / PO 用 `@Data + @Builder + @NoArgsConstructor + @AllArgsConstructor`
- DTO / VO / Query 只用 `@Data`（不需要 Builder）

## 关联模型设计

### 一对一

在**主表**加 `xxx_id` 字段。

### 一对多

在**子表**加 `parent_id` 字段。

### 多对多

**MUST** 建中间表：`{a}_{b}` 命名（如 `user_role`）。

## 禁止事项

- ❌ **MUST NOT** 在 Entity 之间直接建立 JPA 风格的关联（如 `@OneToMany`）
- ❌ **MUST NOT** 跨服务直接读数据库（应用 Feign 调用）
- ❌ **MUST NOT** 在 PO 中使用 `Date` 类型（应用 `LocalDateTime`）

## 关联

- 技能：`data-design` / `structure-boot-new-entity`
- Wiki：`wiki/_common/database-design.md` `wiki/_common/naming.md`
