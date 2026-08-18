---
name: git-ops
description: |
  当用户要求"开始新任务/创建分支/新建功能/切分支"、"提交代码/commit/总结变更/规范化提交"
  或"生成 changelog/写变更日志/补 changelog"时触发。
  Git 全流程：分支策略决策（团队规模 × 任务时长）→ Conventional Commits 规范化提交 → Changelog 生成。
  MUST NOT 跳过本技能执行裸 git commit；MUST NOT 默认推送 feat 分支到远程。

triggers:
  - 开始新任务
  - 创建分支
  - 新建功能
  - 拉分支
  - 切分支
  - 开始开发
  - new branch
  - create branch
  - 总结变更
  - 规范化提交
  - commit message
  - 生成 changelog
  - 写变更日志
  - 补 changelog
  - changelog
  - 变更日志

role: developer
phase: coding

allowed-tools: Bash, Read, Write, Edit

related-rules:
  - common-core
  - common-delivery
  - common-project-stack-detection

reads-before-action:
  - changes/config.yaml
  - wiki/_common/git.md
  - wiki/_common/git-workflow.md
  - wiki/_common/version-management.md
  - wiki/_common/documentation.md

produces:
  - 正确的分支策略 + 已创建的本地或远程分支
  - 符合 Conventional Commits 的 commit（含 body 与 check-files 清单）
  - 更新的 changes/changelog/<version>.md

requires: []

human-in-the-loop:
  - id: confirm-collaboration-scale
    action: 团队规模（单人 / 多人）MUST 询问用户
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 判断错误只影响分支是否推远程，可删除远程分支纠正
  - id: confirm-task-duration
    action: 任务时长（短线 / 长线）MUST 询问用户
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 判断错误只影响是否远程备份，可补推或删除
  - id: confirm-changelog-format
    action: changelog 条目归类与措辞可自主决定，破坏性变更标注 MUST 保留
    level: L2-always-auto
    autonomous:
      behavior: auto
    rollback: changelog 为纯文本，可直接编辑修正

on-failure: |
  用户未明确场景 → MUST 追问，MUST NOT 默认推远程
  误推送 feat 到远程 → 引导用户删除远程分支
  commit-msg hook 连续拦截 3 次 → 停下询问用户，MUST NOT 反复试错
  当前在 master → 拒绝提交，引导切到 feat-* / fix-*

category: git
stack: _common
priority: high
---

# Git 操作（分支 + 提交 + Changelog）

> 分支策略决策 → 规范化提交 → Changelog 生成。
> **MUST NOT 默认推送 feat 分支到远程**；**MUST NOT 跳过本技能执行裸 `git commit`**。

按任务选章：开新任务拉分支 → 一；提交代码 → 二；写变更日志 → 三。

---

# 一、分支策略决策

> 按团队规模 + 任务时长选择正确的 Git 流程。

## 第 1 步：确认场景（MUST 询问用户）

```
Q1: 这个任务是多人协作还是单人独立？
    a) 单人独立
    b) 多人协作

Q2: 预期完成时间？
    a) 短线（< 3 天，无需远程备份）
    b) 长线（≥ 3 天，或需要远程备份）
```

## 第 2 步：按回答选择流程

| Q1 + Q2 | 流程 | 分支策略 |
|---|---|---|
| **单人 + 短线** | 单人短线流程 | 本地 `feat-*`，不推远程，完成后合并到 develop 推送 |
| **单人 + 长线** | 单人长线流程 | 本地 `feat-*`，**推远程**（备份），完成后 PR 合并 |
| **多人 + 任意** | 多人协作流程 | 远程 `feat-*`，**MUST PR 评审**，合并后删远程 |

## 第 3 步：执行对应流程

### 单人短线

```bash
git checkout develop && git pull
git checkout -b feat-<name>
# 编码 + 提交（本地）
# 完成后：
git checkout develop
git merge --no-ff feat-<name>
git push origin develop
git branch -d feat-<name>
```

- ❌ **MUST NOT** 推送 feat 到远程
- ✅ **MUST** 合并到 develop 后推送 develop

### 单人长线

```bash
git checkout develop && git pull
git checkout -b feat-<name>
git push -u origin feat-<name>  # 备份
# 编码 + 提交 + 定期推送
# 完成后：
gh pr create --base develop --title "..."
# CI 通过后：
gh pr merge --squash
git push origin --delete feat-<name>
```

### 多人协作

```bash
git checkout develop && git pull
git checkout -b feat-<name>
git push -u origin feat-<name>
# 每日推送 + 与 develop 同步
# 完成后：
gh pr create --base develop --title "..." --body "..."
gh pr request-review @<reviewer>  # MUST 评审
# 评审通过后：
gh pr merge --squash
git push origin --delete feat-<name>
```

- ✅ **MUST** 推送远程 + PR 评审
- ❌ **MUST NOT** 直接推 develop；**MUST NOT** 直接推 master

## 第 4 步：输出明确指引

告诉用户：创建了哪个分支 / 是否推了远程 / 完成后怎么合并 / 是否需要评审。

---

# 二、规范化提交

> 本章接管 `git commit` 动作，确保提交信息符合 Conventional Commits 规范。
> 若已安装 `commit-msg` hook，不合规提交会被 git 物理拦截；本章在 hook 之前完成校验，避免反复试错。

## ⚠️ 用户确认原则（MUST 遵守）

### ✅ 直接执行（不问）

用户输入含明确提交意图时，**禁止**二次确认，直接执行：
- `提交` / `提交代码` / `commit`
- `推送` / `push`
- `/commit` 指令（来自 `_common/commands/commit.md`）
- 任何包含「提交」「commit」「push」「推送」关键词的明确请求

### ⚠️ 必须询问（3 种场景，非阻塞模式下自动处理场景 3）

仅在以下场景询问，其他情况不问：

| 场景 | 询问示例 | 非阻塞模式行为 |
|---|---|---|
| **1. 意图模糊**（没明确说 commit 还是仅保存本地）<br>用户说「保存一下」「打个点」「checkpoint」 | 「请问是：①仅保存本地文件（不生成 commit）还是②规范化 commit + push？」 | 不变 |
| **2. MUST 检查失败**<br>编译挂 / 测试挂 / commit-msg hook 拦截 3 次 | 「编译失败：<错误摘要>。是否走 hotfix 快速通道降级提交（跳过 SHOULD 检查）？」 | 不变 |
| **3. 无 staged 变更**<br>`git diff --staged` 为空 | 「暂无已 stage 的文件。是否 `git add -A` 暂存全部变更？」 | **✅ 自动 `git add -A`，不询问** |

### ❌ 已废弃的确认方式（禁止使用）

- 「确认使用以上 message？[Y/n]」——用户说「提交代码」就是确认，message 是 AI 的职责，**不问**
- 「提交完成，是否推送？[Y/n]」——push 属 L1，**MUST 在 `git push` 执行前确认**（见 ci-gate `push-feat-remote`），而非"提交后补问"；严禁"先推送再补一句要不要推"

## 第 0 步：读取项目配置（MUST）

读取 `changes/config.yaml` 获取：
- `trust-level`（strict / standard / autonomous）
- `non-blocking-confirm.enabled`（是否开启非阻塞模式）

根据配置决定后续行为：
- **autonomous + 非阻塞**：无 staged 文件时自动 `git add -A`
- **standard / strict**：无 staged 文件时询问用户

## 第 1 步：收集变更

运行 `git status` 与 `git diff --staged`。
- 若无可提交内容（staged 为空）：
  - **autonomous + 非阻塞模式**：自动 `git add -A` 将所有变更暂存
  - **其他模式**：按上方「场景 3」询问用户
- 若 staged 非空但还有 unstaged：**不问**，提交完在摘要里一并说明「还有 X 个未 stage 文件未提交」

## 第 2 步：归类 type

分析变更内容，从白名单选定 type：
- `feat` 新功能 | `fix` 修复 | `docs` 文档 | `style` 格式 | `refactor` 重构 | `test` 测试 | `chore` 杂务 | `perf` 性能

## 第 3 步：推断 scope

按受影响模块/包名推断 scope（小写、可省略）。如 `user`、`auth`、`config`。

## 第 4 步：撰写 description

祈使句、现在时、≤50 字、首字母小写（中文无大小写约束）、结尾不加句号。

## 第 5 步：组装 message

`<type>(<scope>): <description>`，示例：`feat(user): 新增用户登录接口`

## 第 6 步：校验

调用 `scripts/validate-msg.sh` 预校验（若存在）；不通过回到第 3 步修正。

## 第 7 步：分支检查

若当前在 `master`，拒绝提交并提示切到 `feat-*` / `fix-*` 分支。
若当前在 `develop`：多人协作场景拒绝提交（走 PR）；单人场景 MUST 经 ci-gate 确认后方可提交。

## 第 8 步：提交

执行 `git commit -m "<message>" -m "<body（可选）>"`（两段 -m 避免 shell 引号续行问题）。
- ⚠️ 提交后 MUST 用 **双重确认** 验证：
  ```bash
  git log --oneline -1        # 确认 HEAD 产生新 commit
  git status --short          # 确认无残留 staged
  ```
  不凭口头推断成功（经验 920063）。

## body 规范（多行，可选）

若变更较多需 body（推荐 ≥ 3 个文件或 ≥ 2 个独立子任务时写 body）：

```
<type>(<scope>): <description>

<空行>
- 要点 1（为什么改）
- 要点 2（影响范围）

check-files:
  - path/to/file1.java
  - path/to/file2.ts
```

- body 说明「为什么」改，不是「改了什么」（diff 已说明 what）。
- body 末尾加 `check-files:` 逐文件清单（配合 commit-msg hook 校验）。
- body 用**两段 `-m`** 提交，避免 shell 引号/续行错误：`git commit -m "<subject>" -m "<body>"`。

## 提交禁止项

- ❌ **MUST NOT** 跳过校验直接 `git commit -m "..."`
- ❌ **MUST NOT** 在 `master` 分支提交；`develop` 提交 MUST 经 ci-gate 确认
- ❌ **MUST NOT** message 仅写「修改」「更新」「fix bug」等无信息内容
- ❌ **MUST NOT** 在用户已明确「提交代码/commit/push」时二次确认
- ❌ **MUST NOT** 仅用 echo 推断提交成功 —— MUST `git log` + `git status` 双重确认
- ⚠️ **注意**：非阻塞模式下，无 staged 文件时自动 `git add -A`（不再禁止），但仅限 `trust-level: autonomous` + `non-blocking-confirm.enabled: true`

---

# 三、Changelog 生成

> 按 Keep a Changelog 规范生成变更日志。

## 格式

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- <新功能>（proposal: <id>）

### Changed
- <变更>（proposal: <id>）

### Deprecated
- <废弃>

### Removed
- <移除>

### Fixed
- <修复>（proposal: <id>）

### Security
- <安全>
```

## 第 1 步：收集变更来源

优先读已归档提案；提案缺失或跨版本汇总时，用 commit 历史兜底：

```bash
ls changes/archive/                          # 首选：已归档提案
git log <last-tag>..HEAD --oneline           # 兜底：自上次 tag 以来
git log --since="2026-08-01" --oneline       # 或指定时间范围
```

## 第 2 步：按类型分组

| commit type | changelog 段 |
|---|---|
| feat | Added |
| fix | Fixed |
| refactor / style / chore | Changed |
| perf | Changed（标注性能提升幅度） |
| docs | Changed（仅影响用户的文档） |
| test | 不入 changelog |
| 破坏性变更（`!` 或 `BREAKING CHANGE:`） | Removed / Changed，**MUST 显式标注 breaking** |

从 commit message 中的提案 ID 关联对应 proposal。

## 第 3 步：生成 changelog

写入 `changes/changelog/<version>.md`。

## 第 4 步：提交

```bash
git add changes/changelog/
git commit -m "docs(changelog): 更新 <version> 变更日志"
```

## Changelog 关键约束

- ✅ **MUST** 按类型分组，MUST NOT 把不同类型混在一起
- ✅ **MUST** 标注 breaking changes
- ✅ **MUST** 关联 issue / proposal ID（如有）
- ❌ **MUST NOT** 遗漏用户可见的变更

---

## 完成标准

- 分支策略与场景匹配，分支已创建并（按需）推送，用户明确后续步骤
- commit message 符合 Conventional Commits，`git log` + `git status` 双重确认通过
- changelog 按类型分组，breaking changes 已标注

## 关联

- 前置：`requirement-analysis`（拉分支前）/ `ci-gate`（推远程前的门禁与 `push-feat-remote` 确认）
- 后续：`coding` / `ci-pipeline`（PR 流程）/ `archive-change` / `release-ops`
- 兜底拦截：`_common/checks/commit-msg.sh`（git commit-msg hook）
- Wiki：`wiki/_common/git.md`、`wiki/_common/git-workflow.md`、`wiki/_common/version-management.md`
- 规则：`common-core`（L0 红线：禁推主干、分支命名前缀）
