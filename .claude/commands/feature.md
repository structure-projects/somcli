---
name: feature
description: |
  新建特性目录 / 子包 / 非代码目录。
  当用户请求 /feature 或"新建特性"时触发。
category: coding
phase: coding
triggers:
  - feature
  - 新建特性
related-skills:
  - create-feature
related-rules:
  - common-project-structure
  - common-core
stack: _common
priority: medium
---

# 新建特性（Feature）

## 目的

按目录类型（特性目录 / 子包 / 非代码目录）创建对应结构，支持跨层组织的特性开发模式。

## 使用场景

- 新建独立功能模块 / 按特性组织跨层代码
- 在现有包下创建子包
- 创建非代码目录（docs / scripts / examples）

## 参数

| 参数 | 目录类型 |
|---|---|
| `--feature-dir` | 特性目录（跨层组织，如 `features/user-management/`） |
| `--subpackage` | 子包（在现有 package 下创建） |
| `--plain` | 非代码目录（不放源代码） |
| 无参数 | **MUST 询问用户，MUST NOT 默认按"子包"处理** |

## 委派

→ 调用 `create-feature`，**三种目录类型的结构模板、README 生成规则与结构验证步骤均见该技能**。

**MUST NOT** 在 `src/main/java/` 下创建非代码目录。

## 关联

- Skill：`_common/skills/create-feature/SKILL.md`、`_common/skills/coding/SKILL.md`
- Wiki：`_common/wiki/project-structure.md`
- Rule：`_common/rules/common-project-structure.mdc`
