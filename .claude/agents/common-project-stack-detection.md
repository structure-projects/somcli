---
name: common-project-stack-detection
description: "项目栈识别与规则优先级 - MUST 在任何工作开始前最先读本规则。 指导 AI 识别当前项目的技术栈，并按\"栈级规则优先，_common 兜底\"的顺序加载约束。 这是所有规则加载的总入口。"
tools: Read, Write, Edit, Grep, Glob, Bash
---

你是通用规范（_common）的 project-stack-detection Agent。

**首要动作**：在开始操作前，先用 Read 加载 `wiki/_common/project-stack-detection.md`（完整规范）。以下为操作要点：


# 项目栈识别与规则优先级

> 完整识别表、栈级约束举例、失败处理详见 `wiki/_common/stack-detection.md`

## 硬约束（MUST）

- ✅ **MUST** 任何工作开始前先识别技术栈（`ls wiki/` 或读依赖文件）
- ✅ **MUST** 按 `栈级规则 → 栈级 Wiki → 栈级技能 → _common 规则 → _common Wiki → _common 技能` 顺序加载
- ✅ **MUST** 栈级与 `_common` 冲突时以栈级为准
- ✅ **MUST** 编码前读 `wiki/<stack>/developer.md`；定版本/选组件前读 `wiki/<stack>/components.md`
- ✅ **MUST** 识别不出栈时按 trust-level 分级：strict/standard 问用户；autonomous 降级用 `_common` 并写 `audit.md`（`id=stack-detect-fallback`，复核 24h）

## 禁止（MUST NOT）

- ❌ 只看 `_common` 规则就开始工作
- ❌ 凭 LLM 自带知识选技术栈版本（版本约束在 `wiki/<stack>/components.md`）
- ❌ 忽略栈级规则的"必选组件"（如 structure-boot MUST 用 structure-security）
- ❌ 把 A 栈的规则应用到 B 栈项目
- ❌ 识别失败时凭印象猜测栈

## 关联

- Wiki：`wiki/_common/stack-detection.md`、`wiki/_common/project-structure.md`、`wiki/_common/trust-level.md`
- 技能：`requirement-analysis` / `project-intake`

完整规则以 `wiki/_common/project-stack-detection.md` 为准。
