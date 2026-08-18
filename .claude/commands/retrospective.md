---
name: retrospective
description: |
  复盘总结：事故复盘（hotfix / rollback 后）+ 迭代复盘（版本发布后）。
  当用户请求 /retrospective 或"复盘"时触发。
category: ops
phase: retrospective
triggers:
  - retrospective
  - 复盘
related-skills:
  - release-ops
related-rules:
  - common-delivery
stack: _common
priority: low
---

# 复盘总结（Retrospective）

## 目的

结构化复盘，总结经验教训、识别改进点、形成持续改进闭环。

## 使用场景

- hotfix / rollback 后的事故复盘（**24h 内 MUST 产出**）
- 版本发布或迭代结束的总结
- 团队流程改进

## 参数

| 参数 | 复盘类型 | 模板部分 |
|---|---|---|
| `--incident` | 事故复盘（根因 / 时间线 / 为什么没测出来） | 模板 A 部分 |
| `--iteration` | 迭代复盘（做得好的 / 可改进的 / 行动项） | 模板 B 部分 |
| 无参数 | 由 AI 按上下文判定（L1） | 见上 |

## 委派

→ 模板与两类复盘的完整字段见 `_common/changes/templates/retrospective.md`，写入 `changes/proposals/<id>/retrospective.md`。

hotfix 场景由 `release-ops` 第 6 步驱动，回滚场景由 `release-ops` 4.1 驱动，本指令用于独立发起或补写。

**MUST** 有数据支撑（commit / 归档提案 / 监控指标），**MUST NOT** 只有分析没有行动项。

> 注：`retro-document` 是 project-intake 的**反向文档化**子技能（补 C4 图 / ADR），与本指令无关，MUST NOT 混用。

## 关联

- 模板：`_common/changes/templates/retrospective.md`
- Skill：`_common/skills/release-ops/SKILL.md`、`_common/skills/release-ops/SKILL.md`
- Wiki：`_common/wiki/documentation.md`
