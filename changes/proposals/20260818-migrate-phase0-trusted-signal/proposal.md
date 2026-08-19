# 迁移变更提案：M0 建立可信信号

| 字段 | 值 |
|---|---|
| 提案 ID | 20260818-migrate-phase0-trusted-signal |
| 级别 | major |
| 类型 | migration |
| 创建日期 | 2026-08-18 |
| 创建人 | chuck |
| 状态 | review |
| 优先级 | high |
| 总纲 | `changes/proposals/20260818-migrate-arch-convergence/proposal.md` |
| 技术附录 | `doc/提案-架构收敛与测试体系.md` §4（缺陷基线）、§5.2 Phase 0 |
| 场景 | 24 个（`test/matrix.yaml` 中 `phase: 0`），24 done / 0 pending |
| 评审 | `review.md`（⚠️ 有条件通过，2 MUST / 5 SHOULD 已全部处理） |

> 本里程碑是所有后续工作的前置。在 `RunScripts` 吞错与 `SetNode` 断链修好之前，
> 任何"验证通过"都不可信 —— 失败也会打印 `✓ 成功`，连人工判断都不可靠。
>
> 验证方式遵循总纲「功能验证约定」：**一律黑盒**，编译二进制 → 喂真实配置 → 跑真实命令 →
> 断言退出码 / 输出 / 落盘产物 / 目标节点状态。`test/` 下禁止 import 本仓库 `pkg/`。

## 现状

### 已完成的代码修复（工作区未提交）

| 项 | 状态 | 证据 |
|---|---|---|
| D2 `RunScripts` 吞掉全部错误 | 已修 | `pkg/utils/command.go` 逐脚本包装错误并向上返回，本地/远程两条路径均传播 |
| D1 `SetNode` 全仓库零调用点 | 已修 | `pkg/cluster/common.go:51` 调用 `utils.SetNode(config.Cluster.Nodes)`；`GetNode` 改签名返回 `error`，去掉 `127.0.0.1` 静默兜底 |
| F1 `html/template` 转义 shell 的 `&` | 已修 | `pkg/utils/utils.go` 改 `text/template` |
| F4 下载失败仍打印 SUCCESS | 已修 | `pkg/installer/downloader.go:94` `PrintSuccess` 移入成功分支 |
| G8 下载全失败仍 exit 0 | 已修 | `DownloadResources` 返回 error，`cmd/install.go:102` 据此返回失败 |
| D9 `docker-compose --help` 触发真实安装 | 已修 | `cmd/compose.go` 保留透传所需的 `DisableFlagParsing`，在 `Run` 内前置拦截 `--help`/`-h`，安装器之前返回 |
| 配置静默忽略未知字段 | 已修 | `pkg/utils/utils.go:315`、`pkg/cluster/common.go:36`、`pkg/installer/downloader.go:36` 三处改 `yaml.UnmarshalStrict` |
| 17 个文件未格式化 | 已修 | `gofmt -l .` 输出为空 |
| CI 恒绿但零保障 | 已修 | `.github/workflows/ci.yml` 取代 `test.yml`（旧文件用 `go fmt ./...`，就地改写文件且恒退出 0） |

### 已完成但方向错误、必须在本里程碑清除的产物

上一轮把"建设测试体系"当成了交付物，写出了白盒断言与测自己的测试。somcli 需要的是
**对自身功能的黑盒验证**，因此以下产物一律删除或重写：

| 产物 | 为什么错 | 处置 |
|---|---|---|
| `test/unit/` 7 个文件 | 直接 import 并调用 `utils.SetNode`、`utils.ParseStr`、`installer.*` 等导出函数，断言的是内部实现而非工具行为；这类用例会随重构变红，且反向锁死结构 | 删除，覆盖的场景改写为黑盒用例 |
| `test/coverage_matrix_test.go` | 校验"测试函数名是否含场景 ID"，测的是测试自己 | 删除；场景清单改为人工维护、人工评审 |
| `test/contract/config_test.go` | 用 `yaml.UnmarshalStrict` 直接解到 `types.ResourceConfig`，绕过二进制；还维护了一份 `nonconforming` 欠账清单与"清单必须准确"的自校验测试 | 重写为跑二进制的配置校验用例，欠账清单移入本提案与 M4 |
| `ci.yml` 的 `MIN_COVERAGE: '9'` 与 `-coverpkg=./...` | 行覆盖率是白盒指标，会反向逼迫为覆盖率而拆函数、抽接口 | 删除门槛与 `-coverpkg`；保留 `-race` |

`test/contract/exitcode_test.go` 与 `test/contract/cli_debug_test.go` 确为黑盒（`go build` 出二进制、
`exec.Command` 执行、断言退出码与输出），保留，随目录改名迁到 `test/local/`。

### 进度回退说明

`test/matrix.yaml` 中先前记为 `done` 的 16 个场景，删除白盒产物后只剩 3 个仍有承载：
SC-X02（`cli_debug_test.go`）、SC-X06（`exitcode_test.go`）、SC-P10（构建矩阵）。
其余 13 个（SC-E02/E07/E13、SC-D01/D03/D04、SC-F01/F02/F04/F06/F07、SC-X01/X05）已回退为
`pending`，由本里程碑用黑盒用例重新兑现。**宁可进度数字变小，也不要用白盒断言冒充功能验证。**

### 写黑盒用例时新发现的缺陷

写用例的价值不止于防回归 —— 下面几项是补 M0.3 用例时被用例本身逼出来的，
静态阅读代码看不出来（多属于「代码写好了但没接线」这一类，与 D1 同源）。

| 编号 | 位置 | 问题 | 处置 |
|---|---|---|---|
| D10 | `cmd/compose.go` | 透传用的 `DisableFlagParsing` 让 cobra 连 somcli 自己的全局 flag 都不解析：`somcli --workdir X docker-compose up` 既丢了 `--workdir`（产物落回 `./somwork`），又把 `--workdir X` 当成 compose 的参数透传下去。**同一根因让 D9 的修复漏了一半** —— `args[0]` 是 `--workdir` 而不是 `--help`，帮助拦截失效，实测 `somcli --workdir /tmp/x docker-compose --help` 仍然去下载安装。 | 帮助拦截这一半在本里程碑修（跳过头部的全局 flag 再判断，`isHelpRequest` 接收 root 的 `PersistentFlags`）；全局 flag 真正生效这一半需要重排透传命令的解析时机，记为 SC-X08 留给 M1 |
| D11 | `cmd/install.go` | `installer.InstallTool(file, name, quiet)` 早已实现，但 `-n/--name` 从未注册 → 按名安装（SC-E07）整条路径不可达，`somcli install -f cfg -n b` 报 `unknown shorthand flag: 'n'`。 | 已修：注册 `-n/--name` 并接上 `InstallTool` |
| D12 | `cmd/root.go` | `--source` 帮助写着 "comma-separated or multiple flags"，类型却是 `BoolVar`。绑到 viper 的 `mirrors_source` 后 `GetStringSlice` 读回 `["false"]`：每条命令都白跑一次 `InitSource` 并打印 `加载源 -> [false]`，**且把配置文件里真正的 `mirrors_source` 盖掉**。 | 已修：改 `StringSliceVar`，并在 `len(sourceList) > 0` 时才 `InitSource` |
| G9 | `configs/*.yaml`、`cmd/install.go` | 示例配置与帮助描述了一批从未被消费的字段/能力：资源级 `method`（`install --help` 声称支持 package/binary/source/container 四种安装方式，实际只有 download→scripts 一条路径）、`res_type`、`files`、`extra_files`、资源级 `roles`，以及 `configs/tools.yaml` 里的 `package:`；`configs/kubernetes-cluster.yaml` 更是写了 5 个 YAML 文档而加载器只读第一个，后 4 段被静默丢弃。 | 本里程碑：示例配置按当前 schema 重写、`install --help` 改为陈述真实流程并明说 `method` 未消费、`validate` 兜住回归；`method` 真正分发留给 M1 |
| D13 | `pkg/utils/ssh.go` | `sshKey: "~/.ssh/id_rsa"`（**所有示例配置都这么写**）只在 `RunCommandOnNode` 里被展开，`CopyToRemote` / `SSHMCmd` / `RsyncCopy` / `SSHClient` / `getSSHConfig` 都拿原样字符串。它们是 `exec` 直接拉起 ssh/scp 的，没有 shell 帮忙展开 `~`，于是照文档写配置的人**文件分发一律失败**。写 SC-E03 时故意在配置里保留 `~` 形式才撞出来。 | 已修：上述五个入口统一走 `ExpandPath`。`pkg/docker/installer.go` 不动 —— 它拼的是交给 `bash -c` 执行的命令串，那里 `~` 本来就会展开 |

新增只读入口 `somcli validate -f`：走 install 完全相同的加载与渲染路径但一步动作都不做，
用于在动手装之前回答"配置写对了吗"。它同时是 SC-X05 的判据载体 —— 上面 D12 与 G9 都是它一跑就现形的。

### 评审阶段补记（见 `review.md`）

评审对照本提案逐维度过了一遍，结论是有条件通过。两项 MUST 都已修，其中一项是**本提案自己引入的回归**，
值得单独记账 —— 它说明"把静默忽略改成报错"这件事本身也会不小心写出新的静默忽略：

| 编号 | 位置 | 问题 | 处置 |
|---|---|---|---|
| D14 | `pkg/images/utils.go` | M0.7 重写 `loadCustomImageList` 的纯文本分支时改用了 `strings.Cut(line, ":")`，切的是**第一个**冒号：`registry:5000/nginx:latest` 得到 `name=registry` / `tag=5000/nginx:latest`，然后拿这个错名字去 pull。旧代码用 `strings.Split` + `len(parts) != 2` 会警告并跳过 —— 即改动前是"拒绝并告知"，改动后变成"静默取错值"，与本提案的主张正好相反。 | 已修：抽 `splitNameTag`，切最后一个冒号，最后一段含 `/` 则判为无 tag |
| 流程 | `.github/workflows/*` | `ci.yml` 与 `integration.yml` 都只在 push / PR 到 `master`、`develop` 时触发，feat 分支推上去不跑任何 job，`ci.yml` 连 `workflow_dispatch` 都没有。而本提案要求"归档在推送前完成"且"矩阵全绿" —— 拿绿信号必须先开 PR，开 PR 必须先推送，推送前必须先归档，归档前必须先绿。 | 已解（用户定论"改触发条件"）：两个 workflow 的 `push.branches` 加 `feat-*` / `fix-*`，`ci.yml` 补 `workflow_dispatch` |

5 项 SHOULD 全修，无搁置：`LoadDownloadConfig` 改为委托 `utils.LoadConfig`（此前 `download -f`
不应用全局设置，同一份文件 install 认、download 不认，正是 M0.7 要消灭的差异）；
`cluster.LoadConfig` 覆盖节点表补注释说明这是有意取舍；SC-X09 的 desc 收窄到用例真断言的三条命令，
`images --custom-file` 那半条另立 SC-X10 挂到 M4（需 docker）；`applyGlobalSettings` 注释改为与实现一致；
`cmd/validate.go` 的 `MarkFlagRequired` 补 `_ =`。

SC-X09 那条尤其值得记：矩阵是人工维护的唯一事实来源，一条 `done` 里夹着未断言的承诺，
就等于让矩阵重新变成"自己说自己通过了"的东西 —— 正是本里程碑要消灭的。


### 本里程碑剩余范围

代码与用例已全部写完，7 个场景的载体不在开发机上，判定交给 CI：

| 场景 | 载体 | 为什么本机验不了 | 结果 |
|---|---|---|---|
| SC-E03 单个远程节点安装 | `test/remote/dispatch_test.go`（tag `remote`） | 需要一个能连的 SSH 目标；开发机通常没开 sshd，macOS 还得先给 lo0 加回环别名 | ✅ 绿 |
| SC-E04/E05/E06 多远程节点 / `hosts` 定向 / 混合编排 | `test/multinode/`（tag `multinode`）+ 3 节点 sshd 容器 | 需要 docker；本机 `docker` 命令不存在 | ✅ 绿 |
| SC-D01/D03/D04 下载与 target 路径 | `test/local/download_test.go` | 下载器 shell out 到 `wget`，无 curl 回退也未用 `net/http`（F5）；无 wget 时 `t.Skip` | ✅ 绿（ubuntu / ubuntu-arm） |

2026-08-19 在 `feat-arch-convergence` 上两条流水线全绿（CI run 32251103753、
Integration run 32251103809），7 条随即置 `done`。此前一律留 `pending` —— 提前置 `done`
会让矩阵重新变成"自己说自己通过了"的东西，正是本里程碑要消灭的。

首轮 CI 红了两处，都不是 somcli：

1. **multinode 三条全红，红的是用例自己。** ssh helper 用 `CombinedOutput`，而
   `UserKnownHostsFile=/dev/null` 让 ssh 每次连接都往 stderr 写
   `Warning: Permanently added ...`，读回的"产物内容"于是成了 `Warning: ...\r\nnode-a`。
   somcli 的行为在日志里是对的：三节点各自执行、`hosts` 定向只命中 node-b、混排两个方向都对。
   改为只取 stdout；remote 组同一处一并改（那边不禁 known_hosts，首连后不再提示，
   恰好是绿的 —— 等于把断言正确性交给运行顺序）。
2. **静态检查红在 yamllint。** `configs/config.yaml` 缺文件尾换行，
   `new-line-at-end-of-file` 在 relaxed 规则里是 error 而非 warning。

顺带补了一处"跳过冒充通过"：SC-D01/D03/D04 在无 wget 时 `t.Skip`，而 `ci.yml` 不带 `-v`，
日志里跳过与通过长得一模一样。改为 **Linux 上缺 wget 硬失败**（macOS 仍 skip，那里确实没有
wget，也正是 F5 欠账本身），于是 Linux 那两格绿就等于这三条真的跑了。

**结转 M1**：`template_test.go` 的 SC-E13 余下部分（URL / target 两处模板上下文）没有写，
本次也不勾 —— 这两处都在下载器那条路径上，而 M1 本来就要把下载器从 exec wget 改成 `net/http`
重写一遍。CI 绿证明的是 SC-D01/D03/D04，不含这一条。

其中 **SC-E05 的负向断言是 D1 的终极回归**：断言文件出现在目标节点、且**不出现在运行 somcli 的操作机**上，直接封死"误装在操作机"复现的可能。

## 目标状态

从用户视角能观察到的行为：

- 任何失败路径：退出码非 0，输出中不出现 `[SUCCESS]`；
- `somcli <任意命令> --help` 不产生任何文件系统或网络副作用；
- `hosts` 引用未声明的节点 → 明确报错，绝不兜底到本机执行；
- 配置含未知字段 / 重复键 → 拒绝而非静默忽略；
- `--workdir` 一改，全部派生目录随之改变，不再往仓库里写 `somwork`；
- `test/matrix.yaml` 中 24 个 `phase: 0` 场景全部 `status: done`，且每条都由跑二进制的用例承载。

工程侧：`test/` 只剩 `local`/`remote`/`multinode` 三个包，`go test ./...` 默认只跑 `local` 且保持秒级。

## 迁移策略

渐进改造，且**先修错误传播、再补验证**：错误传播修好之前写的用例无法区分"真通过"和"吞错后的假通过"。

黑盒用例的落地手法（对应各场景，不改一行生产代码）：

| 要观察的行为 | 黑盒观察手法 |
|---|---|
| 脚本是否执行、执行顺序 | `pre_install`/`post_install` 写 `echo <标记> >> $FILE`，跑完读文件断言内容与顺序 |
| 模板渲染结果 | 让脚本把渲染后的字符串 `echo` 到临时文件，断言文件内容（覆盖 `&`、`{{.Version}}`、`{{.WorkDir}}`） |
| 模板变量不存在 | 配置里写 `{{.NoSuchVar}}`，断言退出码非 0 且错误信息含变量名 |
| 下载与 checksum | 用例内起 `httptest` 本地 HTTP 服务当下载源，配置 URL 指向它；断言产物落盘位置、内容、校验失败时不留残留文件 |
| 派生目录 | 传不同 `--workdir`，断言产物只出现在该目录下，仓库目录零新增文件 |
| 节点解析 | 配置声明 node-a，`hosts` 写 node-x，断言退出码非 0、错误含 `node-x`，且**本机无任何副作用** |
| SSH 不可达 | `nodes` 指向 `192.0.2.1`（TEST-NET-1，保证不可达），断言错误信息含节点名/用户/IP |
| 帮助无副作用 | 跑 `--help` 前后对 `--workdir` 目录与 `HOME` 做文件树快照对比，要求完全一致 |
| 文档命令真实存在 | 抽 `README.md` + `doc/*.md` 中的 `somcli ...` 调用，对二进制执行 `<cmd> --help`，断言退出码 0；标志则断言出现在该命令的 `--help` 输出里 |
| 远程分发 | CI runner 对自身 SSH；断言目标路径出现文件、内容一致 |

**SC-X05 / SC-F07（配置校验）需要一个不产生副作用的入口**，否则只能靠 `install` 真跑。
本提案的取向：新增 `somcli validate -f <file>` 只读命令 —— 它按**产品能力**立项（严格解析上线后，
用户照抄示例前需要自查手段），不是为测试而加的钩子。定论见总纲待决事项 6；若否决，则退化为
"用一份最小无害配置（只含 `echo` 脚本）间接验证解析行为"，验证力度下降但仍是黑盒。

提交切分（避免 167 个文件一次评审）：

1. `style:` 纯格式化（`gofmt -w`）
2. `chore:` 规则与 changes 骨架安装（`.claude/`、`changes/`）
3. `docs(changes):` 本提案与各子提案
4. `fix:` 按缺陷编号分组：D2+D1（错误传播与节点解析）、F1、F4+G8、D9、严格解析
5. `test:` 场景清单 + 黑盒用例（`test/local/`）
6. `ci:` `ci.yml` 取代 `test.yml`

## 阶段规划

| 里程碑 | 范围 | 完成标准 |
|---|---|---|
| M0.1 | 清除白盒产物：删 `test/unit/`、`coverage_matrix_test.go`，`test/contract/` → `test/local/`，去掉覆盖率门槛 | `go test ./...` 全绿；`grep -r "structure-projects/somcli" test/` 无结果 |
| M0.2 | 已完成项落库：切分支 + 按上述 6 组提交 | `go build ./...`、`go test ./...` 全绿；`gofmt -l` 为空 |
| M0.3 | `local` 组黑盒用例（13 个回退场景 + SC-E01/E02/E07/X04） | 每条用例在缺陷未修版本上能复现失败 |
| M0.4 | `remote` 组（SC-E03） | CI runner 对自身 SSH，走真实 `scp` 分发与远程执行路径 |
| M0.5 | `multinode` 组（SC-E04/E05/E06） | 3 个 systemd-enabled sshd 容器；`hosts` 定向的负向断言通过 |
| M0.6 | `integration.yml` 落地 | 24 个场景 done，两条流水线全绿 |

## 风险评估

- **M0.3 的 SC-X04 会立刻让 CI 变红** —— 技术附录 G4/G5 已实测出大量文档命令不存在（`docker-images`、`cluster deploy`、`offline download`、`registry install -h` 等）。缓解：只修文档中的命令名与标志（trivial 级改动），schema 与文档结构的重建留给 M4。
- **`multinode` 依赖 docker + systemd 容器，本地 macOS arm64 无法验证** → 只在 CI 执行，用 build tag `multinode` 隔离，`go test ./...` 默认不跑。
- **严格解析让 `configs/**` 现有示例直接报错** → 与代码同批修正示例（`{{.Workdir}}` → `{{.WorkDir}}` 等），已知仍不符合类型的 3 个文件（`config.yaml` 的 cluster 列表形态与 `images:` 段、`kubernetes-cluster.yaml` 的多文档 `version/kind` schema、`tools.yaml` 依赖尚未实现的 `package` 能力）留到 M1/M4，本提案在「兼容性保证」中逐条记账，不写成测试白名单。
- **黑盒用例可能"跑了但没验到"** → 每条用例必须有正向产物断言（文件内容 / 输出关键字 / 节点状态），并且新增时先在未修版本上确认会失败。
- **`local` 组在开发机上真实执行脚本** → 副作用一律限制在 `t.TempDir()`，脚本只用 `echo`/`true`/`exit N`；任何需要 root、包管理器或 systemd 的场景归入 `remote`/`multinode`。

## 回滚预案

单分支单合并，回滚 = `git revert` 合并提交。特别注意：回滚会一并恢复"失败打印成功"的行为，因此 M0 一旦合并不应回滚，若有问题应向前修复。

## 兼容性保证

| 维度 | 说明 |
|---|---|
| CLI | 命令与标志无删除、无重命名；新增只读命令 `validate`，新增 `install -n`、`cluster create/remove --cluster-name`、`cluster remove --cluster-type`（纯新增，不影响既有路径）；`--source` 由 bool 改为字符串列表，此前它的 bool 语义本就与帮助文本不符且从未真正工作 |
| 配置 | **BREAKING**：未知键从静默忽略改为报错。受影响的 9 类键见技术附录 §2.3，changelog 逐条列出 |
| 配置 | **BREAKING**：`cluster:` 由映射改为列表，所有现存集群配置需改写（迁移写法见 changelog） |
| 行为 | **BREAKING**：失败退出码从 0 变为非 0，调用方脚本/CI 可见行为变化 |
| `--help` | 从"可能触发安装"变为纯只读，属安全性修正 |
| 已知欠账 | 见 changelog「已知欠账」段：wget 硬依赖（F5）、`method` 等未消费字段（G9）、透传命令的全局 flag 不生效（D10 后半）、`StrictHostKeyChecking=no` |

## 双规范并存期约定

- 老代码：本里程碑只碰错误传播、节点解析、模板引擎、下载结果打印、compose 帮助拦截、YAML 解析六处，其余文件除格式化外不动。
- 新代码：`test/` 下只允许黑盒用例；用例命名含场景 ID（`SC_E05` 形式）是**约定，无机器校验**。
- 边界识别：文件是否在「影响范围」中。

## 影响范围

- **代码**：`pkg/utils/{command,utils,ssh}.go`、`pkg/cluster/common.go`、`pkg/installer/{downloader,installer}.go`、`pkg/images/utils.go`、`pkg/types/{resource,cluster}.go`、`cmd/{compose,install,cluster,root}.go`；新增 `cmd/validate.go`
- **配置**：`configs/**`（示例修正，配合严格解析与统一 schema）
- **测试**：删除 `test/unit/`、`test/coverage_matrix_test.go`、`test/contract/config_test.go`；`test/contract/` → `test/local/`；新增 `test/local/*`、`test/remote/*`、`test/multinode/*`、`test/fixtures/multinode/`；更新 `test/matrix.yaml`
- **CI**：`.github/workflows/ci.yml`（去覆盖率门槛、加 `test/` 不得 import `pkg/` 的 grep 守卫、`feat-*` 触发 + `workflow_dispatch`）、`integration.yml`（新增）、删除 `test.yml`
- **文档**：`README.md` 与 `doc/*.md` 中的命令名/标志修正（仅为通过 SC-X04，不做结构重建）
- **变更记录**：`changes/changelog/0.2.0-alpha.md`（三项 BREAKING 单列）

## 验收标准

- [x] `test/matrix.yaml` 中 24 个 `phase: 0` 场景全部 `status: done`，每条 `files` 指向的用例真实存在
      （2026-08-19 两条流水线全绿后置 done：CI run 32251103753、Integration run 32251103809）
- [x] `test/` 下无任何 `github.com/structure-projects/somcli` import（CI 守卫生效）
- [x] `ci.yml` 中不存在 `MIN_COVERAGE` / `-coverpkg`
- [x] `gofmt -l .` 为空；`go vet ./...` 干净
- [x] SC-E05 负向断言通过：目标节点有文件、未点名节点与操作机都没有（D1 不可能复现）
- [x] 任意失败路径退出码非 0 且输出无 `[SUCCESS]`（D2/F4/G8 不可能复现）
- [x] `somcli <任意命令> --help` 前后文件树快照一致（D9）
- [x] 文档中出现的每条命令与标志，在二进制上 `--help` 可验证存在（SC-X04）
- [x] 每条新增用例都已在缺陷未修版本上确认会失败（`48530cc` 上 20 个用例转红）
- [x] changelog 已补条目，三项 BREAKING 单列
- [x] 已过评审，MUST fix 全修（见 `review.md` 与「评审阶段补记」）

## 任务清单

详见 `tasks.md`。