# 变更提案：<标题>

| 字段 | 值 |
|---|---|
| 提案 ID | YYYYMMDD-<kebab-case-name> |
| 级别 | major |
| 创建日期 | YYYY-MM-DD |
| 创建人 | <user> |
| 状态 | draft / requirement / coding / review / ci / deployed / archived / blocked-pending-human |
| 优先级 | high / medium / low |
| 关联 Issue | #123 |

## 需求描述

<用户故事 / 问题描述。说明为什么要做这个变更，解决什么问题>

## 目标

<本次变更要达成什么。可衡量、可验证>

## 非目标

<本次变更明确不做什么。防止范围蔓延>

## 影响范围

- **模块**：<如 `structure-user`>
- **数据表**：<如 `user`、`user_role`>
- **接口**：<如 `POST /api/users/login`>
- **依赖服务**：<如 `structure-security`>
- **前端页面**：<如 `/users/login`>

## 技术方案

<高层方案概述。复杂变更 MUST 在 `design.md` 中提供详细设计>

## 风险点

- <风险 1 + 缓解措施>
- <风险 2 + 缓解措施>

## 回滚预案

<部署失败或线上问题时如何回滚。包括数据库回滚、配置回滚、代码回滚>

## 验收标准

- [ ] 功能按需求描述工作
- [ ] 单元测试覆盖率 ≥ 80%
- [ ] 集成测试通过
- [ ] 通过 expert-review 评审
- [ ] CI 全部通过
- [ ] 部署验证通过

## 任务清单

详见 `tasks.md`。

## 变更日志

完成后在 `changelog/<version>.md` 中补条目。
