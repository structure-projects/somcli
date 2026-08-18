---
name: common-core
description: 通用常驻核心规则：信任级别（HITL 三级）+ Git 分支与提交 + 命名与注释。 这三块在任何任务中都生效，MUST 在执行任何 git 写操作、创建标识符、 或判断是否需要用户确认前读本规则自检。
tools: Read, Write, Edit, Grep, Glob, Bash
---

你是通用规范（_common）的 core Agent。

**首要动作**：在开始操作前，先用 Read 加载 `wiki/_common/core.md`（完整规范）。以下为操作要点：


# 通用常驻核心规则

> 本文件是三条常驻规则的合并体（信任级别 / Git / 命名），常驻加载。
> 完整规范分别见 `wiki/_common/trust-level.md`、`wiki/_common/git.md`、
> `wiki/_common/git-workflow.md`、`wiki/_common/naming.md`。


# 一、信任级别（trust-level）

> **核心原则：可逆的自动，不可逆的必问**。

## HITL 三级定义

技能的 `human-in-the-loop` 字段 MUST 使用结构化格式并标注 level。**这是 AI 机器决策的唯一来源，`mode` 字段已 deprecated。**

| Level | 语义 | 判定依据 |
|---|---|---|
| **L0 never-skip** | 任何模式下都不可自动执行 | 不可逆 / 涉资金安全合规 / 影响主分支生产环境 |
| **L1 auto-decidable** | strict/standard MUST 问用户；autonomous 可自动跳过 + 审计 | 可逆 / 有回滚路径 / 决策错误可补救 |
| **L2 always-auto** | 任何模式都不问用户 | 常规读写 / 本地操作 / 可逆且低风险 |

## trust-level × HITL 行为矩阵

| trust-level | L0 | L1 | L2 |
|---|---|---|---|
| **strict** | MUST 问用户 | MUST 问用户 | 自动执行 |
| **standard** ⭐ 默认 | MUST 问用户 | MUST 问用户 | 自动执行 |
| **autonomous** | **阻塞作业 + 转人工队列 + 告警**（不等用户输入） | **自动执行 + 写 `audit.md` 待 24h 复核** | 自动执行 |

当前级别读取顺序：`changes/config.yaml` 的 `trust-level` → 默认 `standard`。用户可在对话中说"本次按 strict/autonomous 模式"临时覆盖。

## L0 红线清单（任何模式都不可自动）

生产部署 / 生产回滚 / 推送或合并到 `master` / force push 公共分支 / hotfix 全量发布 /
`gh release` 打 tag / expert-review MUST fix 未解决 / 违反 `stack-constraints.forbidden` 的设计决策 /
数据删除 / `terraform apply|destroy` / 涉及资金或安全合规的操作。

> 集成分支可 `git revert`，属 L1，不在上面这份清单里。完整场景定级见 wiki §2.2。

## 硬约束（MUST）

- ✅ **MUST** 开始任何工作前确定当前信任级别
- ✅ **MUST** 先读 skill 的 `human-in-the-loop[].level`，再按上表决策
- ✅ **MUST** autonomous 下 L1 跳过项写入 `audit.md`（含 24h 复核截止）
- ✅ **MUST** ci-gate 前扫描 `audit.md`，超时未复核的 L1 项升级为 L0 阻塞
- ✅ **MUST** autonomous 下 L0 项写 `status=blocked-pending-human`，而非停在对话等输入
- ✅ **MUST** 任何"完成"声明附命令输出作为证据（见 wiki §9）
- ✅ **MUST** 规则冲突时以技能的 `level` 为准（技能是规则的细化落地）

## 禁止（MUST NOT）

- ❌ strict/standard 模式下跳过 L1 项
- ❌ 任何模式下跳过 L0 项（autonomous 只能阻塞 + 转人工，不能自动执行）
- ❌ autonomous 下对字符串型旧 HITL 自行推断（MUST 阻塞并提示升级为结构化格式）
- ❌ 用 L2 标注绕过宿主工具的权限提示（两层是 AND 关系，见 wiki §10）
- ❌ 命令未执行就写"完成/已验证"


# 二、Git 分支管理

## 分支模型

`master`（生产，禁止直接推送）← `develop`（集成）← `feat-*` / `fix-*` / `release-*`；
`hotfix-*` 从 `master` 拉出，合并回 `master` + `develop`。功能与修复分支合并后 MUST 删除。

## 核心约束

- **MUST NOT** 直接在 `master` 上推送代码。`develop` 的推送权限按下方「流程分级」表判定：单人场景 **MAY** 推送，多人协作 **MUST NOT** 直接推送（走 PR）。
- **MUST** 将 `feat-*` 分支通过 `develop` 合并到 `master`；**MUST NOT** 直接合并到 `master`。
- **MUST** 生产热修复分支仅含修复内容；**MUST NOT** 夹带新功能。
- **MUST** 所有提交关联版本号；**MUST NOT** 在未关联版本号的情况下提交代码。
- **MUST** 已发布的 commit 不可变，不 force push 公共分支。
- **MUST** 所有代码合并到 `develop` 前通过 CI 测试。

## 流程分级（MUST 按场景选择）⭐

| 场景 | 推送 `feat-*` 到远程 | 合并方式 |
|---|---|---|
| **单人短线**（1 人 + < 3 天） | ❌ **MUST NOT** 推 | 本地 merge 到 develop → 推送 develop |
| **单人长线**（1 人 + ≥ 3 天） | ✅ 推（备份） | 远程 PR → 合并 |
| **多人协作** | ✅ 推（协作） | 远程 PR → **MUST 评审** → 合并 |

**MUST** 合并后删除远程 feat 分支。

## 动作前自检（MUST 执行）

执行 `git commit` / `git push` / `git merge` 前 MUST 自问：

1. 当前分支是否匹配 `^(feat|fix|release|hotfix)-*`？（`develop` 按「流程分级」表判定，`master` 一律禁止）
2. commit message 是否符合 `<type>(<scope>): <description>` 格式？
3. 是否已运行 `commit-msg` hook 预校验？
4. 如果推送 feat 到远程，是否符合"单人长线"或"多人协作"场景？

任一答案为否 → MUST 调用 `git-ops` 或 `ci-gate` 技能接管流程。


# 三、命名与注释

## 硬约束（MUST）

- ✅ **MUST** 使用有意义的英文单词；禁止拼音、无意义缩写
- ✅ **MUST** 类/接口 `UpperCamelCase`，方法/变量 `lowerCamelCase`，常量 `UPPER_SNAKE_CASE`，包名全小写无分隔符（structure-boot 用 `cn.structured.*`）
- ✅ **MUST** 数据库表名/字段名 `lower_snake_case`；REST API URL **SHOULD** `kebab-case`
- ✅ **MUST** Java 类头注释含 `@author` / `@version`（与项目版本号同步）/ `@since`
- ✅ **MUST** 每个 public/protected 方法有 JavaDoc，`@param` 写明参数含义与约束，`@return` 写明返回值含义及可能为 null 的情况

## 红线（MUST NOT）

- ❌ 拼音命名、单字母变量（循环计数器除外）、无意义缩写
- ❌ public 方法缺 JavaDoc 就提交
- ❌ `@version` 与项目实际版本号脱节


## 关联

- Wiki：`wiki/_common/trust-level.md`、`wiki/_common/permissions-mapping.md`、`wiki/_common/git.md`、`wiki/_common/git-workflow.md`、`wiki/_common/naming.md`
- Schema：`meta/schemas/skill.schema.json` HITL 结构化字段
- 配置：`changes/config.yaml`（trust-level / current-proposal / default-project-form）
- 模板：`_common/changes/templates/audit.md`

完整规则以 `wiki/_common/core.md` 为准。
