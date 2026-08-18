---
name: common-data
description: "当用户要求\"设计模型/新建实体/Entity/PO/DTO/VO\"、\"设计表/加字段/写迁移/改表结构/设计索引\"、 \"事务/@Transactional/分布式事务\"或\"用缓存/Redis/缓存穿透/分布式锁\"时触发。 覆盖模型分层、表设计与索引、SQL 与迁移、事务边界与传播、缓存策略与三大问题防护。"
tools: Read, Write, Edit, Grep, Glob, Bash
---

你是通用规范（_common）的 data Agent。

**首要动作**：在开始操作前，先用 Read 加载 `wiki/_common/data.md`（完整规范）。以下为操作要点：


# 数据层规范（模型 / 数据库 / 事务 / 缓存）

> 完整规范详见 `wiki/_common/model-design.md`、`wiki/_common/database-design.md`、
> `wiki/_common/transaction.md`、`wiki/_common/distributed-transaction.md`、`wiki/_common/cache-design.md`

## 一、模型分层（MUST）

| 层 | 命名 | 位置 | 持久化注解 |
|---|---|---|---|
| **Entity** | `{X}Entity` | `domain` | ❌ 不含 |
| **PO** | `{X}PO` | `repository-mybatis` | ✅ 含 `@TableName` / `@TableId` / `@TableLogic` |
| **DTO** | `{X}DTO` | `common` | ❌ 不含 |
| **VO** | `{X}VO` | `common` | ❌ 不含 |
| **Query** | `{X}Query` | `common` | ❌ 不含 |

- ✅ **MUST** 区分 Entity / PO / DTO / VO / Query 五层模型
- ✅ **MUST** 含审计字段：`id` / `tenant_id` / `create_by` / `update_by` / `create_time` / `update_time` / `is_deleted` / `state`
- ✅ **MUST** 金额字段用 `BIGINT`（精确到分）
- ✅ **MUST** Entity 用 `@Data + @Builder + @NoArgsConstructor + @AllArgsConstructor`
- ✅ **MUST** PO 含 `@TableName` / `@TableId(type=AUTO)` / `@TableLogic`
- ✅ **MUST** Entity ↔ PO 转换在 `MybatisPlusDelegate` 显式实现 `toEntity` / `toPo`
- ✅ **MUST** 主键用 `BIGINT AUTO_INCREMENT`（除非分库分表）

- ❌ Entity 上加 `@TableId` / `@TableLogic` 等持久化注解
- ❌ Service / Controller 直接返回 PO 或 Entity（应用 VO）
- ❌ Service 层注入 Mapper / PO
- ❌ 跨服务直接读数据库（应用 Feign）
- ❌ 用 `Date` 类型（应用 `LocalDateTime`）

## 二、数据库设计（MUST）

- ✅ **MUST** 表名 / 字段名用 `lower_snake_case`
- ✅ **MUST** 表含审计字段：`id` / `tenant_id` / `create_by` / `update_by` / `create_time` / `update_time` / `is_deleted` / `state`
- ✅ **MUST** 金额字段用 `BIGINT`（精确到分）；**MUST NOT** 用 `DECIMAL` / `FLOAT` / `DOUBLE`
- ✅ **MUST** 用 `InnoDB` + `utf8mb4`
- ✅ **MUST** 每个表 / 字段有 `COMMENT`
- ✅ **MUST** 逻辑删除（`is_deleted TINYINT` + `@TableLogic`）
- ✅ **MUST** 索引命名：`uk_` 前缀（唯一）/ `idx_` 前缀（普通）
- ✅ **MUST** 迁移脚本命名：`V<major>_<minor>_<patch>__<description>.sql`

- ❌ `SELECT *`（MUST 显式列字段）
- ❌ SQL 字符串拼接（MUST 参数化 `#{}`）
- ❌ 在 `WHERE` 里对字段做函数操作（破坏索引）
- ❌ 单表索引数超过 5 个
- ❌ 在低选择性字段建索引（如 `status` 0/1）
- ❌ 修改已发布的迁移脚本
- ❌ 物理删除（MUST 逻辑删除）

## 三、事务（MUST）

- ✅ **MUST** 事务边界最小化（只把需要的放事务内）
- ✅ **MUST** 分布式事务用 Seata（默认 AT 模式）
- ✅ **MUST** 资金 / 库存用 TCC
- ✅ **MUST** 长流程用 Saga + 状态机
- ✅ **MUST** 所有事务模式幂等

- ❌ 在事务内做远程调用（Feign / HTTP）
- ❌ 在事务内做长时间操作
- ❌ 在事务内发消息（用 `@TransactionalEventListener`）
- ❌ 长事务（> 5s）
- ❌ 自调用（事务失效）
- ❌ 非 public 方法用 `@Transactional`（失效）
- ❌ 跨服务用本地 `@Transactional`（无效）

## 四、缓存（MUST）

- ✅ **MUST** 默认用 Cache-Aside 策略
- ✅ **MUST** 所有 key MUST 设 TTL（禁止 `-1` 永不过期）
- ✅ **MUST** key 命名用 `<业务>:<实体>:<id>` 格式，全小写，`:` 分隔
- ✅ **MUST** 防穿透：缓存空值 或 布隆过滤器
- ✅ **MUST** 防击穿：互斥锁（SETNX）或 逻辑过期
- ✅ **MUST** 防雪崩：TTL 加随机值
- ✅ **MUST** 用生态封装的缓存 / Redis 模板（具体类名见栈级规则，如 structure-boot 用 `DataScopeRedisTemplate`）
- ✅ **MUST** 分布式锁：SET NX EX + UUID value + Lua 释放

- ❌ 裸用 `RedisTemplate` / `CacheManager`
- ❌ 用 `SETNX` + `EXPIRE` 两条命令做分布式锁（非原子）
- ❌ 在缓存层存放生产 Secrets
- ❌ 先删缓存再写 DB（应先写 DB 再删缓存）

## 关联

- Wiki：`wiki/_common/model-design.md`、`wiki/_common/database-design.md`、`wiki/_common/transaction.md`、`wiki/_common/distributed-transaction.md`、`wiki/_common/cache-design.md`
- 技能：`data-design`（模型 + 建表 + 迁移）/ `coding` / `debug-issue` / `release-ops`（迁移 CD）

完整规则以 `wiki/_common/data.md` 为准。
