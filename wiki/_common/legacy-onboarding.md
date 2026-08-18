# 老项目接入指南

> 本文档是 `project-intake` 技能的详细参考手册。

## 何时用本流程

- ✅ 已有项目需要接入本规范
- ✅ 老项目改造 / 重构
- ✅ 老项目切换规范

**不适用**：
- ❌ 全新项目（用 `project-intake`）
- ❌ 已有项目的普通变更（用 `requirement-analysis`）

## 完整接入流程

```
第 1 步：audit（现状审计）
   产出：changes/proposals/0000-legacy-onboarding/audit-report.md
   ↓
第 2 步：migration-planning（迁移规划）
   产出：changes/proposals/0000-legacy-onboarding/proposal.md + tasks.md
   ↓
第 3 步：初始化四层结构
   安装 rules / skills / wiki / changes
   ↓
第 4 步：retro-document（可选，反向文档化）
   产出：docs/architecture/ + docs/adr/ + docs/flows/
   ↓
第 5 步：进入正常 SDLC
   新需求 → requirement-analysis
   老代码改造 → migration proposal → coding
```

## 迁移策略对比

### 冻结（Freeze）

**说明**：老代码不动，仅新代码按新规范。

**适用**：
- 稳定老项目，不再演进
- 团队资源紧张

**优点**：
- 风险最低
- 工作量最小

**缺点**：
- 老代码永远是技术债
- 双规范长期并存

### 渐进改造（Boy Scout Rule）⭐ 推荐

**说明**：接触到的老代码顺手改。

**适用**：
- 持续维护的项目
- 团队接受渐进改进

**优点**：
- 风险可控
- 工作量分散
- 不需要专门的"重构时间"

**缺点**：
- 见效慢
- 需要团队纪律

**规则**：
- ✅ 每次 PR 顺手改造接触到的代码
- ✅ 新功能 MUST 按新规范
- ❌ 不强制改造未接触的老代码

### Strangler Fig（绞杀者模式）

**说明**：新功能在新模块，老功能逐步替换。

**适用**：
- 大型重构
- 服务拆分

**优点**：
- 风险可控
- 可并行运行

**缺点**：
- 需要维护路由层
- 工作量大

### 整体重写（极少推荐）

**说明**：一次性重写。

**适用**：
- 极小项目（< 1000 行）
- 技术栈完全变更

**优点**：
- 一次性解决

**缺点**：
- 风险极高
- 业务中断
- 不推荐

## 双规范并存期约定

在迁移期间，**新代码按新规范，老代码按老规范**。

### 边界识别

| 维度 | 新代码 | 老代码 |
|---|---|---|
| 位置 | 新建模块 / 新文件 | 现有文件 |
| 分支 | feat-* 分支 | master / develop |
| Commit | feat/refactor 类型 | 不影响 |

### 规则

- ✅ **MUST** 新代码按新规范
- ✅ **MUST** 修改老代码时顺手改造（Boy Scout Rule）
- ❌ **MUST NOT** 强制改造未接触的老代码
- ❌ **MUST NOT** 在同一文件内混用两种规范

## 常见接入场景

### 场景 A：无规范老项目

**特征**：无 proposal / 无 changelog / 测试覆盖低

**流程**：完整 5 步

### 场景 B：有自己规范想切换

**特征**：已有 CONTRIBUTING.md / 自己的 commit 规范

**流程**：
1. 先做规范映射（老规范 vs 新规范）
2. 再走完整 5 步

### 场景 C：部分模块接入

**特征**：老模块保持现状，新模块按新规范

**流程**：
- 明确边界（哪些模块接入）
- 仅新模块走完整流程

### 场景 D：老项目大改造

**特征**：借 AI 之力重构

**流程**：
- 用 Strangler Fig 策略
- 新建独立模块，逐步替换

## 关键原则

1. **先审计，再决策** —— 不审计不清楚现状
2. **策略由用户定** —— AI 建议但不擅自决定
3. **渐进优先** —— 一次性重写极少推荐
4. **历史不强制** —— 不强制补历史 changelog / 文档
5. **从接入点开始** —— 从下一个版本开始记录

## 关联

- 技能：`project-intake` / `audit` / `project-intake` / `retro-document`
- Wiki：`wiki/_common/migration-strategies.md`
- 规则：`common-legacy-tolerance`
