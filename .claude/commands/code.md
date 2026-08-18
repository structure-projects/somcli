---
name: code
description: |
  按变更提案逐项实现代码。
  当用户请求 /code 或"编码"时触发。
category: coding
phase: coding
triggers:
  - code
  - 编码
related-skills:
  - coding
  - create-feature
related-rules:
  - common-core
  - common-project-structure
  - common-project-structure
stack: _common
priority: high
---

# 编码实现（Code）

## 目的

按变更提案和任务清单逐项实现代码，确保每一项任务都有对应代码和测试。

## 使用场景

- 变更提案已存在，准备开始编码
- 按 tasks.md 逐项实现功能
- trivial / minor 变更跳过设计直接进入本指令

## 参数

| 参数 | 行为 |
|---|---|
| 无参数 | 按 tasks.md 从第一个未完成任务开始 |
| `--task=<n>` | 只实现指定编号的任务 |

## 委派

→ 调用 `coding`，**前置条件、5 步流程、通过标准、自评清单与进度汇报格式均见该技能**。
新增完整功能（含 Controller/Service/Entity 一整套）时转 `create-feature`。

**MUST NOT** 无提案直接编码；**MUST NOT** 只写实现跳过单测。

## 关联

- Skill：`_common/skills/coding/SKILL.md`、`_common/skills/create-feature/SKILL.md`
- Wiki：`_common/wiki/naming.md`、`_common/wiki/architecture.md`
- Rule：`_common/rules/common-core.mdc`
