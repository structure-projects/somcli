# Git 分支策略与工作流规范

> 通用规则，适用范围：所有技术栈和项目类型。

## ⚡ 动作前自检（执行 git 操作前 MUST 执行）

> 声明式规则无法物理阻止 git 动作，以下自检必须在执行前主动完成。若已安装 `_common/checks/commit-msg.sh` hook，不合规提交会被 git 物理拦截。

- **执行 `git commit` 前**，MUST 先校验 commit message 符合 Conventional Commits：
  - 格式 `<type>(<scope>): <description>`
  - 合法 type：`feat | fix | docs | style | refactor | test | chore | perf`
  - 不合规 MUST 先修改 message 再提交，**禁止**直接 `git commit -m "随意内容"`
- **执行 `git push` 前**，MUST 确认当前分支非 `master`；若当前分支为 `develop`，MUST 确认属单人场景（多人协作走 PR）。
- **执行 `git merge` 前**，MUST 确认源分支类型与目标分支匹配（feat→develop，hotfix→master+develop）。
- **创建分支前**，MUST 使用规范前缀：`feat-` / `fix-` / `release-` / `hotfix-`。

## 分支模型

```
master ──────────────────────── ●(hotfix merge) ──────
  ↑                 ↑          ↑
develop ────●←feat-A─●←feat-B──●←release-1.2.0──●←fix-C──
            ↑        ↑         ↑                 ↑
          feat-A   feat-B   release-1.2.0      fix-C
```

### 分支定义

| 分支 | 用途 | 来源 | 合并目标 | 生命周期 |
|------|------|------|----------|----------|
| `master` / `main` | 生产环境稳定代码 | — | — | 永久 |
| `develop` | 开发主分支，集成所有功能 | `master` | — | 永久 |
| `feat-{描述}` 或 `feat-{版本号}` | 功能开发 | `develop` | `develop` | 合并后删除 |
| `fix-{描述}` 或 `fix-{版本号}` | Bug 修复（开发环境） | `develop` | `develop` | 合并后删除 |
| `release-{版本号}` | 发布准备 | `develop` | `master` + `develop` | 合并后删除 |
| `hotfix-{版本号}` | 生产环境热修复 | `master` | `master` + `develop` | 合并后删除 |

## 分支策略（MUST）

### 功能开发

- **MUST** 所有功能开发基于 `develop` 分支。
- **禁止** 多人协作场景下开发者直接在 `develop` 分支上提交代码（单人场景见「流程分级」）。
- **MUST** 从 `develop` 派生新分支，命名：`feat-{功能描述}` 或 `feat-{版本号}`。
  - 示例：`feat-user-login`、`feat-1.2.0`
- **MUST** 功能开发完成后，在分支内完成自测，再合并到 `develop`。
- **SHOULD** 合并后删除功能分支，保持仓库整洁。

### Bug 修复（开发阶段）

- **MUST** 从 `develop` 派生修复分支，命名：`fix-{bug描述}` 或 `fix-{版本号}`。
  - 示例：`fix-login-npe`、`fix-1.1.1`
- **MUST** 修复完成后合并回 `develop`。
- **禁止** 在 `feat-*` 分支中混入不相关的 bug 修复。

### 发布分支（Release）

- **SHOULD** 在功能冻结后、正式发布前，从 `develop` 派生发布分支，命名：`release-{版本号}`。
  - 示例：`release-1.2.0`
- 发布分支用于发布前的集中测试和 Bug 修复。
- **禁止** 在 `release-*` 分支中新增功能。
- **MUST** 发布分支修复完成后，先合并到 `master`（打 tag），再合并回 `develop`。
- **MUST** 合并后删除发布分支。

### 生产环境热修复（Hotfix）

- **MUST** 仅从 `master` 分支派生，命名：`hotfix-{目标版本号}`。
  - 示例：`hotfix-1.1.1`（修复后将发布为 1.1.1）
- **MUST** 修复后同时合并到 `master` 和 `develop`，确保 develop 包含修复。
- **禁止** 在生产热修复分支中夹带新功能。

### 版本发布

- **MUST** tag 只打在 `master` 上（`release-*` 或 `hotfix-*` 合并到 `master` 之后），tag 名 = 版本号。
  - 示例：`v1.2.0`、`v1.1.1`
- **MUST** tag 打在合并后的目标分支上，不打在 `feat-*` / `fix-*` 临时分支上。

## Commit 规范

- **MUST** 使用 Conventional Commits 格式：
  ```
  <type>(<scope>): <description>
  ```
- **MUST** type 为以下之一：
  - `feat` — 新功能
  - `fix` — 修复
  - `docs` — 文档
  - `style` — 格式（不影响代码运行）
  - `refactor` — 重构
  - `test` — 测试
  - `chore` — 构建/工具
  - `perf` — 性能优化
- **SHOULD** commit message 使用中文或英文，一个项目内保持一致。
- **SHOULD** 每个 commit 聚焦单一变更，避免大杂烩 commit。

## PR / MR 规范

- **MUST** PR 标题使用 Conventional Commits 格式。
- **MUST** PR 描述包含：变更目的 / 变更内容 / 测试说明 / 文档同步情况。
- **MUST** PR 合并前确认对应版本的 `docs/{version}/changelog/` 已写入变更记录。
- **SHOULD** 一个 PR 聚焦一个功能或修复，避免大锅饭式 PR。

## 禁止事项

- **禁止** 直接在 `master` 上推送代码；`develop` 按协作场景判定（单人 MAY 推送，多人协作 MUST 走 PR）。
- **禁止** 将 `feat-*` 分支直接合并到 `master`（必须经过 `develop`）。
- **禁止** 在生产热修复分支中夹带新功能。
- **禁止** 在未关联版本号的情况下提交代码（每次开发必须明确目标版本）。

## 分支命名替代方案

> 如果团队已有使用习惯，以下命名作为 `feat-*` / `fix-*` / `hotfix-*` 的等效替代：

| 标准命名 | 替代命名 | 说明 |
|----------|----------|------|
| `feat-*` | `feature-*` | 语义相同，缩写与全称均可 |
| `fix-*` | `bugfix-*` | 语义相同 |
| `hotfix-*` | `hotfix-*`（不变） | 生产热修复用 Hotfix 明确区分紧急程度 |

**建议**：新项目统一使用短前缀（`feat-*` / `fix-*`），与 Conventional Commits 的 type 保持一致。
