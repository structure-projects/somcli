---
name: code-review
description: |
  对照变更提案评审代码，产出评审报告。
  当用户请求 /code-review 或"评审"时触发。
category: review
phase: review
triggers:
  - code-review
  - 评审
related-skills:
  - expert-review
related-rules:
  - common-core
  - common-security
stack: _common
priority: high
---

# 代码审查（Code Review）

## 目的

对照变更提案系统性评审代码质量，产出分级评审报告，确保代码符合规范且安全可靠。

## 使用场景

- 编码完成后进入评审阶段
- PR 提交前的自检 / CI 门禁前置检查
- 需要对代码质量进行全面评估

## 参数

| 参数 | 行为 |
|---|---|
| 无参数 | 评审 `git diff develop...HEAD` 全量变更 |
| `--staged` | 只评审已暂存的变更 |
| `--file=<path>` | 只评审指定文件 |

## 委派

→ 调用 `expert-review`，**前置条件、6 维度评审表、MUST fix / SHOULD fix / NIT 分级标准与报告模板均见该技能**。

**MUST fix 项不解决 MUST NOT 提交**。

⚠️ AI 自检 ≠ 专家评审，关键项目 MUST 引入人类评审。

## 关联

- Skill：`_common/skills/expert-review/SKILL.md`
- Wiki：`_common/wiki/code-review-checklist.md`、`_common/wiki/security.md`
- Rule：`_common/rules/common-security.mdc`
