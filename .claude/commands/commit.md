---
name: commit
description: |
  执行 CI 门禁检查并按 Conventional Commits 规范提交代码。
  当用户请求 /commit 或 /ci 时触发。
  ⚠️ 执行顺序：先 ci-gate（门禁检查）→ 再 git-commit（生成 message + 提交）
category: git
phase: ci
triggers:
  - commit
  - ci
related-skills:
  - ci-gate
  - git-ops
related-rules:
  - common-core
stack: _common
priority: high
---

# 提交代码（Commit）

> 原 `/ci` 已并入本指令（二者 `related-skills` 与 `phase` 完全相同），`ci` 保留为别名。

## 目的

作为提交代码的物理门禁：执行本地预检 + 生成合规 commit message + 推送 + 监控远程 CI，确保只有合格的代码进入主干。

## 使用场景

- 编码 + 测试 + 评审完成后准备提交
- 需要生成规范化的 commit message
- 需要触发远程 CI 验证

## 参数

| 参数 | 含义 |
|---|---|
| 无 | 完整门禁 + 提交 + 推送 |
| `--no-push` | 只提交到本地，不推送 |

## 委派

```
ci-gate（门禁检查）→ git-commit（生成 message + 提交）→ ci-gate（推送 + 监控远程 CI）
```

→ 调用 `ci-gate`，随后 `git-ops`，**步骤、分级检查（MUST / SHOULD）与输出格式均见该技能**。

**核心约束**：ci-gate 是 git-commit 的前置门禁，不可跳过。

## 非阻塞模式行为

当 `changes/config.yaml` 中 `trust-level: autonomous` + `non-blocking-confirm.enabled: true` 时：

- ✅ 无 staged 文件：自动 `git add -A`
- ✅ 推送到 feat-* 分支：自动执行
- ⚠️ 推送到 master：排队等待人工审查（L0，不可自动）
- ⚠️ 推送到 develop：需确认（L1；autonomous 下自动执行并写入 audit.md 待 24h 复核）
- ⚠️ force push：排队等待人工审查
- ⚠️ 结束时汇报待处理队列

## 关联

- Skill：`_common/skills/ci-gate/SKILL.md`（门禁步骤 + 分级检查 + 推送分级）
- Skill：`_common/skills/git-ops/SKILL.md`（message 生成规范）
- Wiki：`_common/wiki/git.md`、`_common/wiki/ci-cd-pipeline.md`
- Rule：`_common/rules/common-core.mdc`
