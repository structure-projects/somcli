---
name: project-intake
description: |
  当用户要求"初始化项目/新建项目/搭建项目/scaffold/创建工程"、"接入老项目/老项目改造/现状审计"
  或"制定迁移计划/迁移策略"时触发。
  项目接入全流程：新项目脚手架（形态决策 → 目录 → 关键文件）| 老项目接入（审计 → 迁移规划 → 进入 SDLC）。
  项目形态与迁移策略 MUST 询问用户，MUST NOT 默认。

triggers:
  - 初始化项目
  - 新建项目
  - 搭建项目
  - 创建工程
  - scaffold
  - init project
  - 项目初始化
  - 新项目
  - 脚手架
  - 接入老项目
  - 老项目改造
  - 项目迁移
  - 规范接入
  - 现状审计
  - 老项目
  - legacy
  - onboarding
  - 重构
  - 制定迁移计划
  - 规划迁移
  - 迁移策略
  - migration planning
  - 迁移计划

role: architect
phase: requirement

allowed-tools: Bash, Read, Write, Edit, Glob, Grep

related-rules:
  - common-core
  - common-project-structure
  - common-legacy-tolerance
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/project-structure.md
  - wiki/_common/architecture.md
  - wiki/_common/project-form-decision.md
  - wiki/_common/legacy-onboarding.md
  - wiki/_common/migration-strategies.md
  - wiki/<stack>/project-scaffolding.md
  - wiki/<stack>/components.md

stack-constraints:
  structure-boot:
    spring-boot-version: "4.0.6"
    jdk: "17+"
    parent: "cn.structured:structure-dependencies:1.4.4"
    required-components:
      - structure-security
      - structure-infra
      - structure-restful-web-starter
    project-form:
      default: "DDD 7+1 多模块"
      alternatives: ["单体 4 模块 + Manager 模式"]
      must-ask-user: true
  vue3:
    required-components:
      - "@structure-projects/components"
      - "@structure-projects/wujie-subapp"
    project-form:
      default: "wujie 微前端子应用"
      alternatives: ["独立前端项目"]
      must-ask-user: true

produces:
  - 新项目：完整目录结构 + pom.xml / package.json + README.md + .gitignore
  - 新项目：changes/proposals/0001-init-project/（初始化提案）
  - 老项目：changes/proposals/0000-legacy-onboarding/audit-report.md
  - 老项目：changes/proposals/0000-legacy-onboarding/proposal.md + tasks.md
  - （可选）架构文档 / ADR

requires:
  - skill: audit
    condition: 老项目接入（第二章）时 audit-report.md 存在
    error: 未完成现状审计，MUST 先调用 audit

human-in-the-loop:
  - id: confirm-project-form
    action: 项目形态（DDD 7+1 / 单体 4 模块 / 单体单模块）MUST 询问用户，MUST NOT 默认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 形态选错需重建目录，代价高，autonomous 下 MUST 写入 audit.md 待复核
  - id: confirm-project-identity
    action: 项目名 / groupId / packageName 与技术栈版本 MUST 询问用户
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 首次提交前可改；已提交需全局重命名
  - id: confirm-migration-strategy
    action: 迁移策略（冻结 / 渐进改造 / Strangler Fig / 整体重写）与改造范围、阶段规划 MUST 用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 策略仅写入 proposal.md，动工前可改

on-failure: |
  用户未明确形态 → MUST 追问，MUST NOT 默认
  依赖版本冲突 → 列出冲突项，让用户选择
  代码扫描失败 → 引导用户手动配置
  迁移策略冲突 → 列出对比，让用户选择

category: requirement
stack: _common
priority: high
---

# 项目接入（新建 / 老项目）

> 新项目从零搭建规范结构；老项目按 **审计 → 迁移规划 → 正常 SDLC** 接入。
> **项目形态与迁移策略 MUST 询问用户，MUST NOT 默认**。

## 双流程区分

| 项目类型 | 走哪章 |
|---|---|
| **全新项目** | 第一章（脚手架） |
| **老项目接入** | 第二章（审计 + 迁移规划） |
| **已有项目普通变更** | 不用本技能，直接 `requirement-analysis` |

---

# 一、新项目脚手架

前置：用户明确要新建项目，且已识别项目栈（`common-project-stack-detection`）。

## 第 1 步：询问用户关键决策（MUST）

**MUST NOT 默认**，按以下决策树引导用户：

### 1.1 项目形态决策树 ⭐

```
Q1: 项目涉及几个业务领域（聚合根）？
    例如：只有"用户" → 1 个；有"用户+订单+商品" → 多个
    ↓
    ├─ 仅 1 个聚合根 + 简单 CRUD → **单体单模块**（工具类项目）
    ↓
    多个聚合根 或 复杂业务 → 继续 Q2

Q2: 团队规模和预期演进周期？
    ↓
    ├─ 1 人 + 短期（< 3 个月） → **单体单模块**
    ├─ 1-3 人 + 中期（3-12 个月） → **单体 4 模块**
    ↓
    ≥ 3 人 或 长期（≥ 1 年） → 继续 Q3

Q3: 是否需要严格的层间隔离（应用层不接触 Mapper）？
    ↓
    ├─ 否 → **单体 4 模块**（用 Manager 模式）
    ↓
    是 → 继续 Q4

Q4: 是否预期拆分为微服务？
    ↓
    ├─ 否 → **单体 4 模块**
    ↓
    是 → **DDD 7+1 多模块** ⭐
```

**MUST 告诉用户推荐结果的理由**，并指向 `wiki/_common/project-form-decision.md` 的详细判断标准，再请用户确认或改选。

### 1.2 项目信息

- 项目名（如 structure-user）
- groupId（如 cn.structured）
- 主包名（如 cn.structured.user）

### 1.3 技术栈版本

MUST 按 stack-constraints 确认（Spring Boot 4.0.6 / JDK 17+ / …）。

## 第 2 步：读栈级脚手架 Wiki

```bash
cat wiki/<stack>/project-scaffolding.md
cat wiki/<stack>/components.md
cat wiki/_common/project-form-decision.md
```

## 第 3 步：生成项目结构

### DDD 7+1 多模块（默认推荐）

```
structure-{X}/
├── structure-{X}-dependencies/        # 父 POM
├── structure-{X}-common/              # DTO / VO / Query / enums / exception
├── structure-{X}-domain/              # {X}Entity、{X}Repository（接口）、DomainService
├── structure-{X}-infra/               # {X}RepositoryImpl、{X}RepositoryDelegate
├── structure-{X}-repository-mybatis/  # {X}PO、{X}Mapper、{X}MybatisPlusDelegate
├── structure-{X}-application/         # I{X}Service、{X}ServiceImpl、{X}Assembler
├── structure-{X}-interfaces/          # controller/api/ + controller/open/
└── structure-{X}-boot/                # 启动类 + application.yaml
```

### 单体 4 模块（备选）

```
structure-{X}/
├── {X}-api/           # 接口定义
├── {X}-biz/           # 业务实现
├── {X}-common/        # 通用类
└── {X}-dependencies/  # 父 POM
```

## 第 4 步：生成关键文件

- 根 `pom.xml` 或 `dependencies/pom.xml`：parent = `cn.structured:structure-dependencies:1.4.4`
- 各模块 `pom.xml`
- 启动类（含必要注解）+ `application.yaml`（含栈级必选配置）
- `.gitignore`
- `README.md`：项目简介 + 技术栈（含版本号）+ 模块结构图 + 快速开始 + 必选组件清单 + 开发规范链接（指向 `wiki/`）

## 第 5 步：初始化 Changes 目录

```bash
mkdir -p changes/proposals/0001-init-project
cp changes/templates/proposal-full.md changes/proposals/0001-init-project/proposal.md
```

## 第 6 步：初始化 git

```bash
git init
git add .
git commit -m "feat(init): 初始化项目结构（DDD 7+1 多模块）"
```

---

# 二、老项目接入（Legacy Onboarding）

前置：已有项目（含源代码 + git 历史），未接入本规范。
流程：**audit → 迁移规划 → retro-document（可选）→ 正常 SDLC**。

## 第 1 步：现状审计（调用 `audit`）

扫描代码结构、规范符合性（命名 / 分支 / commit / 架构分层 / 异常 / 日志 / API / 安全）、测试覆盖率、CI/CD、文档完整度。

**产出**：`changes/proposals/0000-legacy-onboarding/audit-report.md`

## 第 2 步：确定改造范围（MUST 用户确认）

```
Q1: 改造范围？
    a) 全部（一次性迁移）—— 风险高，仅小项目
    b) 部分（仅新代码按新规范）—— 风险低
    c) 渐进（接触到的老代码顺手改）—— 推荐 ⭐
```

## 第 3 步：选择迁移策略（MUST 用户确认）

| 策略 | 说明 | 适用 |
|---|---|---|
| **冻结** | 老代码不动，只新代码按新规范 | 稳定老项目，不演进 |
| **渐进改造** ⭐ | 接触到的老代码顺手改（Boy Scout Rule） | 持续维护的项目 |
| **Strangler Fig** | 新功能在新模块，老功能逐步替换 | 大型重构 / 服务拆分 |
| **整体重写** | 一次性重写 | 极少推荐（风险极高） |

## 第 4 步：制定阶段规划

```
M1：基础规范接入（1 周）
  - 安装 rules / skills / wiki / changes
  - 配置 commit-msg hook
  - 建立 CI 基础

M2：核心模块改造（2 周）
  - 按优先级改造核心模块
  - 补充关键测试

M3：边缘模块改造（按需）
  - 剩余模块 + 补充文档
```

## 第 5 步：评估风险

| 风险 | 影响 | 缓解 |
|---|---|---|
| 老代码改造引入 bug | 高 | 渐进改造 + 完整测试 |
| 双规范并存期混乱 | 中 | 明确边界（新代码 vs 老代码） |
| 团队学习成本 | 中 | 培训 + 文档 + 示例 |
| 进度延误 | 中 | 阶段拆分 + 每周回顾 |

## 第 6 步：产出迁移提案

写入 `changes/proposals/0000-legacy-onboarding/proposal.md`（并生成 `tasks.md`）：

```markdown
# 迁移变更提案：老项目接入

## 现状
<来自 audit-report>

## 目标状态
<接入本规范后的样子>

## 迁移策略
<冻结 / 渐进改造 / Strangler Fig / 整体重写>

## 阶段规划
| 里程碑 | 范围 | 完成标准 |

## 风险评估
## 回滚预案
## 兼容性保证
## 双规范并存期约定
```

## 第 7 步：初始化四层结构

```bash
./install.sh -t <project> -s <stack> -w <tools> -c
mkdir -p changes/proposals/0000-legacy-onboarding
```

## 第 8 步：（可选）反向文档化（调用 `retro-document`）

为核心模块反向生成架构文档（C4）、关键决策 ADR、主要流程时序图 → `docs/architecture/` 或 `docs/adr/`。

## 老项目关键约束

- ✅ **MUST** 先做现状审计（`audit`）
- ✅ **MUST** 迁移策略经用户确认
- ✅ **MUST** 从接入点开始记 changelog（不强制补历史）
- ❌ **MUST NOT** 大面积重写老代码（应用渐进改造）
- ❌ **MUST NOT** 强制老代码立即补测试（新改动必须带测试）

---

## 完成标准

- 新项目：形态经用户确认、目录就位、`mvn clean compile` 通过、README 完整、首次提交完成
- 老项目：`audit-report.md` 完成、迁移提案经用户确认、四层结构初始化完成
- 双方均已进入正常 SDLC

## 下一步

- 新项目 → `requirement-analysis`（开始第一个需求）
- 老项目 → （可选）`retro-document` → `requirement-analysis`
- 需要架构设计 → `high-level-design`

## 关联

- 子技能：`audit`（现状审计）/ `retro-document`（反向文档，可选）
- 后续：`requirement-analysis` / `high-level-design`
- Wiki：`wiki/_common/project-structure.md`、`wiki/_common/project-form-decision.md`、`wiki/_common/legacy-onboarding.md`、`wiki/_common/migration-strategies.md`
- 规则：`common-project-structure`、`common-project-stack-detection`、`common-legacy-tolerance`
