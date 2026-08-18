# 任务清单：<标题>

> 与 `proposal.md` 同目录。编码时逐项完成。
> 每完成一项 MUST 立即勾选 + 写对应单测。

## 准备

- [ ] 创建分支 `feat-<name>` 或 `fix-<name>`
- [ ] 阅读 `proposal.md` 与（如有）`design.md`
- [ ] 阅读相关 Wiki（按 proposal 中的引用）

## 编码（按任务顺序完成）

- [ ] 任务 1：<描述。包含验收标准>
- [ ] 任务 2：<描述>
- [ ] 任务 3：<描述>

## 测试

- [ ] 单元测试覆盖率 ≥ 80%
- [ ] 集成测试通过
- [ ] 本地 `mvn clean test` / `npm test` / `pytest` 全通过

## 评审

- [ ] 通过 expert-review（产出 `review.md`）
- [ ] 修复所有 MUST fix 项
- [ ] SHOULD fix 项已评估（不修复需说明理由）

## 归档（MUST 在提交/推送前完成）

- [ ] `git mv changes/proposals/<id>/ changes/archive/`
- [ ] 更新 `changelog/<version>.md`
- [ ] 更新 README（如有需要）

## 提交与推送

- [ ] 通过 ci-gate（归档 + commit-msg + 编译 + 核心单测）
- [ ] commit message 符合 Conventional Commits
- [ ] 分支正确（`feat-*` / `fix-*` / `hotfix-*`）
- [ ] 推送 feat-* 分支（经确认，见 ci-gate `push-feat-remote`）

## 部署（可选，仅业务项目）

- [ ] 通过 deployment-verification
- [ ] 健康检查通过
- [ ] 监控指标正常
