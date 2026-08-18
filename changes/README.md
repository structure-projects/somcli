# Changes 变更管理

> 本目录管理本项目的所有变更。任何编码动作 MUST 先在本目录建立对应的变更提案。

## 目录结构

```
changes/
├── README.md                          # 本文件
├── templates/                         # 模板（按变更级别分级）
│   ├── proposal-full.md               # major：完整变更提案
│   ├── proposal-simple.md             # minor：简化变更提案（5 字段）
│   ├── proposal-hotfix.md             # hotfix：极简变更提案（5 字段 + 事后补全清单）
│   ├── proposal-migration.md          # 迁移类变更（老项目接入用）
│   ├── design.md                      # 设计文档模板（复杂需求必填）
│   ├── tasks.md                       # 任务清单模板
│   ├── changelog-entry.md             # changelog 条目模板
│   ├── audit-report.md                # 现状审计报告模板（老项目接入用）
│   └── retrospective.md               # 复盘文档模板（hotfix 事后必补）
├── proposals/                         # 进行中的变更
├── archive/                           # 已完成的变更
└── changelog/                         # 变更日志（按版本）
```

## 变更分级

| 级别 | 触发场景 | 流程 | 文档 |
|---|---|---|---|
| **trivial** | typo、文档、格式、注释、配置微调 | 直接改 + changelog | 仅 changelog 条目 |
| **minor** | 小功能调整、简单 bug 修复 | 简化流程 | `proposal-simple.md`（5 字段）+ `tasks.md` |
| **major** | 新功能、架构调整、多模块改动 | 完整 SDLC | `proposal-full.md` + `design.md` + `tasks.md` |
| **hotfix** | 生产紧急修复 | 快速通道 | `proposal-hotfix.md` + 事后 24h 补全 + `retrospective.md` |
| **migration** | 老项目改造、重构 | 完整 SDLC | `proposal-migration.md` + `audit-report.md` |

## 工作流程

### Major / Minor 变更

```
需求 → 创建 changes/proposals/<id>/
   ↓ 复制对应级别模板
   ↓ 填充 proposal.md + tasks.md
   ↓ 创建 feat-<name> / fix-<name> 分支
编码 → 按 tasks.md 逐项实现
   ↓ 完成一项勾选一项
   ↓ 每项同步写单测
评审 → 调用 expert-review，产出 review.md
   ↓ 修复 MUST fix 项
提交 → 调用 ci-gate，物理拦截通过
部署 → 调用 deployment-verification
归档 → git mv changes/proposals/<id>/ changes/archive/
   ↓ 更新 changelog/<version>.md
```

### Hotfix 变更

```
生产问题 → 创建 changes/proposals/YYYYMMDD-hotfix-<name>/
   ↓ 填充 proposal-hotfix.md（5 字段）
   ↓ 创建 hotfix-<version> 分支
修复 + 测试 + 评审（并行）
   ↓
快速 CI（仅 MUST 检查：lint + 编译 + 核心单测）
   ↓
部署验证
   ↓
事后 24h 内强制补全：
   - 完整 proposal（复制 proposal-full.md 重写）
   - retrospective.md（复盘）
   - 完整 changelog
   - 合并回 master + develop
```

### Trivial 变更

```
直接修改 → 写 changelog 条目 → 提交
```

## 变更 ID 规范

格式：`YYYYMMDD-<kebab-case-name>`

| 类型 | 示例 |
|---|---|
| 功能 | `2026-08-15-add-user-login` |
| 修复 | `2026-08-15-fix-login-timeout` |
| Hotfix | `2026-08-15-hotfix-session-leak` |
| 迁移 | `2026-08-15-migrate-to-ddd` |
| 接入 | `0000-legacy-onboarding`（老项目接入专用） |

## 关键约束

- **MUST** 任何编码动作 MUST 先在本目录建立对应的变更提案
- **MUST** 提案 ID MUST 唯一，按日期 + 名称生成
- **MUST** 完成后 MUST 归档到 `archive/` 并更新 changelog
- **MUST NOT** 在 `proposals/` 目录累积已完成变更
- **MUST NOT** 跳过 proposal 直接编码（trivial 除外）
- **MUST NOT** hotfix 跳过事后复盘
