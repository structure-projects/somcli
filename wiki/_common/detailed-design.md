# 详细设计（LLD）规范

> 本文档是详细设计的参考手册。`api-design` 技能引用本文档。

## LLD 的目标

- 把 HLD 的高层设计细化为可编码的详细方案
- 明确类结构、接口契约、数据模型、错误处理
- 为编码提供"图纸"

## LLD 包含内容（MUST）

### 1. 类图（C4 Level 3+4）

用 mermaid 画类图，展示类之间的关系。

### 2. 接口定义

REST API 契约：路径、方法、请求、响应、错误码。

### 3. 数据模型

- 领域模型（Entity）
- 持久化模型（PO）
- 传输模型（DTO / VO / Query）
- 数据库表结构（DDL）

### 4. 时序图

画关键业务流程的时序图。

### 5. 状态图（如涉及状态机）

画状态转换图。

### 6. 错误处理

| 场景 | 错误码 | 处理 |
|---|---|---|

### 7. 测试策略

- 单测覆盖范围
- 集成测试覆盖范围
- E2E 测试覆盖范围

### 8. 关键决策

记录设计中的关键决策（为什么选 A 不选 B）。

## 何时做 LLD

| 变更级别 | 是否做 LLD |
|---|---|
| **trivial**（typo / 格式） | ❌ 跳过 |
| **minor**（小功能调整） | ❌ 跳过或可选 |
| **major**（新功能 / 架构调整） | ✅ MUST |
| **hotfix**（紧急修复） | ❌ 跳过（事后补） |
| **migration**（迁移） | ✅ MUST |

## LLD 文档位置

`changes/proposals/<id>/design.md`

## LLD 与其他设计技能的关系

```
high-level-design（系统级）
   ↓
api-design（功能/模块级）
   ↓ 引用
   ├─ model-design（模型设计）
   ├─ api-design（接口设计）
   └─ database-design（数据库设计）
```

## 关联

- 技能：`api-design`
- 前置：`high-level-design`（新项目）或 `requirement-analysis`（历史项目）
- 后续：`coding`
- Wiki：`wiki/_common/architecture.md`
