# 工具权限白名单映射（trust-level × HITL → permissions）

> 本文件是 `install.sh --permissions` 的规格说明，定义如何把规则层的 trust-level × HITL 决策**落地到各 AI 工具的权限配置文件**。
> 唯一真相源在 `meta/permissions/trust-level-mapping.yaml`，本 wiki 是它的可读文档。

## 1. 背景

HITL 三级模型（L0/L1/L2）与 trust-level（strict/standard/autonomous）只在**规则层**生效——AI 读规则后自觉遵守。但部分 AI 工具（Cursor/Claude/Trae/CodeBuddy/Qoder）有自己的**权限层**，通过配置文件控制命令调用是否需要确认。通义灵码无此机制，见 § 5.1。

如果规则层说"autonomous 下 L1 可自动"，但工具层的 `settings.json` 没有对应 `allow`，工具仍会弹确认框。`--permissions` 解决"如何把规则层决策落地到工具层配置"。

## 2. 映射矩阵（核心规范）

| trust-level | L0 never-skip | L1 auto-decidable | L2 always-auto |
|---|---|---|---|
| **strict** | ask | ask | allow |
| **standard** | ask | ask | allow |
| **autonomous** | deny | allow | allow |

- strict 与 standard 在**工具层行为相同**（差异在规则层）。
- 工具白名单只做**硬性兜底**，不做规则层的细分判断。

## 3. 基线 deny（永远生效，跨 trust-level）

不可逆操作无论 trust-level 如何都 `deny`，见 `meta/permissions/deny-baseline.yaml`（≥ 12 条）：

| 类别 | 示例 |
|---|---|
| 删除根/用户目录 | `rm -rf /`、`rm -rf ~`、`rm -rf /*` |
| 覆盖公共分支历史 | `git push --force *`、`git push -f *` |
| 权限提升 | `sudo *` |
| 生产/基础设施操作 | `kubectl apply *production*`、`helm upgrade *production*`、`terraform apply/destroy` |
| 执行远程脚本 | `curl * \| bash`、`wget * \| sh` |

## 4. 命令模式（HITL level → 具体命令）

HITL 项是自然语言描述，无法直接转成工具命令。`meta/permissions/trust-level-mapping.yaml` 的 `command-patterns` 段提供**策展映射**（非自然语言自动解析）：

| level | 命令模式 |
|---|---|
| L0 | `git push origin master`、`kubectl apply`、`helm upgrade`、`gh release create` |
| L1 | `git push origin develop`、`git push origin feat-/fix-/hotfix-/release-` |
| L2 | `git add/commit/status/log/diff/branch/checkout`、`mvn test`、`npm test`、`npm run build`、`pytest` |

无命令的 HITL 项（如"需求澄清""proposal 确认"）不在此列，仅规则层生效。

## 5. 工具差异表

| 工具 | 配置文件 | allow 语法 | deny 语法 | ask 语法 |
|---|---|---|---|---|
| Cursor | `.cursor/permissions.json` | `terminalAllowlist: [...]` | 无独立 deny（默认需确认） | 无 |
| Claude | `.claude/settings.json` | `permissions.allow: ["Bash(cmd:*)"]` | `permissions.deny` | `permissions.ask` |
| Trae | `.trae/global.json` | `filesystem.readWrite` | 沙箱只读兜底 | 沙箱外需确认 |
| CodeBuddy | `.codebuddy/settings.json` | `permissions.allow: ["Bash(cmd:*)"]` | `permissions.deny` | `permissions.ask` |
| Qoder | `.qoder/settings.json` | `permissions.allow: ["Bash(cmd:*)"]` | `permissions.deny` | `permissions.ask` |
| **通义灵码** | **不支持** | — | — | — |

Qoder 的权限语法与 Claude 同构（同一套 `Bash(...)` 匹配串、同一套 allow/ask/deny 三段），因此安装器渲染出的三个数组与 `.claude/settings.json` 逐字节相同。差异只在于 Qoder 另有一个全局默认档 `general.defaultPermissionMode`：

| trust-level | `defaultPermissionMode` |
|---|---|
| strict | `default` |
| standard | `accept_edits` |
| autonomous | `auto` |

`autonomous` **MUST NOT** 映射到 `bypass_permissions`——该模式跳过全部权限检查，会让 deny 中的 L0 红线一并失效，与信任级别模型直接冲突。`auto` 保留 deny 优先级。此映射由 `test/fixtures/permissions-validation-test.sh` 断言守护。

### 5.1 通义灵码为何不生成权限文件

灵码（Lingma）**未公开可在仓库内声明的工具权限 / 终端命令白名单格式**。本仓库遵循"文档只描述实现"的原则，**不臆造 `.lingma/settings.json` 等格式**——生成一个工具不读取的文件比不生成更危险：它会让用户以为 autonomous 下有权限兜底。

灵码用户的信任级别只有两道防线：

1. **规则层软约束** — `common-core.mdc` 正常安装到 `.lingma/rules/`，AI 读规则后自觉遵守（与其他工具一致）
2. **git hooks 物理拦截** — `install.sh` 安装的 pre-commit / pre-push hooks 与 AI 工具无关，对灵码同样生效

因此**灵码 MUST NOT 用于 `trust-level: autonomous`** 的无人值守场景——缺少权限层意味着 L0 红线（如 `git push --force`、`rm -rf`）只有规则层劝阻，无强制拦截。若必须无人值守，改用 Claude / Cursor / CodeBuddy / Qoder。

> 若灵码后续公开权限配置格式，在 `meta/permissions/` 补 `lingma-*.tpl` 并在 `_render_permissions_for_tool` 的 case 中加分支即可，无需改动映射矩阵。

## 6. 何时启用（决策树）

```
项目是否需要 AI 无人值守/自动化作业？
  ├─ 否 → 不启用（保持默认，工具自行管理权限）
  └─ 是
      ├─ 需要 autonomous 端到端闭环 → 启用 + trust-level: autonomous
      ├─ 需要 standard 但减少误操作 → 启用 + trust-level: standard
      └─ 仅需 deny 兜底（防误删/强推）→ 启用（任何 trust-level 都有基线 deny）
```

## 7. 使用方式

```bash
# 按当前 changes/config.yaml 的 trust-level 生成权限文件
./install.sh -t <target> -w claude,cursor -c --permissions

# 切换 trust-level 后 MUST 重跑以同步
./install.sh -t <target> -w claude,cursor -c --permissions
```

生成的权限文件头部含 `_generated_by` / `_trust_level` / `_warning` 字段，卸载时据此识别并清理，不误删用户自定义文件。

## 8. 关联

- 映射源：`meta/permissions/trust-level-mapping.yaml`
- 基线：`meta/permissions/deny-baseline.yaml`
- 模板：`meta/permissions/*.json.tpl`
- 信任级别规则：`_common/rules/common-core.mdc`
- 实现：`install.sh`（`--permissions` / `deploy_permissions` / `_rm_permissions`）