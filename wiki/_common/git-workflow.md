# Git 工作流（分级规范）

> 本文档是 structure-projects 生态的 Git 工作流单一来源。
> 所有 `git-ops` / `ci-gate` 等技能 MUST 遵循本文档。

## 流程分级（MUST 按场景选择）

### 分级决策树

```
开始一个任务
   ↓
Q1: 是多人协作 还是 单人独立？
   ├─ 多人 → 【多人协作流程】
   ↓ 单人
Q2: 预期完成时间 ≥ 3 天 或 需要远程备份？
   ├─ 是 → 【单人长线流程】
   └─ 否 → 【单人短线流程】
```

### 三种流程对比

| 维度 | **单人短线** | **单人长线** | **多人协作** |
|---|---|---|---|
| 适用 | 1 人 + < 3 天 + 无需远程 | 1 人 + ≥ 3 天 或 需远程备份 | ≥ 2 人协作 |
| 分支 | 本地 `feat-<name>` | 本地 `feat-<name>` | 远程 `feat-<name>` |
| 推送远程 | ❌ **不推** | ✅ 推（备份） | ✅ 推（协作） |
| 合并方式 | 本地 merge 到 develop → 推送 develop | 远程 PR → 合并 | 远程 PR → 评审 → 合并 |
| 评审 | 可选（自评） | 建议 | **MUST** |
| CI 触发 | 仅本地 | 本地 + 远程 | 本地 + 远程 + 强制 |
| 典型场景 | 个人小修改、文档调整、demo | 中型功能、个人分支保护 | 团队功能、跨人协作 |

## 分支命名（MUST）

| 分支 | 用途 | 来源 | 删除时机 |
|---|---|---|---|
| `master` | 生产环境 | — | 永不删除 |
| `develop` | 开发主分支 | master | 永不删除 |
| `feat-{描述/版本}` | 功能开发 | develop | 合并后删除 |
| `fix-{描述/版本}` | Bug 修复（开发环境） | develop | 合并后删除 |
| `release-{版本号}` | 发布准备 | develop | 合并到 master+develop 后删除 |
| `hotfix-{版本号}` | 生产热修复 | master | 合并到 master+develop 后删除 |

## 流程详述

### 单人短线流程（默认，最常见）

**适用**：1 人 + 短任务（< 3 天）+ 无需远程协作

```bash
# 1. 切到 develop，拉最新
git checkout develop
git pull origin develop

# 2. 创建本地 feat 分支（不推远程）
git checkout -b feat-add-user-login

# 3. 编码 + 提交（本地）
git add .
git commit -m "feat(user): 新增用户登录接口"

# 4. 完成后本地合并到 develop
git checkout develop
git merge --no-ff feat-add-user-login

# 5. 推送 develop 到远程
git push origin develop

# 6. 删除本地 feat 分支
git branch -d feat-add-user-login
```

**关键约束**：
- ❌ **MUST NOT** 推送 `feat-*` 到远程（避免远程分支污染）
- ✅ **MUST** 完成后合并到 `develop` 并推送
- ✅ **MUST** 合并后删除本地 feat 分支

### 单人长线流程

**适用**：1 人 + 长任务（≥ 3 天）或 需远程备份

```bash
# 1. 创建 feat 分支并推送远程（备份）
git checkout develop && git pull
git checkout -b feat-<name>
git push -u origin feat-<name>

# 2. 每日提交并推送（防丢失）
git add . && git commit -m "..."
git push origin feat-<name>

# 3. 完成后发起 PR（自评 + CI）
gh pr create --base develop --title "feat(user): ..."

# 4. CI 通过后合并
gh pr merge --squash  # 或 --merge / --rebase

# 5. 删除远程分支
git push origin --delete feat-<name>
```

**关键约束**：
- ✅ **MUST** 推送远程（防本地丢失）
- ✅ **SHOULD** 通过 PR 合并（触发 CI + 留痕）
- ✅ **MUST** 合并后删除远程分支

### 多人协作流程

**适用**：≥ 2 人协作

```bash
# 1. 创建 feat 分支并推送远程
git checkout develop && git pull
git checkout -b feat-<name>
git push -u origin feat-<name>

# 2. 每日推送 + 与 develop 同步
git fetch origin
git rebase origin/develop  # 或 merge
git push origin feat-<name>

# 3. 完成后发起 PR
gh pr create --base develop --title "..." --body "..."

# 4. 请求评审（MUST）
gh pr request-review @<reviewer>

# 5. 处理评审意见（review-fix-loop 技能）
# 修复 → 提交 → 推送 → 复评

# 6. 评审通过后合并
gh pr merge --squash

# 7. 删除远程分支
git push origin --delete feat-<name>
```

**关键约束**：
- ✅ **MUST** 推送远程
- ✅ **MUST** 通过 PR 合并
- ✅ **MUST** 评审（至少 1 人）
- ❌ **MUST NOT** 直接推 `develop` / `master`
- ❌ **MUST NOT** force push 共享分支

## Commit 规范（Conventional Commits）

### 格式

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Type 白名单

| type | 用途 | 示例 |
|---|---|---|
| `feat` | 新功能 | `feat(user): 新增用户登录接口` |
| `fix` | 修复 | `fix(auth): 修复 token 过期问题` |
| `docs` | 文档 | `docs(readme): 更新安装说明` |
| `style` | 格式（不影响代码运行） | `style(java): 统一缩进` |
| `refactor` | 重构 | `refactor(user): 拆分 UserService` |
| `test` | 测试 | `test(user): 补充登录集成测试` |
| `chore` | 杂务（构建/依赖/工具） | `chore(deps): 升级 Spring Boot 到 4.0.6` |
| `perf` | 性能 | `perf(user): 优化登录查询` |

### Scope

按受影响模块/包名推断（小写、可省略）。如 `user`、`auth`、`config`。

### Description

- 祈使句、现在时
- ≤ 50 字
- 首字母小写（中文无大小写约束）
- 结尾不加句号

### Body（可选）

说明**为什么**改（不是改了什么，diff 已说明 what）。

### Footer（可选）

- `BREAKING CHANGE: <说明>` —— 破坏性变更
- `Closes #123` —— 关联 issue

## 核心约束（红线）

- ❌ **MUST NOT** 直接在 `master` 上推送代码；`develop` 按协作场景判定（单人 MAY 推送，多人协作 MUST 走 PR）
- ❌ **MUST NOT** `feat-*` 分支直接合并到 `master`（必须经过 `develop`）
- ❌ **MUST NOT** 在生产热修复分支中夹带新功能
- ❌ **MUST NOT** 在未关联版本号的情况下提交代码
- ❌ **MUST NOT** force push 公共分支（master / develop / 共享 feat）
- ✅ **MUST** 已发布的 commit 不可变
- ✅ **MUST** 所有代码合并到 `develop` 前通过 CI 测试
- ✅ **MUST** 所有提交关联变更提案（`changes/proposals/<id>/`）；唯一豁免：`/quick-fix`（≤5 文件且 ≤50 行）免提案但不免门禁

## 关联

- 技能：`git-ops` / `ci-gate` / `ci-pipeline`
- 规则：`common-core`
- Wiki：`wiki/_common/version-management.md`
