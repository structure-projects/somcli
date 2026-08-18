---
name: audit
description: |
  项目审计统一入口：现状审计（老项目接入）+ 安全审计。
  当用户请求 /audit 或 /scan 时触发。
  ⚠️ 生产环境扫描 MUST 用户确认（L0）。
category: review
phase: review
triggers:
  - audit
  - scan
related-skills:
  - audit
  - project-intake
related-rules:
  - common-security
  - common-project-stack-detection
stack: _common
priority: high
---

# 审计（Audit）

> 原 `/scan` 已并入本指令（二者均指向 `audit` 技能），`scan` 保留为别名。

## 目的

统一审计入口：评估老项目现状（规范符合度 / 测试 / CI-CD / 文档）作为迁移决策依据，或系统性检查代码、配置、依赖的安全问题并给出分级修复建议。

## 使用场景

- 老项目接入本规范前的评估 → 现状审计
- 上线前 / 定期安全检查、合规性检查 → 安全审计
- 全面评估技术债 → 两者都做

## 参数

| 参数 | 审计类型 | 产出 |
|---|---|---|
| `--codebase` | 现状审计 | `changes/proposals/0000-legacy-onboarding/audit-report.md` |
| `--security` | 安全审计 | 漏洞清单（Critical / High / Medium / Low）+ 修复建议 |
| 无参数 | 由 AI 按来意判定（L1） | 见上 |

## 委派

→ 调用 `audit`，**审计类型判定、各维度检查项与报告模板均见该技能**。
现状审计完成后接 `project-intake`；安全审计完成后接 `expert-review`。

**MUST 用户确认**：对生产环境执行扫描为 L0，不可自动跳过。

## 关联

- Skill：`_common/skills/audit/SKILL.md`（现状审计 5 步 + 安全审计 6 维度）
- Skill：`_common/skills/project-intake/SKILL.md`、`_common/skills/project-intake/SKILL.md`
- Wiki：`_common/wiki/security.md`、`_common/wiki/legacy-onboarding.md`
- Rule：`_common/rules/common-security.mdc`
