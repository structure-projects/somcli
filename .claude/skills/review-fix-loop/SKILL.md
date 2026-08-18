---
name: review-fix-loop
description: |
  当 expert-review 产出评审报告且包含 MUST fix 项时触发。
  MUST 询问用户"是否修复"，确认后回到 coding 修复并复评。
  形成"评审 → 修复 → 复评"的完整闭环。

triggers:
  - 修复评审问题
  - 处理 review
  - 修复 MUST fix
  - 复评
  - fix review
  - 处理评审意见

role: developer
phase: review


allowed-tools: Bash, Read, Write, Edit

related-rules:
  - common-core
  - common-project-stack-detection

reads-before-action:
  - changes/proposals/<current>/review.md

produces:
  - 修复后的代码
  - 更新的 review.md（标记已修复项）

requires:
  - skill: expert-review
    condition: changes/proposals/<current>/review.md exists with MUST fix items
    error: 无评审报告或无 MUST fix 项，无需进入本技能

human-in-the-loop:
  - MUST 询问用户"是否修复 MUST fix 项"
  - 用户可全部修复 / 部分修复 / 跳过（跳过需说明理由）

on-failure: |
  修复引入新问题 → 再次触发本技能
  用户选择跳过 → 记录理由到 review.md，允许继续（但有技术债标记）

mode: auto

category: review
stack: _common
priority: high
---

# 评审修复循环

> 形成"评审 → 修复 → 复评"的完整闭环。**禁止只生成报告不处理**。

## 前置条件

1. `changes/proposals/<current>/review.md` 存在
2. review.md 中含未勾选的 MUST fix 项

## 执行步骤

### 第 1 步：读评审报告

```bash
cat changes/proposals/<current>/review.md
```

列出所有 MUST fix 项，编号：

```
MUST fix 项清单：
1. [位置] 问题描述 → 建议修复方式
2. [位置] 问题描述 → 建议修复方式
...
```

### 第 2 步：询问用户处理决策（MUST）

**禁止默认全部修复**。MUST 询问用户：

```
发现 N 个 MUST fix 项，请选择处理方式：

A. 全部修复（推荐）
B. 部分修复（请指定修复哪几项）
C. 全部跳过（需说明理由，记录技术债）
D. 让我看看具体每一项再决定

请选择：
```

### 第 3 步：按用户决策执行

#### 用户选 A（全部修复）

对每一项 MUST fix：
1. 定位到对应代码
2. 调用 `coding` 技能修复
3. 在 review.md 中标记该项为 `- [x]`（已修复）

#### 用户选 B（部分修复）

让用户指定要修复的项，逐项处理。未修复的项 MUST 在 review.md 中标记 `- [ ] 暂不修复（原因：...）`。

#### 用户选 C（全部跳过）

MUST 要求用户提供理由，记录到 review.md：

```markdown
## 技术债说明

本次评审的 MUST fix 项未修复，理由：<用户提供>。
风险：<AI 评估的潜在风险>。
建议后续处理时间：<建议>。
```

### 第 4 步：复评

修复完成后 MUST 调用 `expert-review` 复评：
- 检查修复是否正确
- 检查是否引入新问题
- 更新 review.md（标记复评结果）

### 第 5 步：循环直到通过

如果复评仍有 MUST fix → 回到第 1 步，直到：
- 所有 MUST fix 都已修复
- 或用户明确跳过（有理由记录）

## 产出物

- 修复后的代码
- 更新的 review.md（含修复记录 / 跳过理由）
- 复评通过

## 完成标准

- 所有 MUST fix 项均已修复（或有明确跳过理由）
- 复评通过
- review.md 含完整的修复/跳过记录

## 下一步

进入 `ci-gate` 提交。

## 关联

- 前置：`expert-review`
- 中途：`coding`（修复时调用）
- 后续：`expert-review`（复评）+ `ci-gate`
