---
name: investigate
description: |
  系统性调查问题：收集 → 复现 → 定位 → 修复 → 验证。
  当用户请求 /investigate 或"排查"时触发。
category: support
phase: debug
triggers:
  - investigate
  - 排查
related-skills:
  - debug-issue
related-rules:
  - common-observability
stack: _common
priority: high
---

# 问题调查（Investigate）

## 目的

系统性排查和修复**原因未明**的问题：收集信息 → 复现 → 定位根因 → 修复 → 验证。**禁止凭直觉猜测**。

## 使用场景

- 用户报告错误信息或异常行为，但原因不明
- 功能不工作或结果不符合预期
- 日志分析、性能问题定位

## 与 `/quick-fix` 的边界（MUST 先判定）

| 情形 | 用哪个 |
|---|---|
| 原因**不明**，需要复现定位 | **`/investigate`**（本指令，完整 8 步） |
| 原因**已明确** + 方案清晰 + ≤ 5 文件且 ≤ 50 行 | `/quick-fix`（快速通道） |
| 生产环境热修复 | 两者都不用 → `release-ops` |

调查中途发现原因已明确且改动很小 → MAY 转 `/quick-fix`；
快速修复中发现原因其实不明 → **MUST** 转回本指令。

## 委派

→ 调用 `debug-issue`，**8 步流程、按层次排查顺序、常见问题模式与诊断报告模板均见该技能**。

## 关联

- Skill：`_common/skills/debug-issue/SKILL.md`
- Wiki：`_common/wiki/error-handling.md`、`_common/wiki/logging.md`
- Rule：`_common/rules/common-observability.mdc`
