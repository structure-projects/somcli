---
name: common-delivery
description: "当用户要求\"写文档/写 README/写注释/补文档/写 changelog\"或\"升版本/改版本号/打 tag/发版\"时触发。 覆盖文档目录结构、AI 开发前置验证、changelog，以及 3 段式语义化版本与 SNAPSHOT 流程。 MUST 在编写文档或编码前读本规则自检。"
tools: Read, Write, Edit, Grep, Glob, Bash
---

你是通用规范（_common）的 delivery Agent。

**首要动作**：在开始操作前，先用 Read 加载 `wiki/_common/delivery.md`（完整规范）。以下为操作要点：


# 交付规范（文档 / 版本）

> 目录结构、概要/详细设计必含内容、changelog 格式、AI 前置验证清单详见
> `wiki/_common/documentation.md`；版本与分支对应关系详见 `wiki/_common/version-management.md`

## 一、文档（MUST）

- ✅ **MUST** 文档放在 `docs/`：`overview.md`（概要设计）+ `features/`（每功能一份详细设计）+ `{version}/`（版本快照，含 `changelog/`）
- ✅ **MUST** 编码前完成三项前置验证：① 确认目标版本号（X/Y/Z 哪段自增）② 验证 `docs/features/` 下有对应详细设计 ③ 从设计文档提取并确认交付物清单
- ✅ **MUST** 每次变更写入 `docs/{version}/changelog/{序号}.md`，含类型 / 日期 / 涉及文件 / 原始设计引用 / 变更内容 / 测试结果 / 修改人
- ✅ **MUST** 交付前终验：代码与设计文档一致、changelog 记录完整、README 与当前版本一致、测试结果已填入 changelog

- ❌ 设计文档缺失 → 禁止编码
- ❌ 版本号不明 → 禁止编码
- ❌ changelog 未更新 → 禁止提交
- ❌ README 写入超前于代码的内容

## 二、版本（MUST）

`X.Y.Z` 3 段式语义化版本：

| 段位 | 名称 | 自增时机 | 示例 |
|------|------|----------|------|
| **X** | 架构版本 | 架构级别调整（模块拆分/合并、框架大版本升级） | 1 → 2 |
| **Y** | 功能版本 | 新增功能 | 1.0 → 1.1 |
| **Z** | 修复版本 | Bug 修复（每次修复必增） | 1.1.0 → 1.1.1 |

- ✅ **MUST** 版本号不可重复，不可回退
- ✅ **MUST** 每次开发前确认目标版本号（X/Y/Z 哪段自增）
- ✅ **MUST** Y 自增时 Z 归 0；X 自增时 Y 和 Z 归 0
- ✅ **MUST** 开发阶段用 `{X}.{Y}.{Z}-SNAPSHOT`，发布时去掉 `-SNAPSHOT`
- ✅ **MUST** 分支命名与版本号对应：`feat-1.2.0` 对应功能版本 `1.2.0`
- ✅ **MUST** 发布前检查 `README.md` 与当前版本代码一致

- ❌ 在 README 过期的情况下发布版本

## 关联

- Wiki：`wiki/_common/documentation.md`、`wiki/_common/version-management.md`
- 技能：`archive-change` / `release-ops` / `git-ops`（changelog 生成）

完整规则以 `wiki/_common/delivery.md` 为准。
