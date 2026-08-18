---
name: ci-gate
description: |
  当用户要求"提交/commit/推送/push/合并/merge"时触发。
  MUST 执行本地预检 + 物理拦截 + CI 监控。
  紧急 hotfix 走快速 CI 通道（仅 MUST 检查）。

triggers:
  - 提交
  - commit
  - 推送
  - push
  - 合并
  - merge
  - 提交代码
  - 保存
  - 打个点
  - checkpoint

role: devops
phase: ci


allowed-tools: Bash, Read

related-rules:
  - common-core
  - common-project-stack-detection

reads-before-action:
  - changes/config.yaml
  - wiki/_common/git.md
  - wiki/_common/ci-cd-pipeline.md
  # 栈级规范（MUST 根据识别的栈动态替换 <stack>）
  - wiki/<stack>/developer.md
  - wiki/<stack>/components.md

produces:
  - git commit
  - git push
  - CI 通过

requires:
  - skill: coding
    condition: tasks.md all checked
  - skill: testing
    condition: 本地测试通过
  - skill: expert-review
    condition: changes/proposals/<current>/review.md exists
    error: 缺少评审报告，MUST 先调用 expert-review
    auto-bypass:
      when: trust-level == autonomous
      action: ai-self-review
      mark: autonomous-self-review
      post-condition: 24h 内 MUST 人工复核
      audit: true
  - skill: archive-change
    condition: 提案已归档 + changelog 已更新（归档 MUST 在推送前完成）
    error: 未归档，MUST 先完成 archive-change

human-in-the-loop:
  # L0 不可逆 / 生产操作（任何模式都不可自动；autonomous 阻塞转人工）
  - id: push-master
    action: 推送到 master MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（生产主干不可逆）
  - id: force-push
    action: 强制推送（force push）MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: alert
      alert: true
    rollback: N/A（已发布 commit 不可变）
  - id: hotfix-merge-master
    action: 合并 hotfix 到 master MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: deploy-production
    action: /deploy 生产环境 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: pkg-publish
    action: pkg-publish 打 tag + 发布 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（发布不可逆）

  # L1 意图模糊/检查降级（strict/standard 问；autonomous 有默认）
  - id: clarify-save-vs-commit
    action: 模糊措辞澄清（保存/打个点/checkpoint → 仅保存本地 or 规范化 commit+push）
    level: L1-auto-decidable
    autonomous:
      behavior: auto-pass
      audit: true
    rollback: 误 commit 可 git reset
  - id: downgrade-to-hotfix-path
    action: 编译失败/核心单测失败 询问是否走 hotfix 快速通道降级提交
    level: L1-auto-decidable
    autonomous:
      behavior: auto-pass
      audit: true
    rollback: 可改走标准通道重跑 CI
  - id: commit-msg-hook-failure
    action: commit-msg hook 3 次拦截失败询问是否人工介入
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 修复 commit message 重试
  - id: push-feat-remote
    action: git push 到 feat-* / fix-* 等非主干分支 MUST 经确认（宿主权限门禁或用户确认）
    level: L1-auto-decidable
    autonomous:
      behavior: auto-pass
      audit: true
    rollback: 删除远程 feat 分支 / 修正推送
  - id: push-develop
    action: 推送/合并到 develop（单人场景直推；多人协作 MUST 走 PR）
    level: L1-auto-decidable
    autonomous:
      behavior: auto-pass
      audit: true
    rollback: git revert / 重置 develop

  # L2 明确授权（任何模式都直接执行）
  - id: direct-commit-explicit
    action: 用户明确输入「提交代码/commit/推送/push/合并/merge」时 commit 环节不问
    level: L2-always-auto
    autonomous:
      behavior: auto-pass
      audit: false
    rollback: git reset
  - id: deploy-staging-direct
    action: 用户显式 /deploy 测试环境（非生产）时不问
    level: L2-always-auto
    autonomous:
      behavior: auto-pass
      audit: false
    rollback: 部署撤销
  - id: commit-local
    action: 本地 git commit 不问（可逆，git reset 可撤销）
    level: L2-always-auto
    autonomous:
      behavior: auto-pass
      audit: false
    rollback: git reset

on-failure: |
  本地预检失败 → 修复后重试
  CI 失败 → 分析日志修复；strict/standard 3 次失败问用户；autonomous 阻塞转人工+告警
  hotfix 紧急 → 走快速 CI（仅 MUST 检查）
  执行前扫描 audit.md：超时未复核的 L1 跳过项 MUST 升级为 L0 阻塞（强制复核闭环）

mode: auto  # deprecated: 已被 HITL 结构化 level + trust-level 矩阵替代

# 栈级硬约束（MUST 遵守）
stack-constraints:
  structure-boot:
    spring-boot-version: "4.0.6"
    jdk: "17+"
    parent: "cn.structured:structure-dependencies:1.4.4"
    required-components:
      - structure-security
      - structure-infra
      - structure-restful-web-starter
    forbidden:
      - "Jackson / Gson"
      - "RestTemplate / WebClient"
      - "Spring Boot 3.x"
  vue3:
    required-components:
      - "@structure-projects/components"
      - "@structure-projects/wujie-subapp"
      - "@structure-projects/gateway-client"
    forbidden:
      - "Vue 2"
  react:
    forbidden:
      - "class 组件（必须函数式 + Hooks）"

category: ci
stack: _common
priority: high
---

# CI 门禁

> 提交代码的物理门禁。本地预检 + git hooks + CI 监控。
> 即使绕过其他 skills，本层也拦得住。

## 前置条件（MUST 全部满足）

1. **编码完成**：`tasks.md` 所有任务勾选
2. **测试通过**：本地测试全部通过
3. **评审完成**：`changes/proposals/<current>/review.md` 存在，无未解决的 MUST fix

> 例外：`/quick-fix` 走「Quick-fix 快速通道」时豁免第 1、3 项（无提案即无 tasks.md / review.md），第 2 项不豁免。

## 分级检查

### MUST 检查（任何提交都必须通过）

- commit-msg 格式（Conventional Commits）
- 分支名（`feat-*` / `fix-*` / `hotfix-*` / `release-*`）
- 编译通过（`mvn clean package -DskipTests` / `npm run build` / ...）
- 核心单测通过

### SHOULD 检查（hotfix 可降级）

- 覆盖率 ≥ 80%
- 全量测试通过
- lint 无 error
- 安全扫描通过

## 执行步骤

### 第 0 步：读取项目配置（MUST）

读取 `changes/config.yaml` 获取：
- `trust-level`（strict / standard / autonomous）
- `non-blocking-confirm.enabled`（是否开启非阻塞模式）
- `non-blocking-confirm.queue-operations`（需要排队的操作类型）

根据信任级别 × HITL 矩阵决策：
- **autonomous + 非阻塞**：L2 操作自动执行，L0 操作阻塞+转人工队列+告警
- **standard**：L0/L1 问用户，L2 自动
- **strict**：L0/L1 问用户，L2 自动

### 第 1 步：本地预检

```bash
# 分支检查
git branch --show-current | grep -E "^(feat|fix|hotfix|release)-"

# 编译
mvn clean package -DskipTests  # 或 npm run build / pytest

# 核心单测
mvn test  # 或 npm test / pytest
```

### 第 2 步：生成 commit message

调用 `git-ops` 子技能（MUST 在 ci-gate 门禁通过后执行）：
- 按 Conventional Commits 生成 `<type>(<scope>): <description>`
- 校验 commit-msg hook
- 若 `trust-level: autonomous` + `non-blocking-confirm.enabled: true` 且 staged 为空，自动 `git add -A`

### 第 3 步：提交

```bash
git commit -m "<message>"
```

### 第 4 步：推送（MUST 先确认后执行）

推送前 MUST 按 HITL `push-feat-remote`（L1）处理，先停下来等确认：
- **strict / standard**：MUST 问用户，收到确认后才执行
- **autonomous**：可自动执行 + 写 audit.md
- **禁止"先执行再询问/汇报"**：宿主工具弹的"是否执行"必须在命令执行前获得批准

```bash
git push origin <branch>
# 首次推送：
git push -u origin <branch>
```

### 第 5 步：监控远程 CI

- 追踪 CI 状态（GitHub Actions / GitLab CI / Jenkins）
- 失败 MUST 修复，不允许"先合并再说"

## 快速通道

### Hotfix 快速通道

紧急 hotfix 时可降级 SHOULD 检查：
- 跳过覆盖率检查
- 跳过全量测试（仅跑核心单测）
- 事后 24h 内补跑完整 CI

MUST 检查任何情况都不可跳过。

### Quick-fix 快速通道

`/quick-fix`（≤5 文件且 ≤50 行的已知原因修复）走此通道：
- 豁免：变更提案（proposal.md）、archive-change 归档、覆盖率检查
- **不豁免**：commit-msg hook、编译、相关单测
- 任一不豁免项失败 MUST 转完整流程，MUST NOT 降级放行
- MUST NOT 用于生产热修复（转 hotfix-release）

## 完成标准

- commit-msg hook 通过
- 编译通过
- 核心单测通过
- 远程 CI 通过
- 推送成功

## 下一步

门禁全部通过且推送成功后 MUST 进入 `deployment-verification`（部署验证）。

- 生产部署为 L0，autonomous 模式下 MUST 阻塞转人工队列 + 告警
- 主线顺序：`coding` → `testing` → `expert-review` → `archive-change` → **`ci-gate`** → `deployment-verification`

## 关联

- 前置：`coding` `testing` `expert-review` `archive-change`
- 后续：`deployment-verification`
- 子技能：`git-ops`
- Wiki：`wiki/_common/git.md` `wiki/_common/ci-cd-pipeline.md`
- 物理拦截：`_common/checks/commit-msg.sh`
