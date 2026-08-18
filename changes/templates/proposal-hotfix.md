# Hotfix 变更提案：<标题>

| 字段 | 值 |
|---|---|
| 提案 ID | YYYYMMDD-hotfix-<name> |
| 级别 | hotfix |
| 创建日期 | YYYY-MM-DD |
| 严重度 | critical / high |
| 状态 | draft / requirement / coding / review / ci / deployed / archived / blocked-pending-human |

## 问题描述

<生产环境什么问题。包含错误信息、影响范围、发现时间>

## 影响范围

<影响哪些用户 / 功能 / 数据>

## 修复方案

<怎么修。包含根因分析和修复思路>

## 回滚预案

<失败如何回滚。Hotfix 必须可快速回滚>

## 验证方式

<怎么验证修复成功。包含生产环境验证步骤>

## 事后补全（强制，24h 内完成）

- [ ] 补完整 proposal（复制 `proposal-full.md` 重写）
- [ ] 补 `retrospective.md`（复盘文档）
- [ ] 补完整 changelog
- [ ] 合并回 `master` + `develop`
- [ ] 归档到 `changes/archive/`
