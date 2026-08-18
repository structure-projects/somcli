---
name: quick-fix
description: |
  快速修复原因已明确的小问题（跳过深度分析）。
  当用户请求 /quick-fix 或"快速修复"时触发。
category: support
phase: support
triggers:
  - quick-fix
  - 快速修复
related-skills:
  - debug-issue
  - ci-gate
  - release-ops
related-rules:
  - common-observability
stack: _common
priority: medium
---

# 快速修复（Quick Fix）

## 目的

在**原因已明确**的前提下跳过深度分析，直接修复并走 `ci-gate` 快速通道提交。

## 使用场景

- 已知原因的 bug 修复
- 配置错误修正、文档 / 注释 / typo 修正
- 非生产环境的紧急修复

## 与 `/investigate` 的边界（MUST 先判定）

| 情形 | 用哪个 |
|---|---|
| 原因**已明确** + 方案清晰 + ≤ 5 文件且 ≤ 50 行 | **`/quick-fix`**（本指令） |
| 原因**不明**，需要复现定位 | `/investigate`（完整 8 步） |
| 生产环境热修复 | 两者都不用 → `release-ops` |

**MUST NOT** 在原因不明时使用本指令；修复过程中发现原因其实不明或范围超限 → **MUST** 转 `/investigate` 完整流程。

## 委派

→ 调用 `debug-issue` 的**快速通道**（5 步 + 回报格式见该技能"快速通道"节）。

**MUST NOT** 跳过 `ci-gate` 快速通道裸 `git commit`；门禁失败 MUST 转完整流程。

## 关联

- Skill：`_common/skills/debug-issue/SKILL.md`（快速通道）
- Skill：`_common/skills/ci-gate/SKILL.md`（提交门禁，MUST 经过）
- Skill：`_common/skills/release-ops/SKILL.md`（生产热修复改用此技能）
- Wiki：`_common/wiki/error-handling.md`
