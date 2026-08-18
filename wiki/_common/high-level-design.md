# 概要设计（HLD）规范

> 本文档是概要设计的参考手册。`high-level-design` 技能引用本文档。

## HLD 的目标

- 让团队在**编码前**对齐系统级设计
- 明确系统边界、技术选型、模块划分
- 识别关键风险并制定缓解措施

## HLD 包含内容（MUST）

### 1. 系统边界

- **业务价值**：本系统解决什么问题
- **目标**：要做什么
- **非目标**：明确不做什么
- **用户**：谁在用（内部 / 外部 / 第三方）
- **上下游**：依赖谁 / 被谁依赖

### 2. 系统上下文图（C4 Level 1）

用 mermaid 画系统与外部的关系。

### 3. 容器图（C4 Level 2）

把系统拆分为可独立部署的"容器"（前端 / 后端 / DB / MQ / Cache / Worker）。

### 4. 技术选型

MUST 按 `stack-constraints` 选择，禁止凭印象。

### 5. 数据流图

画关键业务场景的数据流（如登录、下单）。

### 6. 风险与缓解

至少识别 3 个风险，并给出缓解措施。

### 7. 模块划分（高层）

列出各模块职责（一句话）。

## C4 模型参考

- **Level 1：系统上下文**（System Context）→ 系统与外部的关系
- **Level 2：容器**（Container）→ 系统内的部署单元
- **Level 3：组件**（Component）→ 容器内的组件（LLD 阶段做）
- **Level 4：代码**（Code）→ 类图（LLD 阶段做）

**HLD 只覆盖 Level 1 + Level 2**。

## 何时做 HLD

| 场景 | 是否做 HLD |
|---|---|
| 全新项目 | ✅ MUST |
| 大版本重构（X 升级） | ✅ MUST |
| 架构演进（拆服务、技术栈升级） | ✅ MUST |
| 新功能（Y 升级） | ❌ 跳过，直接 LLD |
| 小修复（Z 升级） | ❌ 跳过 |

## HLD 文档位置

`changes/proposals/<id>/hld.md`

## 关联

- 技能：`high-level-design`
- 后续：`api-design`（LLD）
- Wiki：`wiki/_common/architecture.md`
