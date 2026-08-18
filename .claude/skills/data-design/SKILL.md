---
name: data-design
description: |
  当用户要求"设计模型/设计实体/设计 DTO/设计 VO"、"设计表/建表/加字段"或"写迁移/改表结构/设计索引"时触发。
  数据设计全流程：领域模型四层设计（Entity / PO / DTO / VO / Query）→ 表结构与索引 → Flyway 迁移脚本 → 数据字典。
  MUST 区分四层模型；表 MUST 含审计字段与逻辑删除；表名 / 字段名 / 索引 MUST 与用户或 DBA 确认。

triggers:
  - 设计模型
  - 领域模型
  - 设计实体
  - 实体设计
  - 设计 DTO
  - 设计 VO
  - model
  - entity
  - DTO
  - VO
  - 设计表
  - 建表
  - 设计用户表
  - 加字段
  - 写迁移
  - 改表结构
  - 设计索引
  - table
  - schema
  - migration
  - 索引
  - index
  - DDL

role: architect
phase: design
supports-skill: coding

allowed-tools: Bash, Read, Write, Edit

related-rules:
  - common-core
  - common-data
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/model-design.md
  - wiki/_common/database-design.md
  - wiki/<stack>/developer.md
  - wiki/<stack>/components.md
  - wiki/<stack>/orm-design.md

stack-constraints:
  structure-boot:
    entity-suffix: "Entity"      # {X}Entity
    po-suffix: "PO"              # {X}PO
    dto-suffix: "DTO"            # {X}DTO
    vo-suffix: "VO"              # {X}VO
    query-suffix: "Query"        # {X}Query
    migration-tool: "Flyway"
    migration-path: "db/migration/"
    migration-naming: "V{version}__{description}.sql"
    required-fields:
      - "id BIGINT PRIMARY KEY AUTO_INCREMENT（@TableId）"
      - "create_time DATETIME"
      - "update_time DATETIME ON UPDATE"
      - "deleted TINYINT DEFAULT 0（@TableLogic）"
      - "tenant_id BIGINT（多租户）"
    forbidden:
      - "Entity 直接用 @TableId（应用 PO）"  # DDD 形态
      - "在 Service 注入 Mapper / PO"
      - "物理删除（MUST 逻辑删除 @TableLogic）"
      - "SELECT *"
      - "跨服务直接读库"

produces:
  - 领域模型设计文档（融入 proposal 或单独 model.md）
  - Entity / PO / DTO / VO / Query 类骨架
  - 数据表 DDL + 索引设计 + 数据字典
  - Flyway 迁移脚本（db/migration/V<x>__<name>.sql）

requires:
  - skill: requirement-analysis
    condition: changes/proposals/<current>/proposal.md exists
    error: 无变更提案，MUST 先调用 requirement-analysis

human-in-the-loop:
  - id: confirm-model-boundary
    action: 模型边界（哪些字段属于本模型、关联关系）MUST 与用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 边界判断错误只影响未落库的设计，可重写
  - id: confirm-table-naming
    action: 表名 / 字段名 MUST 与 DBA 或用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 迁移脚本未执行前可改；已执行需新写一条迁移
  - id: confirm-index-design
    action: 索引设计 MUST 评估数据量后确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 索引可增删，但大表加索引需评估锁表窗口

on-failure: |
  模型边界不清 → 回到 requirement-analysis 澄清
  数据库设计冲突 → 与 DBA 确认
  迁移脚本版本冲突 → 与用户确认版本号，MUST NOT 改已发布脚本
  性能影响大 → 提供离线迁移方案

category: model-design
stack: _common
priority: high
---

# 数据设计（模型 + 数据库）

> 四层模型（Entity / PO / DTO / VO / Query）→ 表结构与索引 → 迁移脚本。
> **MUST 区分四层模型，MUST NOT 混用**；表 MUST 含审计字段与逻辑删除。

## 前置条件

- 变更提案存在（`changes/proposals/<current>/proposal.md`）
- 已识别项目栈（DDD / 单体）

---

# 一、模型设计

## 第 1 步：识别模型边界

**MUST 与用户确认**：
- 这个模型属于哪个 bounded context？
- 哪些字段属于本模型，哪些属于关联模型？
- 是一对一 / 一对多 / 多对多关系？

## 第 2 步：设计四层模型

| 层 | 位置 | 命名 | 说明 |
|---|---|---|---|
| **Entity**（领域实体） | `{X}-domain` | `{X}Entity` | 业务领域模型，不含持久化注解 |
| **PO**（持久化对象） | `{X}-repository-mybatis` | `{X}PO` | 表映射，含 `@TableName` / `@TableId` / `@TableLogic` |
| **DTO**（数据传输） | `{X}-common` | `{X}DTO` | 服务间传输（Feign 调用） |
| **VO**（视图对象） | `{X}-common` | `{X}VO` | 返回给前端的视图 |
| **Query**（查询对象） | `{X}-common` | `{X}Query` | 分页 / 条件查询 |

## 第 3 步：生成代码骨架

按 `wiki/<stack>/developer.md` 中的代码模板生成。

---

# 二、数据库设计

## 第 1 步：设计表结构

```sql
CREATE TABLE `user` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` BIGINT NOT NULL COMMENT '租户 ID',
  `username` VARCHAR(64) NOT NULL COMMENT '用户名',
  `password` VARCHAR(128) NOT NULL COMMENT '密码（加密存储）',
  `email` VARCHAR(128) COMMENT '邮箱',
  `mobile` VARCHAR(32) COMMENT '手机号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1 启用 0 禁用',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `is_deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除：0 正常 1 已删',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_username` (`tenant_id`, `username`),
  KEY `idx_email` (`email`),
  KEY `idx_mobile` (`mobile`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

## 第 2 步：核验红线（不在此重述级别）

建表红线由 `common-data` 规则声明（命名 / 审计字段 / 金额 `BIGINT` / `InnoDB` + `utf8mb4` / `COMMENT` / 逻辑删除 / 索引前缀 / 迁移脚本命名），完整规范见 `wiki/_common/database-design.md`。该规则与本技能触发词相同，执行时必然同时加载。

本步骤只做**逐条核验**，**MUST NOT** 在此复制约束正文或强制级别 —— 两份副本各自漂移后，AI 按技能执行、人按 wiki 理解，就会出现两边都没写错但行为不一致。

## 第 3 步：设计索引

**MUST 索引**：
- 主键（PRIMARY KEY）
- 唯一约束（UNIQUE KEY）
- 高频查询字段（KEY）
- 外键关联字段（KEY）
- 多租户字段（tenant_id，几乎所有查询都带）

- ❌ **MUST NOT** 在低选择性字段建索引（如 status 只有 0/1）
- ❌ **MUST NOT** 单表超过 5 个索引（影响写入性能）

## 第 4 步：生成 Flyway 迁移脚本

位置：`<stack>-repository-mybatis/src/main/resources/db/migration/`
命名：`V{version}__{description}.sql`，例 `V1_2_0__add_user_table.sql`

```sql
-- V1_2_0__add_user_table.sql
-- 新增用户表

CREATE TABLE `user` (
  -- ... 上述 DDL
);
```

## 第 5 步：数据字典

写入 `design.md`：

```markdown
## 数据字典

### user 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | BIGINT | 主键 |
| username | VARCHAR(64) | 用户名（租户内唯一） |
| ...
```

---

## 完成标准

- 模型边界经用户确认，四层模型（Entity/PO/DTO/VO/Query）齐全，命名符合规范
- 表结构含所有审计字段，索引设计合理
- 迁移脚本可执行（在测试库验证）

## 关联

- 前置：`requirement-analysis` / `high-level-design`（拆分场景）
- 后续：`coding`（按模型编码）/ `release-ops`（迁移 CD）
- Wiki：`wiki/_common/model-design.md`、`wiki/_common/database-design.md`
