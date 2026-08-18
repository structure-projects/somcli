---
name: require
description: |
  SDLC 入口：需求澄清 + 变更级别判定 + 产出变更提案。
  当用户请求 /require 或 /proposal 时触发。
category: requirement
phase: requirement
triggers:
  - require
  - proposal
related-skills:
  - requirement-analysis
related-rules:
  - common-core
  - common-project-structure
stack: _common
priority: high
---

# 需求分析（Require）

> 原 `/proposal` 已并入本指令（二者 `category` / `phase` / `related-skills` / `related-rules` 完全相同，
> 且各自重复同一张变更级别表），`proposal` 保留为别名，对应 `--quick` 行为。

## 目的

作为 SDLC 入口，将用户需求转化为结构化的变更提案，确保后续开发有明确的目标和路径。

## 使用场景

- 用户提出新需求或新功能
- 需要对现有功能进行较大调整
- 架构变更或技术栈升级
- 新项目从零开始

## 参数

| 参数 | 行为 |
|---|---|
| 无参数 | 完整流程：澄清需求 → 判定级别 → 影响分析 → 技术方案 → 产出提案 |
| `--quick` | 跳过需求澄清，已明确级别时直接生成 proposal.md + tasks.md 并建分支（原 `/proposal`） |
| `--level=<级别>` | 指定 trivial / minor / major / hotfix / migration，跳过级别询问 |

## 委派

→ 调用 `requirement-analysis`，**澄清清单、变更级别判定表、提案 ID 规则、模板复制与建分支步骤均见该技能**。

产出后按变更规模进入 `/design`（功能级）或直接 `/code`（trivial / minor）。

**MUST NOT** 在级别不确定时默认选择——MUST 询问用户。

## 关联

- Skill：`_common/skills/requirement-analysis/SKILL.md`（5 种变更级别 + 提案产出）
- Skill：`_common/skills/high-level-design/SKILL.md`（新项目）
- Skill：`_common/skills/api-design/SKILL.md`（复杂需求）
- Wiki：`_common/wiki/requirement-analysis.md`、`_common/wiki/architecture.md`、`_common/wiki/project-structure.md`
- Rule：`_common/rules/common-core.mdc`
