# 项目栈识别规范

> 通用规则，适用范围：所有技术栈和项目类型。
> 紧凑版红线见 `common-project-stack-detection.mdc`，本文是完整流程与识别表。
>
> 目的：确保 AI 使用**正确的栈级规则**，而不是泛化的 `_common` 规则。

## 核心原则

**栈级优先，`_common` 兜底**。识别不出栈就按 `trust-level` 分级处理，MUST NOT 凭印象猜。

## 第 1 步：识别项目栈

### 通过 wiki 目录识别（最直接）

```bash
ls wiki/   # 安装器按栈建目录，目录名即栈名
```

### 通过依赖文件识别

```bash
# Java / Spring Boot 系
grep -o "cn\.structured\|spring-boot" pom.xml | head -5

# 前端系
grep -o "@structure-projects\|vue\|react\|next" package.json | head -5
```

### 标识文件对照表

| 标识文件 | 推断栈 |
|---|---|
| `pom.xml` 含 `cn.structured` | `structure-boot` |
| `pom.xml` 含 `spring-boot-starter` 但无 `cn.structured` | `spring-boot` |
| `package.json` 含 `vue` | `vue3` |
| `package.json` 含 `react` | `react` |
| `package.json` 含 `next` | `nextjs` |
| `build.gradle` / `build.gradle.kts` | 按 `plugins` / `dependencies` 识别 |
| `Cargo.toml` | Rust 栈（按 dependencies 识别 axum / actix） |
| `go.mod` | Go 栈（按依赖识别 gin / echo） |
| `requirements.txt` / `pyproject.toml` | Python 栈（按依赖识别 django / fastapi / flask） |
| `pubspec.yaml` | `flutter` |
| `*.xcodeproj` / `Podfile` | `ios` |
| `build.gradle`（含 Android plugin） | `android` |

多个标识文件同时命中时按 monorepo 处理：**按当前改动文件所在子目录判定栈**，不同子目录可适用不同栈级规则。

## 第 2 步：规则加载优先级

```
1. 栈级规则（<stack>-*.mdc）          ← 优先级最高
2. 栈级 Wiki（wiki/<stack>/*.md）     ← 细节参考
3. 栈级技能（<stack>-<action>）       ← 栈级动作
4. _common 规则（common-*.mdc）       ← 通用兜底
5. _common Wiki（wiki/_common/*.md）  ← 通用参考
6. _common 技能（git-commit 等）       ← 通用动作
```

栈级与 `_common` 冲突时**以栈级为准**——栈级规则携带具体版本号与必选组件约束，`_common` 只给通用方向。

## 第 3 步：必读栈级 Wiki

| 文件 | 何时必读 |
|---|---|
| `wiki/<stack>/developer.md` | 任何编码任务 |
| `wiki/<stack>/components.md` | 引入依赖 / 选组件 / 定版本 |
| `wiki/<stack>/architect.md` | 涉及分层、模块划分、接口设计 |
| `wiki/<stack>/project-scaffolding.md` | 初始化项目 / 新建模块 |

## 第 4 步：常见错误

- ❌ 只看 `_common` 规则就开始工作
- ❌ 凭 LLM 自带知识选技术栈版本（版本约束在 `wiki/<stack>/components.md`）
- ❌ 忽略栈级规则里的"必选组件"
- ❌ 把 A 栈的规则应用到 B 栈项目

## 栈级约束举例

举例说明"栈级规则携带的信息量"，**不替代** `wiki/<stack>/` 的完整约束。

### structure-boot

| 维度 | 约束 |
|---|---|
| Spring Boot | `4.0.6`（不是 3.x） |
| JDK | 17+ |
| 包名 | `cn.structured.*`（含 d）；`structure-common` / `structure-infra` 用 `cn.structure.*`（无 d） |
| 安全框架 | `structure-security`（含 JWT） |
| JSON | FastJSON（禁止 Jackson / Gson） |
| 服务间调用 | `@FeignClient` + fallback |
| 持久化 | `RepositoryFacade + Delegate` 模式 |
| 异常 | `CommonException` + `{X}ExceptionEnum` |
| 响应 | `ResResultVO<T>` + `ResultUtilSimpleImpl` |

### vue3

| 维度 | 约束 |
|---|---|
| 技术栈 | Vue 3 + Vite + TS + Pinia + Vue Router + Element Plus + UnoCSS |
| 微前端 | `wujie` + `@structure-projects/wujie-subapp` |
| 组件库 | `@structure-projects/components`（按需命名导入） |
| HTTP | `@structure-projects/gateway-client` |
| npm scope | `@structure-projects` |

## 识别失败的处理

按 `trust-level` 分级，详见 `wiki/_common/trust-level.md` §4.2 的 L1 `ai-infer` 行为。

### strict / standard（默认）

1. **MUST** 问用户："请确认当前项目使用的技术栈"
2. **MUST NOT** 默认按通用规则开始工作
3. **MUST NOT** 凭印象猜测栈

### autonomous

1. **降级仅使用 `_common` 规则**（不阻断流程）
2. **MUST** 写 `audit.md`：`id=stack-detect-fallback` / 风险=中 / 复核 24h
3. **MUST** 触发告警，通知人工后续补齐栈识别
4. **MUST NOT** 凭印象猜测栈；严格按 `_common` 层规范执行
5. 后续若识别到栈信息，**SHOULD** 重新安装栈级规则并补充约束

## 关联

- 规则：`common-project-stack-detection.mdc`（紧凑红线版）
- Wiki：`wiki/_common/project-structure.md`、`wiki/_common/trust-level.md`
- 技能：`requirement-analysis` / `project-intake`
