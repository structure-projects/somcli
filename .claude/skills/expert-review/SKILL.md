---
name: expert-review
description: |
  当用户要求"评审/看一下/检查/review/CR/审查代码"时触发。
  MUST 对照变更提案评审代码，产出评审报告。
  AI 自检 ≠ 专家评审：关键项目 MUST 引入人类评审。

triggers:
  - 评审
  - 看一下
  - 检查
  - 审查
  - review
  - CR
  - code review
  - 查一下

role: reviewer
phase: review


allowed-tools: Bash, Read, Grep, Glob

related-rules:
  - common-core
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/code-review-checklist.md
  - wiki/_common/security.md
  # 栈级规范（MUST 根据识别的栈动态替换 <stack>）
  - wiki/<stack>/developer.md
  - wiki/<stack>/components.md

produces:
  - changes/proposals/<current>/review.md

requires:
  - skill: coding
    condition: changes/proposals/<current>/tasks.md all checked
    error: 编码未完成，MUST 先完成 coding 技能

human-in-the-loop:
  - id: mark-ai-self-review
    action: AI 评审 MUST 标注"AI 自检"
    level: L2-always-auto
    autonomous:
      behavior: auto-pass
      audit: false
    rollback: 自动打标（review.md 含 AI 自检前缀）
  - id: require-human-review-critical
    action: 关键项目（涉钱/涉安全/涉合规）MUST 引入人类评审
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（关键项目人类评审不可跳过）
  - id: block-unresolved-must-fix
    action: MUST fix 项不解决 MUST NOT 提交
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（质量红线，必须修复）

on-failure: |
  发现 MUST fix 问题 → 回到 coding 修复后复评
  发现 proposal 不匹配 → 回到 requirement-analysis

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

category: review
stack: _common
priority: high
---

# 专家评审

> 对照变更提案评审代码，产出评审报告。
> ⚠️ **AI 自检 ≠ 专家评审**：关键项目 MUST 引入人类评审。

## 前置条件（MUST 全部满足）

1. **编码已完成**：`changes/proposals/<current>/tasks.md` 所有任务勾选
2. **proposal 存在**：`changes/proposals/<current>/proposal.md` 存在

## 评审维度（MUST 逐项检查）

| 维度 | 检查点 | 通过标准 |
|---|---|---|
| **符合性** | 代码是否实现 proposal 所有目标 | 目标 100% 覆盖 |
| **规范性** | 是否遵守 naming / architecture / 栈 rules | 无 MUST 违反 |
| **测试覆盖** | 关键路径是否有测试 | 行覆盖 ≥ 80%，关键路径 100% |
| **安全性** | SQL 注入 / XSS / 越权 / 敏感信息泄露 | 无 MUST 风险 |
| **性能** | N+1 / 慢查询 / 内存泄漏 / 大数据量 | 无 P0 问题 |
| **可读性** | 命名清晰 / 函数简短 / 注释充分 | 新人可读懂 |

## 严重等级分类

| 等级 | 说明 | 处理 |
|---|---|---|
| **MUST fix** | 违反红线、有安全/数据风险、不符合 proposal | 不解决 MUST NOT 提交 |
| **SHOULD fix** | 不规范但不影响功能 | 建议修复；不修复需说明理由 |
| **NIT** | 风格、个人偏好 | 可选修复 |

## 执行步骤

### 第 1 步：读变更提案

```bash
cat changes/proposals/<current>/proposal.md
cat changes/proposals/<current>/design.md  # 如有
```

### 第 2 步：读 diff

```bash
git diff develop...HEAD
# 或
git diff master...HEAD  # hotfix
```

### 第 3 步：按维度逐项评审（TODO 阶段 3 填充详细 checklist）

### 第 4 步：产出评审报告

写入 `changes/proposals/<current>/review.md`，格式：

```markdown
# 评审报告：<提案 ID>

| 字段 | 值 |
|---|---|
| 评审日期 | YYYY-MM-DD |
| 评审人 | <AI / 用户> |
| 结论 | ✅ 通过 / ⚠️ 有条件通过 / ❌ 不通过 |

## MUST fix（必须修复）

- [ ] <问题 1 + 位置 + 建议>

## SHOULD fix（建议修复）

- [ ] <问题 1 + 位置 + 建议>

## NIT（可选）

- [ ] <...>

## 评审意见

<总体评价 + 是否建议合并>
```

## 产出物

- `changes/proposals/<current>/review.md`

## 完成标准

- 所有维度都评审过
- MUST fix 项已修复或明确不修复理由
- review.md 写入提案目录

## 下一步

- 有 MUST fix → 回到 `coding` 修复后复评
- 无 MUST fix → 进入 `ci-gate` 提交

## 关联

- 前置：`coding`
- 后续：`ci-gate`
- Wiki：`wiki/_common/code-review-checklist.md` `wiki/_common/security.md`
