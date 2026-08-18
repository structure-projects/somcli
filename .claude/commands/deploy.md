---
name: deploy
description: |
  部署并验证服务健康状态。
  当用户请求 /deploy 或"部署"时触发。
  ⚠️ 生产部署 MUST 用户确认（L0）。
category: ops
phase: deploy
triggers:
  - deploy
  - 部署
related-skills:
  - deployment-verification
related-rules:
  - common-runtime
  - common-delivery
stack: _common
priority: medium
---

# 部署验证（Deploy）

## 目的

部署到目标环境并系统性验证服务健康状态、功能正确性、性能指标，确保发布质量。

## 使用场景

- 新版本部署后验证
- 生产环境发布后的冒烟测试 / 灰度发布验证
- 回滚决策依据

## 参数

| 参数 | 环境 | HITL 级别 |
|---|---|---|
| `--staging` / `--dev` | 测试环境 | L2（显式指定不再询问） |
| `--prod` | 生产环境 | **L0（MUST 用户确认，不可自动跳过）** |
| 无参数 | 由 AI 按上下文判定 | 生产一律回落 L0 |

## 委派

→ 调用 `deployment-verification`，**前置条件、健康检查表与具体命令、核心功能清单、基线对比判定与报告模板均见该技能**。

前置：`ci-gate` 全部通过；changelog 与版本号已更新。

**MUST 用户确认**：生产部署与回滚均为 L0，AI MUST NOT 自行执行。

## 关联

- Skill：`_common/skills/deployment-verification/SKILL.md`
- Skill：`_common/skills/infra-ops/SKILL.md`
- Wiki：`_common/wiki/deployment.md`、`_common/wiki/observability.md`
- Rule：`_common/rules/common-delivery.mdc`
