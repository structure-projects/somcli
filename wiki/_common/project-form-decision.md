# 项目形态决策指南

> 本文档是 `project-intake` 技能的详细参考。当你需要决定"用 DDD 7+1 还是单体"时，MUST 读本文档。

> 📌 **栈归属说明**：本文档的**示例代码**以 **structure-boot** 栈为主（含具体类名如 `ResResultVO` / `DataScopeRedisTemplate` / `structure-security` 等）。
> **规则部分**（MUST / MUST NOT）适用于所有技术栈。
> 其他栈的使用者请参考对应栈级 Wiki（`wiki/<stack>/developer.md`）获取具体类名与组件。

## 三种项目形态对比

| 维度 | **DDD 7+1 多模块** | **单体 4 模块** | **单体单模块** |
|---|---|---|---|
| **业务复杂度** | 多 bounded context（≥ 2 个聚合根） | 单一业务领域 | 工具类 / 简单 CRUD |
| **聚合根数量** | ≥ 3 个 | 1-2 个 | 1 个 |
| **领域逻辑** | 复杂（DomainService / 事件 / CQRS） | 适中 | 简单 |
| **团队规模** | ≥ 3 人 | 1-3 人 | 1 人 |
| **演进周期** | ≥ 1 年长期演进 | 中期（3-12 月） | 短期 demo / 工具 |
| **微服务拆分** | 预期拆为微服务 | 单体即可 | 单体即可 |
| **层间隔离** | 严格（application 不接触 Mapper） | 一般（Manager 直接调用 Mapper） | 不需要 |
| **依赖组件** | 需要 `structure-infra` 的 `RepositoryFacade`/Delegate | 用 Manager 模式（`IManager` / `ManagerImpl`） | 不需要 |
| **测试要求** | 完整领域测试（Domain + Application + Infra） | 业务测试为主 | 简单单测 |
| **改造成本** | 高（需完整设计） | 中 | 低 |

## 快速决策树

```
开始新项目
   ↓
Q1: 涉及多个业务领域（≥ 2 个聚合根）吗？
   ├─ 否 → 单体单模块（工具类项目）
   ↓ 是
Q2: 团队规模 ≥ 3 人 或 预期演进 ≥ 1 年？
   ├─ 否 → 单体 4 模块
   ↓ 是
Q3: 需要严格层间隔离（应用层不接触 Mapper）？
   ├─ 否 → 单体 4 模块
   ↓ 是
Q4: 预期拆分为微服务 或 长期演进？
   ├─ 否 → 单体 4 模块
   └─ 是 → **DDD 7+1 多模块** ⭐
```

## 一句话判断

> **"如果你不确定用不用 7+1，那就不要用。"**
>
> 7+1 是给"确定会复杂"的项目用的。小项目用 7+1 是过度设计，反而拖慢节奏。

## 真实案例对照

### 应该使用 DDD 7+1

| 项目 | 原因 |
|---|---|
| `structure-user` | 用户 / 组织 / 角色 / 权限多 bounded context，长期演进 |
| `structure-org` | 多部门 / 多层级 / 数据权限 |
| `structure-tenant` | 多租户 / 套餐 / 计费，业务复杂 |
| `structure-order` | 订单 / 订单项 / 支付 / 物流多聚合根 |

### 应该使用单体 4 模块

| 项目 | 原因 |
|---|---|
| `structure-pro` | 云原生脚手架，整合多组件但本身业务简单 |
| `structure-mono-template` | 老项目模板，中等复杂度 |
| 内部管理系统 | 1-3 人团队，单一业务领域 |

### 应该使用单体单模块

| 项目 | 原因 |
|---|---|
| 内部报表工具 | 只读不写，无业务逻辑 |
| demo / poc 项目 | 演示用，1 人 1-3 个月 |
| 简单脚本工具 | 单文件可完成 |
| 学习项目 | 验证概念 |

## DDD 7+1 详细结构

```
structure-{X}/
├── structure-{X}-dependencies/        # 父 POM
├── structure-{X}-common/              # DTO / VO / Query / enums / exception
├── structure-{X}-domain/              # Entity / Repository 接口 / DomainService
├── structure-{X}-infra/               # RepositoryImpl / RepositoryDelegate
├── structure-{X}-repository-mybatis/  # PO / Mapper / MybatisPlusDelegate / Flyway
├── structure-{X}-application/         # I{X}Service / {X}ServiceImpl / {X}Assembler
├── structure-{X}-interfaces/          # Controller（api/ + open/）
└── structure-{X}-boot/                # 启动类 + application.yaml
```

### 模块职责

| 模块 | 职责 | 依赖 |
|---|---|---|
| **dependencies** | 父 POM，统一管理依赖版本 | 无 |
| **common** | DTO / VO / Query / 枚举 / 异常 | 无 |
| **domain** | 领域实体 + 仓储接口 + 领域服务 | common |
| **infra** | 仓储实现（防腐层） | domain + common |
| **repository-mybatis** | MyBatis 持久化（PO / Mapper） | domain + common |
| **application** | 应用服务（编排领域服务） | domain + infra |
| **interfaces** | 控制器（HTTP / Feign） | application |
| **boot** | 启动类 + 配置 | all |

### 依赖方向（MUST 遵守）

```
common → domain → infra → repository-mybatis
                     ↑
application → domain + infra
interfaces → application
boot → all
```

**禁止反向依赖**：
- ❌ domain → infra
- ❌ domain → application
- ❌ interfaces → domain

## 单体 4 模块详细结构

```
structure-{X}/
├── {X}-api/           # 接口定义（DTO / VO / Feign 客户端）
├── {X}-biz/           # 业务实现（Service / Manager）
├── {X}-common/        # 通用类（Utils / Constants / Enums）
└── {X}-dependencies/  # 父 POM
```

### 模块职责

| 模块 | 职责 |
|---|---|
| **api** | 接口定义、DTO、VO、Feign 客户端 |
| **biz** | 业务实现、Manager、Service |
| **common** | 工具类、常量、枚举 |
| **dependencies** | 父 POM |

### 关键区别（vs DDD 7+1）

- 用 `IManager` / `ManagerImpl` 替代 `I{X}Service` / `{X}ServiceImpl`
- Entity 直接用 `@TableId` / `@TableLogic`（**不区分 Entity / PO**）
- Manager 可直接访问 Mapper（**无 Delegate 防腐层**）

## 单体单模块适用场景

仅当满足**全部**以下条件：

- ✅ 团队 1 人
- ✅ 预期演进 ≤ 3 个月
- ✅ 仅 1 个聚合根
- ✅ 简单 CRUD（无复杂业务逻辑）
- ✅ 不拆分为微服务

**任一不满足 → 至少用单体 4 模块**。

## 形态迁移成本

### 单体单模块 → 单体 4 模块

- **工作量**：1-2 天
- **改动**：拆分模块、调整包名
- **风险**：低

### 单体 4 模块 → DDD 7+1

- **工作量**：1-2 周
- **改动**：
  - 拆分 Entity / PO
  - 引入 `RepositoryFacade` / Delegate
  - 重写 Service 层为 Application 层
  - 调整测试
- **风险**：中（需要完整回归测试）

### 单体 → 微服务

- **工作量**：数月
- **改动**：服务拆分、引入注册中心、分布式事务、链路追踪
- **风险**：高

**建议**：**形态决策 MUST 在项目初期确定**，后期迁移成本高。

## 常见误用

### ❌ 小项目过度设计

```
场景：1 人 1 个月的报表工具
错误：用 DDD 7+1
正确：单体单模块
```

### ❌ 大项目简陋设计

```
场景：3 人团队的业务中心，预期 2 年演进
错误：用单体单模块
正确：DDD 7+1
```

### ❌ 为"未来可能"用 DDD

```
场景：demo 项目，"以后可能扩展"
错误：用 DDD 7+1（"以后可能"不是理由）
正确：单体单模块
```

**原则**：**按当前确定性需求选型，不为"可能"过度设计**。

## 相关文档

- `wiki/_common/architecture.md` —— 分层架构原则
- `wiki/_common/project-structure.md` —— 项目结构约定
- `wiki/structure-boot/architect.md` —— structure-boot 架构决策
- `_common/skills/project-intake/SKILL.md` —— 项目初始化技能
- `_common/skills/high-level-design/SKILL.md` —— 模块拆分技能
