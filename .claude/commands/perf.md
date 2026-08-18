---
name: perf
description: |
  性能测试与调优建议。
  当用户请求 /perf 或"性能"时触发。
  ⚠️ 生产环境压测 MUST 用户确认（L0）。
category: review
phase: review
triggers:
  - perf
  - 性能
related-skills:
  - performance
related-rules:
  - common-runtime
stack: _common
priority: medium
---

# 性能分析（Perf）

## 目的

用压测建立基线 → 定位瓶颈 → 优化 → 复测验证。**先测量，后优化**。

## 使用场景

- 上线前性能基准测试 / 容量规划
- 性能问题排查
- 代码变更的性能回归测试

## 参数

| 参数 | 行为 |
|---|---|
| `--baseline` | 只压测建立基线，不做优化 |
| `--tune` | 已有基线，直接进入定位瓶颈 + 优化 + 复测 |
| 无参数 | 先建基线再调优 |

## 委派

→ 调用 `performance`，**工具选择、K6 脚本示例、关键指标阈值、瓶颈定位顺序、优化手段表与报告模板均见该技能**。

**MUST NOT** 在生产环境直接压测（L0，需用户确认）；
**MUST NOT** 通过放宽 thresholds 让压测"通过"。

## 关联

- Skill：`_common/skills/performance/SKILL.md`
- Wiki：`_common/wiki/performance.md`
- Rule：`_common/rules/common-runtime.mdc`
