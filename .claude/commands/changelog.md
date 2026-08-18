---
name: changelog
description: |
  生成版本变更日志。
  当用户请求 /changelog 或"变更日志"时触发。
category: ops
phase: release
triggers:
  - changelog
  - 变更日志
related-skills:
  - git-ops
related-rules:
  - common-delivery
stack: _common
priority: medium
---

# 生成变更日志（Changelog）

## 目的

基于已归档提案与 commit 历史，生成符合 Keep a Changelog 规范的版本变更日志。

## 使用场景

- 版本发布前生成 changelog
- 汇总一段时期内的变更
- 合规性 / 发布说明文档

## 参数

| 参数 | 行为 |
|---|---|
| 无参数 | 自上次 tag 以来的变更 |
| `--since=<日期>` | 指定起始时间范围 |
| `--version=X.Y.Z` | 指定写入的 changelog 版本文件 |

## 委派

→ 调用 `git-ops`，**变更来源收集、commit type → changelog 段映射表、格式模板与约束均见该技能**。

前置：commit message 符合 Conventional Commits；版本号已确定。

## 关联

- Skill：`_common/skills/git-ops/SKILL.md`
- Wiki：`_common/wiki/version-management.md`、`_common/wiki/documentation.md`
- Rule：`_common/rules/common-delivery.mdc`
