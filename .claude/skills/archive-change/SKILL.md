---
name: archive-change
description: |
  当测试通过（编码+测试+评审完成）后、提交代码前触发。
  MUST 归档提案到 changes/archive/，更新 changelog，检查并更新 README。
  归档 MUST 在本地代码推送前完成，与代码变更同一次提交。

triggers:
  - 归档
  - 完成变更
  - 结束变更
  - archive
  - 收尾
  - 完成 proposal
  - 关闭变更

role: devops
phase: ci


allowed-tools: Bash, Read, Write, Edit

related-rules:
  - common-core
  - common-delivery
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/version-management.md
  - wiki/_common/documentation.md

produces:
  - changes/archive/<id>/（从 proposals/ 迁移过来）
  - 更新 changes/changelog/<version>.md
  - 更新 README.md（如有 API/功能变化）

requires:
  - skill: coding
    condition: tasks.md 所有任务已勾选
    error: 存在未完成任务，MUST 先完成 coding
  - skill: testing
    condition: 本地测试全部通过
    error: 测试未通过，MUST 先完成 testing
  - skill: expert-review
    condition: review.md 存在且无未解决 MUST fix
    error: 评审未完成或存在未解决 MUST fix，MUST 先完成 expert-review

human-in-the-loop:
  - 归档前 MUST 确认所有任务完成
  - changelog 内容 MUST 用户确认

on-failure: |
  tasks.md 未全部勾选 → 禁止归档，提示用户完成剩余任务
  review.md 有未解决 MUST fix → 禁止归档，提示用户先修复

mode: auto

category: ci
stack: _common
priority: high
---

# 变更归档

> SDLC 的最后一环：归档变更提案，更新 changelog，更新 README。
> **禁止跳过归档** —— 未归档的变更无法追溯。

## 前置条件（MUST 全部满足）

1. **tasks.md 全部勾选**：`changes/proposals/<current>/tasks.md` 无 `- [ ]` 未完成项
2. **review.md 无未解决 MUST fix**
3. **测试通过**（本地单测 / 集成测试全部通过）
4. **当前分支为 feat-* / fix-* / hotfix-***

任一不满足 → 禁止归档。

## 执行步骤

### 第 1 步：最终检查

```bash
# 检查 tasks.md
grep -c "^- \[ \]" changes/proposals/<current>/tasks.md
# 预期：0

# 检查 review.md 是否有 MUST fix
grep -A 10 "MUST fix" changes/proposals/<current>/review.md
# 预期：无未勾选
```

### 第 2 步：更新 changelog

写入 `changes/changelog/<version>.md`（如 `1.2.0.md`）：

```markdown
## [1.2.0] - 2026-08-15

### Added
- 新增用户登录接口（proposal: 2026-08-15-add-user-login）

### Changed
- ...

### Fixed
- ...
```

条目 MUST 包含：
- 类型（Added / Changed / Fixed / Security / Deprecated / Removed）
- 简短描述
- 关联 proposal ID

### 第 3 步：归档提案

```bash
git mv changes/proposals/<id>/ changes/archive/<id>/
```

### 第 4 步：更新 README 等相关文件

归档完成后 MUST 检查并更新以下相关文件：

| 文件 | 何时更新 |
|---|---|
| `README.md` | 新增功能 / API / 模块 / 版本号变化 / 启动方式变化 |
| `AGENTS.md` | 技术栈表、技能清单、安装说明变化 |
| `docs/USER_GUIDE.md` | 使用方式、触发机制、流程变化 |
| `docs/` 其他文档 | 架构、设计文档与代码不一致时 |

**MUST 更新 README 的情况**：
- 影响用户使用方式的变更
- 新增模块 / 新增加载项
- 版本号变化

### 第 5 步：交由 ci-gate 统一提交

归档 + changelog 的改动 **不单独 commit**，而是与代码变更一起由 `ci-gate` 提交（归档 MUST 在推送前完成，与代码同一次提交）。

### 第 6 步：（可选）合并到 develop / master

```bash
# 功能分支合并到 develop
git checkout develop
git merge --no-ff feat-<name>

# hotfix 合并到 master + develop
git checkout master
git merge --no-ff hotfix-<version>
git checkout develop
git merge --no-ff hotfix-<version>
```

## 产出物

- `changes/archive/<id>/`（完整提案目录）
- 更新 `changes/changelog/<version>.md`
- 更新 `README.md`（如需要）
- 合并 commit

## 完成标准

- 提案目录已从 proposals/ 移到 archive/
- changelog 含本次变更条目
- README 已更新（如需要）
- 归档 + changelog 改动已就绪，交由 ci-gate 与代码同次提交

## 下一步（可选）

归档完成后，可选继续：

- **打 Tag + 发 Release** → 调用 `release-ops` 技能
- **触发发布流水线** → 用 `gh workflow run` 触发对应 workflow（Maven / npm / Docker）
- **结束本次变更** → 无后续

## 关联

- 前置：`testing` `expert-review`
- 后续：`ci-gate`（统一提交，归档与代码同次提交）
- Wiki：`wiki/_common/version-management.md` `wiki/_common/documentation.md`
