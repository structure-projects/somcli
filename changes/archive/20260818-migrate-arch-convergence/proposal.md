# 迁移变更提案：架构收敛、功能验证与文档重建（总纲）

| 字段 | 值 |
|---|---|
| 提案 ID | 20260818-migrate-arch-convergence |
| 级别 | major |
| 类型 | migration |
| 创建日期 | 2026-08-18 |
| 创建人 | chuck |
| 状态 | coding |
| 优先级 | high |
| 技术附录 | `doc/提案-架构收敛与测试体系.md`（实测事实基准：21 个叶子命令清单、26 个能力域、82 个场景、D/E/F/G 缺陷编号表） |
| 子提案 | 见「阶段规划」，每个里程碑一个独立提案目录 |

> 本提案是总纲，只定义现状、目标状态、策略、里程碑与全局约束。
> 逐阶段的任务与验收在各子提案中，独立评审、独立归档。
> 所有以 D/E/F/G 开头的缺陷编号、以 SC- 开头的场景编号，均引用技术附录，本提案不复述细节。

## 现状

somcli 的定位是**初始化运行环境，并对业务服务或工具进行编排与实施**；容器/集群安装（Docker、Swarm、Kubernetes）是这套通用引擎的应用场景之一，不是产品本体。产品本体是 `resources:` 这一层：数组顺序即执行顺序，每个资源走「下载 → 按 `hosts` 分发 → `pre_install` → `post_install`」，脚本经 SSH 在目标节点执行。

技术附录对全仓库做过一次实测审计（编译 + 运行 + 逐字段 grep），结论：

| 维度 | 实测结果 |
|---|---|
| 叶子命令 | 21 个，其中 ✅ 8 / ⚠️ 9 / 🕳 空壳 1 / ❌ 不可用 3 |
| `Resource` 字段 | 14 个，其中 6 个**零消费者**（`method`/`image`/`files`/`res_type`/`remove_scripts`/`extra_files`） |
| 配置键 | 9 类键写了等于没写（`yaml.Unmarshal` 静默忽略），含目标架构的关键字段 `k8sConfig.resources` |
| 能力域 | 26 个，审计时仅 1 个达标 |
| 场景 | 82 个，审计时 ✓ 9 / △ 24 / ✗ 49，自动化覆盖率 0 |
| 架构倒置 | `pkg/cluster/kubernetes.go` 把 4 个 `types.Resource` 硬编码在 Go 里，而不是作为随产品发布的配置去消费通用引擎 |

审计后已在**当前工作区（尚未提交）**完成 Phase 0 的主体部分，实测复核如下：

| 已完成项 | 证据 |
|---|---|
| D2 `RunScripts` 不再吞错 | `pkg/utils/command.go` 逐脚本包装错误并返回，失败向上传播 |
| D1 `SetNode` 断链 | `pkg/cluster/common.go:51` 调用 `utils.SetNode`；`GetNode` 改为返回 `error`，去掉 `127.0.0.1` 兜底 |
| F1 模板转义 | `pkg/utils/utils.go` 已改 `text/template` |
| F4 / G8 假成功 | `PrintSuccess` 移入成功分支；`DownloadResources` 失败返回 error，退出码非 0 |
| D9 帮助有副作用 | `cmd/compose.go` 保留透传所需的 `DisableFlagParsing`，在 `Run` 内前置拦截 `--help`/`-h` |
| 配置严格解析 | `pkg/utils/utils.go`、`pkg/cluster/common.go`、`pkg/installer/downloader.go` 三处改 `yaml.UnmarshalStrict` |
| 格式化 | `gofmt -l .` 输出为空 |
| 验收清单 | `test/matrix.yaml`（82 场景，人工维护、人工评审） |
| 黑盒用例 | `test/contract/exitcode_test.go`、`cli_debug_test.go` 两个文件确为黑盒（编译二进制 + 真实配置 + 断言退出码/输出） |
| CI | `.github/workflows/ci.yml` 取代无效的 `test.yml`（旧文件用 `go fmt ./...`，会改写文件且恒退出 0） |

工作区还有一批**方向错误的产物需要在 M0 内清除**（详见 M0 子提案）：

| 错误产物 | 为什么错 |
|---|---|
| `test/unit/` 7 个文件 | 直接 import 并调用 `pkg/utils`、`pkg/installer` 的导出函数，是对内部实现的白盒测试，不是对 somcli 功能的验证 |
| `test/coverage_matrix_test.go` | 校验「测试文件名与场景 ID 是否对应」，测的是测试自己，属测试体系而非功能验证 |
| `test/contract/config_test.go` | 用 `yaml.UnmarshalStrict` 直接把示例配置解到 `types.ResourceConfig`，绕过了二进制 |
| `ci.yml` 的 `MIN_COVERAGE` 与 `-coverpkg` | 代码行覆盖率是白盒指标，会反向逼迫为覆盖率而拆函数 |

**当前进度：82 个场景中 3 个 `done`（SC-X02、SC-X06、SC-P10），79 个 `pending`。**
先前记为 done 的另外 13 个场景由 `test/unit/` 与 `config_test.go` 承载，随其删除一并回退为 `pending`，
由 M0 用黑盒用例重新兑现。这个回退是有意的：宁可进度数字变小，也不要用白盒断言冒充功能验证。

## 目标状态

```
                   ┌─────────────────────────────┐
   configs/*.yaml  │   声明层（用户可改）          │
                   │   resources / nodes /        │
                   │   cluster.resources[名称引用] │
                   └──────────────┬──────────────┘
                   ┌──────────────▼──────────────┐
                   │   通用编排引擎（唯一执行路径） │
                   │   pkg/installer              │
                   │   解析 → 规划 → 分发 → 执行   │
                   │   method 分发 / 幂等 / 回滚   │
                   └──────────────┬──────────────┘
        ┌────────────┬────────────┼────────────┬────────────┐
     cluster      docker       compose      registry      images
   （只做编排）    （薄封装）
```

硬性原则：**`pkg/cluster` 不得再包含任何 `types.Resource` 字面量**，一切安装内容下沉为随产品发布的配置。

验收看三件事：

1. `test/matrix.yaml` 的 82 个场景全部 `status: done`，每个 done 场景都有一条**跑真二进制**的用例，CI 全绿；
2. 21 个叶子命令中 `🕳 空壳` 与 `❌ 不可用` 计数归零（当前 1 + 3）；
3. `doc/00-功能清单与矩阵.md`、`doc/11-命令参考.md` 由代码与场景清单渲染生成，CI `git diff --exit-code` 零差异。

## 功能验证约定（全局硬约束）

本项目的测试只有一个目的：**验证 somcli 这个工具的功能是否如宣称那样工作**。不建设"测试体系"。

- **一律黑盒**：编译出 `somcli` 二进制 → 喂一份真实配置文件 → 执行真实命令 → 断言退出码、输出、落盘产物、目标节点状态。
- **禁止 import 本仓库的 `pkg/`**：`test/` 下任何文件不得引用 `github.com/structure-projects/somcli/...`。这条由 `ci.yml` 的 `static` job 用一条 grep 守住。
- **不得为可测性改生产代码**：不抽接口、不注入依赖、不为了断言 argv 而拆纯函数。修缺陷本身不算侵入 —— 缺陷是要修的，只是不能以"方便测试"为理由改变结构。
- **不设代码覆盖率门槛**：覆盖率是白盒指标，会把注意力从"功能对不对"引到"行有没有被走到"。进度用 `test/matrix.yaml` 的场景数衡量。
- **不写测自己的测试**：没有元测试、没有矩阵校验测试、没有测试命名检查。清单靠人评审。
- **观察口只能是用户能看到的东西**：退出码、stdout/stderr、文件系统、远程节点上的真实状态。若某个行为从外部完全不可观察，那它要么该有一个用户可见的入口，要么就不该被断言。

按需要的执行环境分组，而不是按"测试层级"：

| 环境 | 依赖 | build tag | 载体 |
|---|---|---|---|
| `local` | Go + 本机 shell | 无（默认跑） | `test/local/` |
| `remote` | 一个 SSH 可达节点（CI runner 自连） | `remote` | `test/remote/` |
| `multinode` | 3 个 sshd 容器 | `multinode` | `test/multinode/` |
| `cluster` | 可安装 k8s / swarm 的特权环境 | `cluster` | `test/cluster/` |
| `matrix` | 多发行版 / 多架构 | — | 流水线矩阵，无 Go 用例 |

`go test ./...` 默认只跑 `local`，必须保持秒级。

## 迁移策略

**渐进改造**（不冻结、不整体重写）。理由：

- 项目已有真实用户路径（`install -f`、`cluster create`），整体重写会失去唯一可参照的行为基准；
- 缺陷互相掩盖 —— 在 `RunScripts` 吞错与 `SetNode` 断链修好之前，任何"测试通过"都不可信，因此必须先建立可信信号，再谈重构；
- 按能力域切片后每个里程碑都能独立验收（场景 ID 是天然的切分线）。

执行顺序的唯一强约束：**先立清单 → 再建可信信号 → 再补引擎 → 再动集群 → 最后平台与文档**。
清单（`test/matrix.yaml`）先行的理由是它把「功能清单 / 场景清单 / 验收标准」三张表压成一份，进度可数、不可自欺。它是一份人维护的验收清单，不需要机器来校验它自己。

## 阶段规划

| 里程碑 | 提案 | 范围 | 场景 | 完成标准 |
|---|---|---|---|---|
| M0 | `20260818-migrate-phase0-trusted-signal` | 建立可信信号：D1/D2/D9/F1/F4/G8 + 严格解析 + 场景清单 + 清除白盒产物 + 黑盒用例（local/remote/multinode）+ CI | 22 | 失败路径退出码非 0 且日志无 `✓`；`--help` 无副作用；22 个场景 done 且全部由黑盒用例承载 |
| M1 | `20260818-migrate-phase1-engine-completion` | 引擎补全：`method` 分发、`extra_files` 落盘、`uninstall`、用户变量、幂等/状态/并发、下载器修正、compose/docker/images/registry 子系统修复 | 27 | 累计 49 场景 done；空壳与不可用命令归零 |
| M2 | `20260818-migrate-phase2-cluster-as-config` | 集群安装归位：硬编码 Resource 外置为 `configs/k8s/*.yaml`；CNI、cri-dockerd、多 master、join 命令修正 | 21 | 累计 70 场景 done；E2E 单节点矩阵全绿、双节点 Join 与跨节点通信通过 |
| M3 | `20260818-migrate-phase3-platform-compat` | 平台兼容：发行版抽象、架构参数化、换源、构建矩阵归一 | 12 | 累计 82 场景 done；5 个发行版镜像上 `method: package` 均成功 |
| M4 | `20260818-migrate-phase4-config-and-docs` | 配置 schema 归一 + 文档全量重建 + 防漂移机制 | — | `doc/00`、`doc/11` 与代码零 diff；82 个场景 ID 在场景文档中均有说明 |

子提案在其里程碑启动时置为 `coding`，未启动前保持 `draft`；`changes/config.yaml` 的 `current-proposal` 始终指向当前活跃的那一个。

## 风险评估

- **一次性提交 167 个变更文件难以评审** → M0 内按语义拆提交：格式化独立一提、代码修复按缺陷编号分组、测试与 CI 各自独立；提交前先切 `feat-*` 分支（`.git/hooks/commit-msg` 禁止在 master/develop 直接提交）。
- **`yaml.UnmarshalStrict` 会让用户既有配置直接报错** → 属预期的破坏性变更（原本是静默失效，更危险）。缓解：同步修正 `configs/**` 全部示例，在 changelog 标注 BREAKING 并列出被拒绝的 9 类键。
- **`multinode`/`cluster` 环境依赖 docker + systemd 容器，本地 macOS arm64 无法验证** → 这两组只在 CI 执行；本地开发以 `local` 组为准，用 build tag 隔离，保证 `go test ./...` 默认秒级。
- **M2 把 k8s 安装内容从 Go 外置为配置，可能引入行为回退** → 严格顺序：先在 M2 内落地 `cluster` 环境的 E2E 断言现有行为，再做外置；外置与修 CNI/多 master 分成两批提交。
- **E2E 3×2×2 = 12 job × 40min 的 nightly 成本** → 见「待决事项」，倾向按 k8s 版本轮转而非每晚全跑。
- **大面积重写与 `common-legacy-tolerance` 冲突** → 见「双规范并存期约定」，每个里程碑只动本里程碑 scope 内的文件，禁止顺手重构。
- **黑盒用例的最大风险是"跑了但没验到"** —— 断言只看退出码 0 就容易通过而无信息量。缓解：每个场景的用例 MUST 有可观察的正向产物断言（文件内容、节点状态、输出关键字），且新增用例时 MUST 先确认它在缺陷未修的版本上会失败（回归有效性自检，写入各阶段 tasks）。
- **本机执行真实脚本可能污染开发机** —— `local` 组用例只允许在 `t.TempDir()` 内产生副作用，脚本限于 `echo`/`true`/`exit N` 之类；任何需要 root、包管理器或 systemd 的场景一律归入 `remote`/`multinode`/`cluster` 组。

## 回滚预案

- **代码回滚**：每个里程碑一个 `feat-*` 分支、一次合并。回滚 = `git revert` 该里程碑的合并提交，里程碑之间无交叉依赖（除顺序前置关系）。
- **配置回滚**：`configs/**` 与代码同批变更，随代码一起 revert。严格解析属破坏性变更，回滚需同时恢复用户侧配置，故 M0 合并前需在 changelog 明确列出受影响键。
- **CI 回滚**：`ci.yml` 与 `integration.yml`/`e2e.yml` 相互独立，单个工作流可单独禁用而不影响其他组。
- **无数据库、无线上服务**，不涉及数据迁移与配置中心回滚。
- **不引入兼容开关/feature flag**：本次全部是"修正错误行为"，保留旧行为等于保留假成功信号。

## 兼容性保证

| 维度 | 约定 |
|---|---|
| CLI | 现有命令与标志只补齐不删除。例外：`--source` 由 `bool` 改为 `StringSlice`（原类型即错误，`viper.GetStringSlice` 取出 `["false"]`），属 BREAKING，M3 处理并写 changelog |
| 配置 | 严格解析后未知键报错；被拒绝的 9 类键在 `doc/` 给出逐条迁移写法。schema 归一（M4）保留 `config.yaml` 形态为唯一目标形态，另两套写法给出等价改写示例 |
| 行为 | 失败不再打印 `✓`、退出码不再恒为 0 —— 属修正而非破坏，但会改变调用方（CI/脚本）的可见行为，MUST 在 changelog 显著标注 |
| 产物 | 5 平台交叉编译保持可用；windows 产物去留见「待决事项」 |

## 双规范并存期约定

- **老代码**（`pkg/cluster/kubernetes.go`、`pkg/docker`、`pkg/registry` 等）：在其所属里程碑到达前保持现状，只做必要的缺陷修复，不重构、不改分层。
- **新代码**：按 `.claude/` 规则与 somcli 自身结构。注意 somcli 是 cobra CLI，`gin-*` 规则中的 handler/service/repository 分层不适用，冲突时以 somcli 现有结构为准。
- **用例命名**：测试函数名或 `t.Run` 名含场景 ID（如 `TestSC_F04_PreInstallFailureAborts`），便于从清单定位到用例。这是**约定，不是机器校验**；与 `common-testing` 的 `should<Expected>When<Condition>` 冲突时取场景 ID，因为清单是本项目的验收语言。
- **边界识别**：文件是否出现在当前里程碑子提案的「影响范围」中。未列入者不得在本里程碑修改（格式化提交除外）。

## 待决事项

1. **windows 产物是否保留** —— 工具强依赖 `sh`/`ssh`/`scp`/`systemctl`，倾向移除；当前 `build.sh` / `Makefile` / `go.yml` / `ci.yml` 四处构建矩阵互相矛盾，M3 归一时需定论。
2. **arm64 支持级别** —— 上游 cri-dockerd 与部分 CNI 的 arm64 资产不齐，倾向 M3 做到"引擎与工具编排支持 arm64，k8s 集群安装标注实验性"。
3. **多 master VIP 方案** —— keepalived + haproxy 由 somcli 编排，还是要求用户自备 LB。
4. **E2E nightly 成本** —— 12 job × 40min 是否可接受，或按 k8s 版本轮转。
5. **`doc/设计.md` 处置** —— 保留加注，还是拆分入新文档结构后归档。
6. **是否新增 `somcli validate -f <file>` 只读校验命令** —— 纯黑盒验证「配置能否被正确解析与渲染」需要一个不产生副作用的入口，否则只能靠 `install` 真跑（会真装东西）。这条按**产品能力**立项而非测试便利：严格解析上线后，用户照抄示例前更需要一个自查入口。倾向在 M0 内实现（新增只读命令，不改动既有执行路径）。若不做，SC-X05/SC-F07 只能降级为"用一份最小无害配置间接验证解析行为"。

## 任务清单

详见 `tasks.md`（里程碑级），逐阶段任务见各子提案的 `tasks.md`。

## 变更日志

每个里程碑合并后在 `changes/changelog/<version>.md` 补条目；BREAKING 项（严格解析、退出码语义、`--source` 类型）单列。