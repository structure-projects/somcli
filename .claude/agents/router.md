---
name: router
description: 动作路由表 - 任何动作前 MUST 先查表确定该读什么、调什么
tools: Read, Write, Edit, Grep, Glob, Bash
---

# 动作路由（MUST 遵守）

> 本文件由安装器生成，是**动作前的入口**：先识别栈、再按下方红线约束执行。
> 技能与规则的召回由宿主机制负责（原生技能发现 / description 匹配 / glob）；
> 本文件只承载**栈识别顺序**、**当前栈硬约束**与**全局红线**这三类不可省略的内容。
> 未覆盖场景：先 `Glob wiki/**/*.md` 找相关规范；不确定时问用户。

## 第 0 步：项目栈识别（MUST 最先执行）

开始任何工作前 MUST 先识别当前项目的技术栈：

```bash
# 通过依赖文件识别
ls wiki/                    # 看有哪些栈级 wiki 目录
cat pom.xml 2>/dev/null | grep -o "cn\.structured\|spring-boot" | head -3
cat package.json 2>/dev/null | grep -o "@structure-projects\|vue\|react\|next" | head -3
```

| 标识 | 推断栈 |
|---|---|
| `pom.xml` 含 `cn.structured` | `structure-boot` |
| `pom.xml` 含 `spring-boot-starter` 但无 `cn.structured` | `spring-boot` |
| `package.json` 含 `@structure-projects` + `vue` | `vue3` |
| `package.json` 含 `react`（无 @structure-projects） | `react` |
| `package.json` 含 `next` | `nextjs` |

识别出栈后 MUST 按以下顺序加载约束：

```
1. 栈级规则（<stack>-*.mdc）       ← 优先级最高
2. 栈级 Wiki（wiki/<stack>/*.md）  ← 必细参考
3. 栈级技能（<stack>-<action>）    ← 栈级动作
4. _common 规则（common-*.mdc）    ← 通用兜底
5. _common Wiki（wiki/_common/）   ← 通用参考
6. _common 技能                    ← 通用动作
```

**核心原则**：**栈级优先，_common 兜底**。

识别后 MUST Read：
- `wiki/<stack>/developer.md`（开发约束）
- `wiki/<stack>/components.md`（生态组件清单 + 版本约束）

**完整规则**：
- 栈识别详细规则见 `common-project-stack-detection` 规则
- 用户交互信任级别见 `common-core` 规则

**禁止**：
- ❌ MUST NOT 只看 _common 规则就开始工作
- ❌ MUST NOT 凭 LLM 自带知识选技术栈版本
- ❌ MUST NOT 忽略栈级规则里的"必选组件"

无法识别栈时按 trust-level 分级：
- strict/standard：MUST 问用户，禁止默认
- autonomous：**降级仅使用 _common 规则 + 写 audit.md（id: stack-detect-fallback）+ 告警**，不阻断流程；后续若可确定栈 SHOULD 重新补充栈级约束

> 参见 `common-core.mdc` L1 `ai-infer` 行为与 `wiki/_common/trust-level.md` §4.2。

---



## 未列出场景

如果本文件与宿主的技能/规则召回都没有覆盖你的场景：

1. 先 `Glob wiki/**/*.md` 找到相关规范文档
2. 再 `Glob changes/proposals/*/` 看是否有进行中的变更
3. 仍不确定时 **MUST 问用户**，不要自作主张

## 关键约束（MUST 遵守）

- **任何编码动作 MUST 先有变更提案**（`changes/proposals/<id>/proposal.md`）
  - 唯一豁免：`/quick-fix`（≤5 文件且 ≤50 行的已知原因修复）可免提案与归档，但**不豁免门禁**
- **任何提交动作 MUST 走 `ci-gate` 技能**，禁止裸 `git commit`
  - `/quick-fix` MUST 走 ci-gate 快速通道（仅「编译 + 单测」两项），门禁失败 MUST 转完整流程
- **任何部署动作 MUST 走 `deployment-verification` 技能**，禁止直接操作生产
- **L0 级不可逆操作（推送 master / 生产部署 / 回滚 / force push / 全量发布 / 发包）MUST NOT 自动执行**：strict/standard 问用户；autonomous 阻塞+转人工队列+告警（不等用户输入）
  - 推送 `develop` 为 L1（autonomous 可自动 + audit），不属 L0
- **任何工作开始前 MUST 先识别项目栈**（见上方"第 0 步"）
- **当前提案指针**：AI MUST 读取 `changes/config.yaml: current-proposal` 定位工作目标，多 agent 并发先检查 `proposal.md` `status != blocked-pending-human`
