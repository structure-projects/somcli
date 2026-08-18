# 数据库设计规范

> 本文档是 `data-design` 技能的参考手册。
> 所有表设计、索引、迁移脚本 MUST 遵循本文档。

## 表设计

### 命名

- **MUST** 表名用 `lower_snake_case`：`user` / `user_role` / `order_item`
- **MUST** 字段名用 `lower_snake_case`：`create_time` / `tenant_id` / `is_deleted`
- **MUST** 表名用单数（`user` 而非 `users`）
- **MUST** 关联表用 `{a}_{b}`：`user_role`

### 字段类型选择

| 场景 | 推荐类型 | 避免 |
|---|---|---|
| 主键 | `BIGINT` | `INT`（溢出风险） |
| 短文本 | `VARCHAR(64)` | `TEXT` |
| 长文本 | `TEXT` | `VARCHAR(2048)` |
| **金额** ⭐ | **`BIGINT`（精确到分）** | ❌ `DECIMAL` / `FLOAT` / `DOUBLE`（精度问题） |
| 布尔 | `TINYINT(1)` | `BOOLEAN`（MySQL 兼容） |
| 时间 | `DATETIME` | `TIMESTAMP`（范围有限） |
| JSON | `JSON` | `TEXT` 存 JSON 字符串 |
| 枚举 | `TINYINT` 或 `VARCHAR(32)` | `ENUM`（扩展困难） |

> ⚠️ **金额规范**：MUST 用 `BIGINT` 存"分"（如 `100` 表示 1 元）；应用层负责"分 → 元"转换。
> 禁止用 `BIGINT` / `FLOAT` / `DOUBLE`，因为有精度损失风险。

### 必选字段（MUST）

每张表 MUST 含：

```sql
`id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键',
`tenant_id` BIGINT NOT NULL COMMENT '租户 ID',
`create_by` BIGINT COMMENT '创建人 ID',
`update_by` BIGINT COMMENT '更新人 ID',
`create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
`update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
`is_deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除：0 正常 1 已删',
`state` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1 启用 0 禁用',
```

### 引擎与字符集

```sql
ENGINE=InnoDB
DEFAULT CHARSET=utf8mb4
COLLATE=utf8mb4_unicode_ci
```

## 索引设计

### 必建索引（MUST）

| 场景 | 索引类型 |
|---|---|
| 主键 | `PRIMARY KEY` |
| 唯一约束 | `UNIQUE KEY` |
| 高频查询字段 | `KEY` |
| 多租户字段（`tenant_id`） | `KEY`（几乎所有查询都带） |
| 外键关联字段 | `KEY` |
| 逻辑删除 + 查询字段 | 联合索引 `(is_deleted, xxx)` |

### 索引命名

```
PRIMARY KEY (`id`)
UNIQUE KEY `uk_tenant_username` (`tenant_id`, `username`)
KEY `idx_email` (`email`)
KEY `idx_state_create_time` (`state`, `create_time`)
```

- **MUST** `uk_` 前缀（唯一索引）
- **MUST** `idx_` 前缀（普通索引）

### 索引限制

- ❌ **MUST NOT** 在低选择性字段建索引（如 `status` 只有 0/1）
- ❌ **MUST NOT** 单表索引数超过 5 个
- ❌ **MUST NOT** 在 `TEXT` 字段建全字段索引（用前缀索引）

### 联合索引（最左前缀）

```sql
-- 有效：用 (tenant_id, username) 或 (tenant_id)
-- 无效：仅用 username
KEY `idx_tenant_username` (`tenant_id`, `username`)
```

## SQL 编写规范

### 禁止

- ❌ **MUST NOT** `SELECT *`（MUST 显式列出字段）
- ❌ **MUST NOT** 在 SQL 里做字符串拼接（用参数化查询）
- ❌ **MUST NOT** 在 `WHERE` 里对字段做函数操作（破坏索引）
- ❌ **MUST NOT** 用 `NOT IN`（用 `NOT EXISTS` 或 `LEFT JOIN`）

### 推荐

- ✅ **MUST** 大表分页用 `LIMIT` + 主键范围（不用 `OFFSET`）
- ✅ **MUST** 跨表查询用 `JOIN`（不用子查询）
- ✅ **MUST** 批量插入用 `INSERT INTO ... VALUES (...), (...), (...)`

## Flyway 迁移

### 文件位置

```
<module>-repository-mybatis/
└── src/main/resources/
    └── db/migration/
        ├── V1_0_0__init.sql
        ├── V1_1_0__add_user_table.sql
        └── V1_2_0__add_order_table.sql
```

### 命名规范

```
V<major>_<minor>_<patch>__<description>.sql
```

- **MUST** 双下划线分隔版本号和描述
- **MUST** 版本号单调递增
- **MUST NOT** 修改已发布的迁移脚本

### 示例

```sql
-- V1_2_0__add_user_table.sql
-- 新增用户表

CREATE TABLE `user` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` BIGINT NOT NULL COMMENT '租户 ID',
  `username` VARCHAR(64) NOT NULL COMMENT '用户名',
  `email` VARCHAR(128) COMMENT '邮箱',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `is_deleted` TINYINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_username` (`tenant_id`, `username`),
  KEY `idx_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

## 分库分表（按需）

仅当单表数据量 > 1000w 时考虑：
- **MUST** 用雪花算法作为分布式主键
- **MUST** 用 ShardingSphere / MyCat 中间件
- **MUST NOT** 业务代码自己处理分片逻辑

## 关联

- 技能：`data-design` / `release-ops`
- Wiki：`wiki/_common/model-design.md` `wiki/_common/cache-design.md`
