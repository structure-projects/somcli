---
name: design
description: |
  设计层统一入口：概要设计（HLD）+ 详细设计（LLD）。
  当用户请求 /design 或 /design-plan 时触发。
category: design
phase: design
triggers:
  - design
  - design-plan
related-skills:
  - high-level-design
  - api-design
  - data-design
related-rules:
  - common-project-structure
  - common-project-structure
  - common-core
stack: _common
priority: high
---

# 设计（Design）

> 原 `/design-plan` 已并入本指令（`related-skills` 与 `phase` 重叠，HLD/LLD 边界在原设计中已失效），`design-plan` 保留为别名。

## 目的

把需求提案转化为可编码的设计：新项目 / 大版本重构先定系统边界与技术选型（HLD），功能级变更定 API 契约、类图、数据模型、时序图与错误处理（LLD）。

## 使用场景

- 新项目立项、架构演进、技术选型 → HLD
- 功能级变更（含模型 / 流程）、新增或调整接口 → LLD
- trivial / minor 变更（typo、配置）→ 跳过设计，直接 `/code`

## 参数

| 参数 | 粒度 | 委派技能 | 产出 |
|---|---|---|---|
| `--hld` | 概要设计 | `high-level-design` | `changes/proposals/<id>/hld.md`（系统边界 / 容器图 / 技术选型 / 风险评估） |
| `--lld` | 详细设计 | `api-design` | `changes/proposals/<id>/design.md`（类图 / 接口 / 数据模型 / 时序图 / 错误处理 / 测试策略） |
| 无参数 | 由 AI 按变更规模判定（L1） | 见上 | 见上 |

## 委派

→ `--hld` 调用 `high-level-design`；`--lld` 调用 `api-design`（数据模型部分转 `data-design`）。
**各步骤、图表模板与文档骨架均见对应技能**。

新项目 MUST 先 HLD 再 LLD；历史项目功能级变更可直接 LLD。

## 关联

- Skill：`_common/skills/high-level-design/SKILL.md`（HLD）
- Skill：`_common/skills/api-design/SKILL.md`（LLD + API 契约 + OpenAPI）
- Skill：`_common/skills/data-design/SKILL.md`、`_common/skills/data-design/SKILL.md`
- Wiki：`_common/wiki/high-level-design.md`、`_common/wiki/detailed-design.md`、`_common/wiki/api-design.md`
- Rule：`_common/rules/common-project-structure.mdc`
