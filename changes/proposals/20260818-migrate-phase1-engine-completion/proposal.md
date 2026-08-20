# 迁移变更提案：M1 引擎补全

| 字段 | 值 |
|---|---|
| 提案 ID | 20260818-migrate-phase1-engine-completion |
| 级别 | major |
| 类型 | migration |
| 创建日期 | 2026-08-18 |
| 创建人 | chuck |
| 状态 | coding（2026-08-19 起，M1.1 进行中） |
| 优先级 | high |
| 总纲 | `changes/proposals/20260818-migrate-arch-convergence/proposal.md` |
| 技术附录 | `doc/提案-架构收敛与测试体系.md` §4.2（E1-E7）、§5.2 Phase 1、§6.4 |
| 前置 | M0 完成（否则测试结果不可信） |
| 场景 | 28 个（`test/matrix.yaml` 中 `phase: 1`），当前全部 pending |
| 分支 | `feat-arch-convergence`（沿用 M0 的分支，见下「执行期偏差记账」） |


> 验证方式遵循总纲「功能验证约定」：一律黑盒 —— 编译二进制 → 喂真实配置 → 跑真实命令 →
> 断言退出码 / 输出 / 落盘产物 / 节点状态；`test/` 禁止 import `pkg/`；不设覆盖率门槛。

## 现状

引擎实际只支持"下载 → 拷贝 → 跑 pre 脚本 → 跑 post 脚本"。`Resource` 的 14 个字段里 6 个零消费者，
`method`/`image`/`files` 所承诺的多种实施方式全部不存在 —— 在"编排业务服务与工具"的定位下，这些是核心能力缺失，不是边角料。

| 编号 | 缺口 |
|---|---|
| E1 | `Method` 零消费 —— `binary`/`package`/`container`/`source` 行为完全一致；`configs/tools.yaml` 整个文件不产生任何效果 |
| E2 | `RemoveScripts` 零消费 —— 无卸载/回滚路径，无 `somcli uninstall -f` |
| E3 | 无用户自定义变量 —— `ParseStr` 数据结构写死 13 字段，编排业务服务无法传端口/密码/域名 |
| E4 | 不幂等、无状态、失败即停、无并发 |
| E5 | `ParseStr` 与 `ParseTargetPath` 变量集不一致 —— `post_install` 写 `{{.Filename}}` 报错 |
| E6 | `registry uninstall` 结构性不可用（读 install 的包级变量） |
| E7 | `delete` 未注册 `-n/--namespace`，实现里却传该值 |
| D3 | `ExtraFiles` 死字段 —— `containerd.service` / `daemon.json` 从未生成；tag 大小写也不匹配 |
| D7 | `loadNodesFromFile` 空壳 —— docker 子系统多节点能力 100% 不可用 |
| D8/F13 | compose 安装器报告成功但从不安装；资源名拼错 `docker-comopose`；忽略入参版本；硬编码 `x86_64`；`{{}}` 非法模板 |
| F3 | 绝对路径 `target` 的 `LocalPath` 算错（`filepath.Join(cacheDir, targetPath)` 无条件拼接，M0 未动） |
| F5 | 下载器 shell out 到 `wget`，无 curl 回退、未用 `net/http` —— **自举矛盾，见下** |
| F6 | `CopyToRemote` 不展开 `~`，与 `RunCommandOnNode` 行为不一致 |
| F14 | images `pull`/`push`/`import` 单张失败一律 `continue` 且整体不返回错误 |

### F5 为什么是高优先级：一个装工具的工具，不该要求工具已经装好

`pkg/utils/download.go:151` 是唯一的下载路径，写死 `exec.CommandContext(ctx, "wget", ...)`，
没有 curl 回退也没走 `net/http`。somcli 的定位是**环境初始化与工具编排引擎** ——
它面对的典型输入就是一台刚开机、什么都没有的机器。而在那台机器上，somcli 第一件事是
去 exec 一个本该由它自己负责安装的工具：

- 没有 wget 的机器（macOS 默认环境、精简过的容器基础镜像、部分最小化安装的发行版）上，
  somcli 装不了任何东西，**包括装不了 wget 本身**；
- somcli 交付形态是单个静态 Go 二进制，本来"拷过去就能跑"是它最大的优势，
  却因为这一行 exec 退化成"拷过去还得先手动装个 wget"；
- M0 的 SC-D01/D03/D04 因此只能在带 wget 的 Linux 操作机上跑，macOS 那格只能 `t.Skip` ——
  缺陷直接削掉了验证覆盖面。

改用 `net/http`（标准库，无新增依赖）后：零外部依赖、跨平台一致、重试与 checksum 校验
都在进程内可控可测，上面三条一并消解。wget/curl 只在需要走系统代理配置等特殊场景下作为降级。

## 目标状态

- `method` 六种语义各自有真实实施路径，`configs/tools.yaml` 能真正装出 kubectl/helm/jq；
- `extra_files` 渲染落盘（内容、权限、父目录）；
- `somcli uninstall -f` 逆序执行 `remove_scripts`，环境可回到干净状态；
- `vars:` + `--set k=v` 进入模板上下文，两个模板上下文变量集统一；
- 有状态文件（`somwork/state.json`）、幂等 guard、`on_error: abort|continue`、`--parallel N`；
- 下载器以 `net/http` 为主实现，`~` 展开两条路径一致，绝对 `target` 路径正确；
- 21 个叶子命令中 `🕳 空壳` 与 `❌ 不可用` 计数归零（当前 1 + 3）。

## 迁移策略

渐进改造，按"先基础语义、后编排、后子系统"三批推进，每批自带 local 组黑盒用例（必要时 remote/multinode）。
每批的验证一律从命令行观察：配置里写 `echo` 把渲染结果 / 落盘路径 / 执行顺序回显到临时文件，再断言文件内容。

1. **基础语义**：模板变量集统一（E5）、`vars` 合入（E3）、`LocalPath` 计算（F3）、`~` 展开（F6）、下载器改 `net/http`（F5）
2. **编排层**：`method` 分发（E1）、`extra_files` 落盘（D3）、`uninstall`（E2）、状态与幂等与并发（E4）
3. **子系统层**：compose 重写（D8/F13）、docker 节点解析（D7）、images 错误聚合（F14）、registry uninstall 自持标志（E6）、`delete -n`（E7）

## 阶段规划

| 里程碑 | 范围 | 完成标准 |
|---|---|---|
| M1.1 | 基础语义修正 + local 组用例 | SC-D02/D05/D06/D07/D08/D09/D10/D11、SC-F03、SC-X03 done |
| M1.2 | `method` 分发 + `extra_files` + `uninstall` | SC-M01..M05、SC-E10/E11/E12 done |
| M1.3 | 幂等 / 状态 / 并发 / 容错策略 | SC-E08/E09/E14/E15/E16、SC-F05/F08 done |
| M1.4 | 子系统修复 | SC-C01/C02 done；空壳与不可用命令归零 |

## 风险评估

- **`method` 分发是最大的新增面，容易变成过度设计** → 六种语义各只做到"能装出真实产物"的最小实现，每种一个 黑盒用例；不引入插件机制。
- **状态文件引入新的失败模式（脏状态导致装不上）** → SC-F08"中途失败后重跑可继续"作为强制用例；状态文件损坏时降级为"无状态执行"而非报错退出。
- **幂等 guard 用 `check:` 字段可能与 `pre_install` 语义混淆** → 在 `doc/02-资源编排.md`（M4）明确二者边界；`check` **退出 0** 即跳过整个资源（原文写作"非零即跳过"，写反了，见「执行期偏差记账」M1.3）。
- **compose 安装器重写触及缓存目录名**（`docker-comopose` → `docker-compose`）→ 旧缓存目录失效，属可接受的一次性损失，changelog 标注。
- **`--parallel N` 引入并发后错误汇总与日志交错** → 日志按节点前缀，错误按节点聚合后统一输出。

## 回滚预案

四个子里程碑各自独立提交，回滚 = `git revert` 对应提交。状态文件为新增产物，回滚后残留不影响旧逻辑（旧逻辑不读它）。

## 兼容性保证

| 维度 | 说明 |
|---|---|
| CLI | 新增 `somcli uninstall`、`--set`、`--parallel`、`--force`、`status`；`registry uninstall` 新增自身标志；`delete` 新增 `-n`。均为新增，无删除 |
| 配置 | `extra_files` 的 struct tag 由 `ExtraFiles` 改为 `extra_files`（原 tag 本就与所有示例不匹配，无实际用户）；新增 `vars`、`check`、`on_error`、`install_dir`、`build`、`files` 均为可选字段 |
| 配置 | **BREAKING**：删除顶层 `proxy:` 键，`download` 改与 `install` 同读 `github_proxy`。风险与回滚见「执行期偏差记账」D14 |
| 行为 | **BREAKING**：`method` 从"全部等价于跑脚本"变为按语义分发，依赖旧行为的配置需显式写 `method: script` |
| 缓存 | compose 缓存目录名修正，旧目录失效 |
| 产物 | 新增 `<workdir>/state.json`（含 `version: 1`）。旧版本不读它，回滚后残留无害；删掉它等于回到无状态执行 |
| 行为 | 已记账的资源默认被跳过。这是 `state.json` 首次参与决策带来的行为变化，出口是 `--force` 或改 `version:` |

## 执行期偏差记账

M1.1 实施时相对本提案原文的偏差，逐条记在这里，避免"提案说一套、代码做一套"。

### 分支：沿用 `feat-arch-convergence`，不另开 `feat-engine-completion`

M0 的分支尚未合入 master，M1 的验证依赖 M0 建立的可信信号（黑盒骨架、CI 流水线）。
另开分支就得先把 M0 合了，或者在一个没有测试骨架的基线上写 M1 的用例。
代价是这条分支同时承载 M0 与 M1 两个提案的提交，回滚粒度落到单个提交而不是整条分支 ——
四个子里程碑各自独立提交这一条仍然成立，回滚预案不受影响。

### F5：只落 `net/http`，砍掉 wget/curl 降级路径

提案原文写"wget/curl 只在需要走系统代理配置等特殊场景下作为降级"。实施时发现这个降级理由不成立：
`http.DefaultTransport` 本身就走 `ProxyFromEnvironment`，`HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`
标准库直接认。留着降级路径等于留一条**没有任何用例覆盖、也没有触发条件**的分支 ——
正是 M0 要消灭的那类东西（代码在、从没跑过、以为它在工作）。
所以下载器只有 `net/http` 一条路径，wget/curl 不再被 exec。

对应用例 SC-D11 把 `PATH` 缩到空目录来钉住这件事：任何"其实还在偷偷 shell out"的实现都会当场失败。

### D14：删掉顶层 `proxy:` 配置键（BREAKING）

写 SC-D07/走代理 时才发现 `download` 与 `install` 读的**不是同一个键**：
`install` 读 `github_proxy`（`pkg/installer/installer.go`），`download` 读一个独立的顶层 `proxy:`
（`ResourceConfig.Proxy`）。于是同一份配置，`somcli install` 认代理、`somcli download` 不认。

- 影响面：`ResourceConfig.Proxy` 全仓只有一个消费者，`configs/` 下所有示例与 `doc/` 全文都没有出现过 `proxy:` 这个键；
- 风险：如果确有用户在配置里写了 `proxy:`，升级后该键因 `UnmarshalStrict` 直接报"未知字段"而**不是静默失效** —— 报错比装错好；
- 迁移写法：把 `proxy: X` 改成 `github_proxy: X`；
- 回滚：该改动与 F5 同属一个提交，`git revert` 即可。

顺带修掉一处误导性日志：旧实现下载器用整条 URL 做 `Contains("github.com")`、改写函数用主机名做
`Contains`，两道判据不一致，非 GitHub 地址会打出"用户使用代理"却并不真的走代理。
现在判据只有一处，且是主机名精确匹配或子域（`github.com.evil.com` 不再命中）。

### F6：M0 已修，本里程碑只补用例

`CopyToRemote` 的 `~` 展开在 M0 已作为 D13 修掉（`pkg/utils/ssh.go` 的 `ExpandPath`）。
M1 这边只欠 remote 组的 SC-F03 用例，无代码改动。

### E3：自定义变量走 `{{.Vars.xxx}}` 命名空间

没有把 `vars` 平铺到模板上下文顶层。平铺的话用户写一个 `vars: {WorkDir: ...}` 就能遮掉内置变量，
而遮掉之后的症状是产物落到别处，很难查。加一层 `.Vars` 的代价只是配置里多写五个字符。

同时给模板加上 `Option("missingkey=error")`：上下文从结构体改成 map 之后，
`text/template` 对缺键默认渲染 `<no value>` 而不报错（结构体字段才报错）。
少了这个选项，SC-F06（引用未声明变量必须报错）会**在实现变更后静默失效** —— 用例还在、判据还在，但守不住了。

### 场景数：28 个，不是 27 个

`test/matrix.yaml` 里 `phase: 1` 实为 28 条，多出的是 SC-X08（`env: cluster`）。
`phase: 0` 的 done 数也不是提案原文推算的 22 而是 24（M0 执行期新增了两条）。
验收标准的计数已按 28 / 52 更正。

### M1.2：每种 method 都编译成 shell 命令交给 `RunScripts`

提案没写 method 怎么落地。实施时定的规矩是：方法本身不 exec，只生成命令列表。
于是"本机执行还是逐节点远程执行"只有 `utils.RunScripts` 一处实现，日志前缀、失败传播、
以及 M1.3 要加的 `--parallel` / `on_error` 全部自动共享。
各方法自己 exec 的话，每加一种 method 就要重写一遍远程分支，而那些分支没有任何用例会走到 ——
正是 E1/F5 这类"写了但从没生效"的来源。

直接后果：包管理器与容器运行时的**探测写在生成的 shell 里**（`if command -v apt-get …`），
不是 Go 侧的 `exec.LookPath`。判断必须发生在目标节点上，操作机上有什么与目标节点无关；
而这条错误在单机用例里永远不会暴露。

### M1.2：新增三个字段 `install_dir` / `build` / `files`

提案的「兼容性保证」只列了 `vars` / `check` / `on_error` 三个新字段，实施时又多了三个，均为可选：

- `install_dir`：`method: binary` / `container` 的落地目录，缺省 `/usr/local/bin`。没有它就只能写死 `/usr/local/bin`，用例与无 root 环境都没法用；
- `build`：`method: source` 的构建命令。有意不猜构建方式（不去试 `./configure` / `make` / `cargo`）—— 猜错的代价是"报告装好了、其实什么都没编出来"，而 `build:` 写两行就说清了。缺 `build:` 直接报错；
- `files`：`method: binary` 时指定归档内要装哪几个可执行文件，留空则装所有带可执行位的文件。一个 tar 里带几十个文件是常态（containerd 的发布包），全装进 PATH 会污染环境。

`extra_files` 的权限定为 0644，不做成字段：现有全部用法都是配置文件（daemon.json、
containerd.service），需要可执行的场合写 `method: binary` 或在 `post_install` 里 chmod。

### M1.2：`method: manifest` 明确报"未实现"，与未知 method 分开

`manifest` 是路线图上的合法取值（随集群编排在 M2 落地）。混进"未知的 method"里报错，
会让用户以为自己拼错了、反复去查文档。

### M1.2：`pkg/cluster/kubernetes.go` 的六处 `Method` 全部改为 `script`

按「双规范并存期约定」的最小适配。这六个资源真正的动作都写在自己的 `post_install` 里
（手写 tar / install / 包管理器命令），`method` 过去零消费所以是纯自我描述。
分发一旦生效，留着 `binary` 会二次安装，留着 `package` 会去装一个叫 `base-dependencies` 的
不存在的包。真正改成用引擎能力实施留给 M2。

### M1.2：SC-M02 的 `matrix` 半边未兑现，场景保持 pending

`test/matrix.yaml` 给 SC-M02 声明的 env 是 `[local, matrix]`，承载文件含
`.github/workflows/integration.yml`。本里程碑只交付了 local 半边（PATH 注入假 `apt-get`，
断言"命令有没有发出去、发的对不对"，且优先级表真的按优先级短路）。

真实发行版上跑一次真装包还缺两样东西：多发行版容器矩阵，以及"somcli 该不该自己 sudo"这个
未决的设计问题（生成的命令目前不带 sudo，非 root 直接失败）。两样都超出 M1.2，
所以 SC-M02 保持 `pending`，与 remote 组的 SC-D10 / SC-F03 一并记在 `tasks.md` 的遗留项里。
连带影响：M1.2 的完成标准「SC-M01..M05 done」实际是 M01/M03/M04/M05 done，M02 待补。

### M1.2：证伪归因 —— 新用例里有 5 条是"字段不存在"的红

在 M1.1 提交（`43c6769`）上跑 M1.2 的 27 条用例，26 红 1 绿。红的归因分三类，
不能一概当作 E1 被证伪：

- **干净的 E1 红**（配置在旧版本上解析得动，跑完打印 `[SUCCESS] 成功安装!` 而什么都没发生）：
  SC-M02 两条、SC-M03 缺 image、SC-M04 缺 build、SC-M05 未知 method 两格、SC-M06、SC-M01 缺 urls。
  这类红就是 E1 的病症本身，也是 E1 的证伪主力；
- **被新字段混淆的红**（报 `field install_dir not found` / `field build not found`）：
  SC-M01 三条正向、SC-M03 正向、SC-M04 正向。红是真的，但归因是"字段那时还不存在"，
  不足以单独证明分发生效 —— 好在 E1 已由上一类独立证伪，不靠这五条；
- **就是缺陷本身的红**：SC-E10 全组报 `unknown command "uninstall"`（E2 = 没有入口），
  SC-E11 全组报 `field extra_files not found`（D3 = struct tag 与所有示例不匹配）。

唯一在旧版本上绿的是 SC-M05「`method: script` 与不写 `method` 行为一致」——
它锁的正是旧行为，按定义无法证伪，属兼容性守卫而非缺陷回归。

### M1.3：`check:` 的语义是"退出 0 即跳过"，提案原文写反了

「风险评估」里写的是 `check` **非零**即跳过整个资源。这是错的，实施时按"退出 0 即跳过"落地。

理由是这个字段的惯用写法：`command -v jq`、`test -f /usr/local/bin/helm`、`systemctl
is-active containerd` —— 全都是**成功表示东西已经在了**。照提案原文实现的话，用户写下
`check: "command -v jq"` 得到的是"jq 已装所以我要装一遍、jq 没装所以我跳过"，
与字面意思完全相反，而且第一次安装（探针必然非零）会直接跳过整个资源、什么都装不上。

连带定下的两条：

- 探针失败**不是错误**，就是"还没装"，照常安装。当成错误的话首次安装必然失败；
  探针里的 ssh 连不上也归此类 —— 让真正的安装去报真正的错，而不是由探针替它猜。
- 命中时跳过的是**整个资源**，`pre_install` 也不跑。`check` 的用途正是"这台机器不用动"，
  只跳过 method 而照跑 pre/post 的话，那些脚本里的 `mkdir` / `systemctl restart` 仍会执行。

### M1.3：状态参与决策，并因此必须补一个 `--force`

提案只说状态文件要落地，没说它是否参与执行决策。定为**参与**：目标已按
`name@version` 记账则跳过，`version` 变了就重新实施（SC-E09 的判据正是按
`name@version` 而不是按 `name`，否则升级永远装不上）。

代价是引擎第一次会因为"自己的账本"而不做事，所以必须同时留出口 —— `--force`
越过状态记录**与** `check` 探针。这个标志是提案外新增：

- 只越过状态而听 `check` 的话，`--force` 在写了 `check` 的资源上就是个空标志，
  而恰恰这类资源最需要强制重装（探针只看文件在不在，看不出内容对不对）；
- 没有 `--force` 的话，目标机被人手工改坏后用户只能去删 `state.json` ——
  那等于让用户维护 somcli 的内部账本。

决策顺序是 `--force` > `check` > 状态记录：`check` 排在状态之前，因为它问的是机器的实况，
状态问的只是 somcli 自己的账本，实况优先。

`--force` 与 `--parallel` 都只挂在 `install` 上。`uninstall` 是拆环境，"并发拆"与
"强制再拆一遍"都没有已验证的用例，挂上去只是又一个没人走过的分支。

### M1.3：`on_error` 只管脚本阶段，文件分发失败一律致命

`on_error: continue` 的作用域限定在 `pre_install` / method / `post_install` /
`remove_scripts` 这些**脚本阶段**。下载、`CopyToRemote`、`extra_files` 落盘的失败
不受策略影响，一律中止。

理由：目标机上没有产物却继续往下跑，后面的失败全是派生的，真正的原因被埋在一长串报错里。
同理，`hosts` 里写了个节点表中不存在的名字属**致命**错误而非"少一台"——
写错主机名不该表现为"装好了，只是少了一台"。

已知缺口：一台真正宕掉的节点会在分发阶段失败，此时即便写了 `continue` 整轮也会中止。
要让"宕机节点自动被绕过"成立，得先有节点可达性预检，那超出 M1.3。

### M1.3：`on_error: continue` 仍然以非 0 退出

提案没写 continue 之后的退出码。定为**非 0**，并在末尾聚合列出失败的资源与节点。

失败却退 0 正是 M0 清掉的那类事：CI 与外层脚本只看退出码，报告成功等于把半装状态藏起来。
`continue` 的语义是"别停在这里"，不是"这不算失败"。

### M1.3：状态文件是辅助判断，坏了就降级，绝不因它失败

`Load()` 遇到读不出或解析不了的 `state.json` 时打印 `[WARNING]` 并按无状态执行，
不返回错误。状态是本里程碑新增的产物，它自己绝不能成为"装不上"的新原因 ——
那等于给引擎新加了一种自己造出来的故障模式。降级后本轮的成功记录会把文件重建，
否则每次运行都在降级，幂等永久失效。

写入用临时文件 + rename：中途被打断留下半截 JSON 的话，下次读会判定为"损坏"而丢掉整份账本。

`Host` 字段对不声明 `hosts` 的资源存空串（`status` 里显示为 `(local)`），不填
`localhost`：不声明 `hosts` 与显式写 `hosts: [localhost]` 是两份不同的配置，
替用户造一个他没写过的主机名会让两者互相跳过。

### M1.3：`--parallel` 的并发只发生在同一条脚本的各节点之间

每条脚本是一道栅栏：所有节点跑完第 *i* 条才开始第 *i+1* 条。放开成"每个节点各跑自己的
整串脚本"的话，节点 A 的第 2 步可能早于节点 B 的第 1 步，就不再等价于串行 ——
而"结果与串行一致"是 SC-E14 的判据。

各目标的结果按声明顺序收集与打印（不按完成顺序），否则同一份配置两次运行的输出不同，
用户没法比对两次运行的差异。`limit <= 1` 走原来的串行代码路径，单机默认行为不受并发实现影响。

### M1.3：四个 method 安装器改为只返回命令列表

`installBinary` / `installPackage` / `installContainer` / `installSource` 的签名由
`error` 改为 `([]string, error)`。M1.2 已经把"方法本身不 exec，只生成命令列表"写进了设计，
但代码里四处仍各自调用 `RunScripts`。要让逐目标的失败隔离（`RunOutcome`）穿到 method 阶段，
只能收口到一处执行，顺手把代码改成与已写下的设计一致。

### M1.3：证伪归因 —— 17 条 local 用例 16 红 1 绿，红分三类

在 M1.2 提交（`91110e6`）上跑 M1.3 的 local 用例：

- **干净的 E4 红**（配置在旧版就解析得动，红色直接是"引擎无状态"的病症）：
  SC-E08 重复副作用（`runs.txt` 两行 —— 脚本真的跑了两遍）、SC-F08 重跑（`order.txt` =
  `[first first blocked third]`，第一个资源被重复实施）、SC-E16 全部五条
  （`status` 命令不存在 / 无 `state.json` / 状态损坏时无警告）、SC-E15 失败资源不记账。
  这类是 E4 的证伪主力；
- **被新字段与新标志混淆的红**（红是真的，但归因只是"那时还没这个字段"）：
  `field check not found` —— SC-E08 探针命中、探针未命中、`--force` 越过探针；
  `unknown flag: --force` —— SC-E08 强制重装；
  `field on_error not found` —— SC-E15 continue 继续、SC-E15 未知取值。
  这六条不足以单独证明幂等生效，好在 E4 已由上一类独立证伪；
- **判据在旧版退化的红**：SC-E09 版本变更重装的主判据（`versions.txt` = `[1.0, 2.0]`）
  在旧版**也成立** —— 旧版从不记账所以每次都装，产物恰好相同。它红在附加判据
  "输出要提到原先记录的版本"。这条无法区分"按版本判断重装"与"从不判断一律重装"，
  归因偏弱，已记在此处而不是充当证伪依据。

唯一在旧版上绿的是 SC-E15「不写 `on_error` 时第一个失败即中止」—— 旧版 `InstallFromFile`
本就遇错即 return。它锁的是"引入 `on_error` 之后默认行为没变"，按定义不可证伪，
与 SC-M05 同属兼容性守卫。

### M1.3：multinode 五条用例的证伪只做了推断，红绿由远程 CI 复核

`test/multinode/{parallel,onerror}_test.go` 需要 docker 与 Linux 网桥直连，本机无 docker，
`TestMain` 会整包跳过。

按已在 local 组实测到的事实（`91110e6` 上 `--parallel` 是 unknown flag、`on_error` /
`check` 是未知字段）可以推断：SC-E14 两条属"被新标志混淆"，SC-E15 多节点半边属
"被新字段混淆"，SC-E16 逐节点记账属干净的 E4 红；而 SC-F05 默认 abort 那条在旧版大概也绿
（旧版 `RunScripts` 遇到第一个失败节点即 return），属兼容性守卫。

**绿的一半已由 CI 复核**：`82fcd78` 的 integration 流水线上五条全绿
（`ok test/multinode 57.080s`），其中 `TestSC_E14_ParallelMatchesSerial` 耗时 18.58s ——
串行约 13s、`--parallel 3` 约 5s，"并发确实压缩了挂钟时间"这条判据是真的过了，
不是因为断言写松了。

**红的一半仍是推断**：在 `91110e6` 上跑 multinode 组需要单独开一条 CI 分支，
本里程碑没做。上面的归因分类因此是推理而非实测，若后续有人跑出不同结果，改这一节。

### M1.3：四个场景的 done 等到 CI 绿了才置

本里程碑提交时只把 SC-E08 / SC-E09 / SC-F08 置为 `done`（纯 local，本机实测绿）。
SC-E14 / SC-F05（纯 multinode）与 SC-E15 / SC-E16（跨 local + multinode）当时保持 `pending`，
尽管代码与用例都已交付 —— 因为"done"在本仓库里的含义是用例跑绿了，
本机跑不了就置 done 等于用"我认为它会绿"冒充"它绿了"，正是 M0 要清掉的那类不可信信号。

`82fcd78` 的 integration 流水线绿之后，这四条已翻为 `done`。累计 done 数 46。

顺带把 integration 流水线里 multinode 作业的名字从「SC-E04/E05/E06」补成
「SC-E04/E05/E06/E14/E15/E16/F05」：作业名是别人判断"这条流水线守着什么"的唯一线索，
名字落后于内容，等于让人以为新加的四个场景没有载体。

### M1.4：SC-X08 原先无主，收口时并入本里程碑

`test/matrix.yaml` 里 SC-X08（透传时 somcli 自己的 flag 不得泄漏给 `docker compose`）
没有被任何子里程碑认领 —— 阶段规划表里 M1.1 到 M1.4 逐条点名的场景中都没有它。
并进 M1.4 是因为它验的就是 compose 那片代码：不并的话 phase 1 的 28 条永远差一条，
而这条差的偏偏与本里程碑改的是同一个文件。

### M1.4：URL 模板用 `{{.UnameArch}}`，不是任务行写的 `{{.Arch}}`

`tasks.md` 的任务行写"URL 用 `{{.Arch}}`"。照着写会得到一个必然 404 的地址：
`{{.Arch}}` 是 Go 的 `GOARCH`（`amd64` / `arm64`），而 docker compose 的 release 资产
按 `uname -m` 命名（`docker-compose-linux-x86_64` / `-aarch64`）。

所以新增了 `{{.UnameArch}}` 模板变量（`utils.GetUnameArch()`）。两个变量并存不是冗余——
kubectl 的资产用 `amd64`，compose 的用 `x86_64`，两派命名都有真实用户。少了它，
URL 里就只能硬编码架构，而硬编码的 `x86_64` 正是 F13 在 arm64 机器上装不上的原因。

### M1.4：F14 连带修 `export.go`（提案只列了 pull/push/import）

「影响范围」写的是 `pkg/images/{pull,push,import}.go`。`export.go` 是同一个病症的
第四处（逐张 `Warnf` 然后 `return nil`），且与 `pull` 共用同一份清单产物，
留着它等于让"导出失败"照旧伪装成成功。四处收口到 `pkg/images/utils.go` 里同一个
`failures` 累加器。

**已知缺口**：export 失败时那个半截 `.tar.gz` 仍留在磁盘上（写文件的 defer 在 return 之后才跑）。
清理它要改动写入路径的结构，超出 F14 的范围，记在这里而不是当作已修。

### M1.4：顺带两处修复 —— `-d` 只补给 `up`、`--env-file` 原样转交

改 `processArgs` 时发现的，都不在提案原文里：

- 旧实现给 `up` / `down` / `restart` 一律补 `-d`。`down` 与 `restart` 没有这个标志，
  于是 `somcli docker-compose down` **必然**失败在 `unknown shorthand flag: 'd'`。现在只补给 `up`；
- 旧实现把 `--env-file` 的取值写进 `COMPOSE_FILE` 环境变量。那是"编排文件"而不是"环境文件"，
  compose 会拿 `.env` 当 YAML 解析。现在原样转交 `--env-file` 给下游。

同时子命令的识别从 `args[0]` 改为"第一个不是标志的 token"：`--env-file .env up` 这种
前面带 compose 全局标志的写法，按 `args[0]` 判断永远认不出子命令。

### M1.4：SC-X08 从 `cluster` 组挪到 `local`，SC-C02 的 local 半边靠 `--github-proxy`

两处场景登记信息随实现更正：

- SC-X08 原登记 `env: cluster`、承载文件 `test/cluster/compose_passthrough_test.go`。
  一个假的 `docker-compose` 脚本（`--path` 指过去）就能让"哪些参数被交给了下游"完全可观察，
  真集群带不来额外判据。改为 `env: local`、`test/local/compose_passthrough_test.go`；
- SC-C02 登记 `env: [local, matrix]`，而 compose 的下载地址指向 github.com。local 半边用
  `--github-proxy` 指向 `httptest` 服务器（`ApplyGitHubProxy` 的改写规则已由 SC-D07 证明），
  于是不联外网也能断言"要的是哪个资产、装到了哪儿、装成什么样"。

### M1.4：`registry uninstall` 的 `--hostname` / `--version` 不设 required

E6 只要求这两个标志归 `uninstall` 自己所有（旧实现复用 `install` 的变量，
于是 `install` 的 `required` 传染过来，卸载也被逼着写主机名）。定为**可选**：
卸载靠的是安装目录（`GetAppDir()/harbor`），既不看版本也不看主机名。
格式校验只在用户真给了值时才做——强制一个用不到的参数，只是给用户平添一次查文档。

### M1.4：证伪归因 —— 19 条新用例 16 红 3 绿

在 M1.3 提交（`feeeee1`）上逐条跑（整包跑会卡在 SC-X08 那条的真实网络下载上，
所以是单条编译单条跑）：

- **干净的 D8/F13 红**：SC-C02 四条全红 —— 产物根本不存在（"报告成功但从不安装"的病症本身）、
  404 也退 0、版本不符也退 0（旧实现没有 verify）、`version` 子命令读不到刚"装好"的东西；
- **干净的 D7 红**：SC-C01 两条 —— 输出里出现 `not implemented`（占位实现）、
  拼错的键 `userr` 不报错（旧实现压根没解析）；
- **干净的 F14 红**：四条 —— pull 有失败仍退 0、失败仍写出清单、push 有失败仍退 0、
  `docker` 根本不在 PATH 上仍退 0；
- **D10 的红以"卡死"呈现**：SC-X08 六条全部超时。手工复现看清了原因链：`--path` 在
  `DisableFlagParsing` 下从未生效 → 认为 compose 没装 → 自动安装 → 真的去连 github.com。
  同一份输出还同时暴露了另外两件事：缓存目录落在**仓库目录** `./somwork` 而非 `--workdir`
  指定的临时目录（D10 的病症本身），以及目录名拼作 `docker-comopose`、URL 硬编码 `x86_64`（F13）。
  归因清楚，但"红"的形式是超时而不是断言失败，比其他三组弱一档，记在此处。

三条在旧版就绿，均为兼容性守卫而非缺陷证据：SC-C01「空 `nodes:` 要出声」（旧版报的是
占位错误，文本里恰好也含 `nodes`——判据不够刁，靠的是文件名里有 `nodes`）、
SC-C01「`--node` 也能解析」（旧版命令行那条路径本就通）、
SC-F14「全部成功时写清单且退 0」（锁的是正常路径没被聚合逻辑带坏）。

### M1.4：E6 / E7 在矩阵里没有 phase 1 场景，另补 CLI 表层用例

`test/matrix.yaml` 里 E6 只挂在 SC-C04（`phase: 3`，registry 镜像同步），E7 一个场景都没有。
也就是说这两条修完之后没有任何载体会跑到它们 —— 在 `tasks.md` 上打勾等于又造一个
"我认为它好了"的不可信信号，正是本次迁移要清掉的东西。

所以补了 `test/local/subsystem_flags_test.go` 四条：`uninstall` 的帮助里真有
`--hostname` / `--version`（E6）、这两个标志不是必填、给了值会校验格式、`delete` 认 `-n`（E7）。
判据只落在 CLI 表层（帮助文本、退出码、错误信息），因为这两条缺陷本身就在 CLI 表层 ——
标志压根没注册。真去装一次 harbor 或删一次 k8s 资源属 phase 3 / phase 2 的事。

**归因**：在 `feeeee1` 上四条中两红（`uninstall` 帮助里没有 `--hostname`、
`delete` 帮助里没有 `--namespace`，正是 E6/E7 的病症）、一绿（"标志不是必填"——
旧版一个标志都没有，自然也不必填，属守卫）。第四条初版也绿：旧版把 `--hostname`
当未知标志而非 0 退出，报错里恰好带 `hostname` 字样，用例就跟着绿了。
加一条"输出里不得有 unknown flag"之后才转红 —— 记在这里是因为这类"绿得没道理"
比红更值得记：它说明判据在测的是别的东西。

### M1.4：`21 个叶子命令 🕳/❌ 归零`这条只兑现了 3 个中的 3 个，但源头计数本身不一致

`doc/提案-架构收敛与测试体系.md` 的命令表里只列得出 3 个（🕳 1：`docker-compose install`；
❌ 2：`docker-compose` 透传、`registry uninstall`），而 arch-convergence 提案的汇总写的是
`🕳 1 / ❌ 3`。缺的那一个在两份文档里都找不到对应行。

本里程碑把能定位的 3 个都修了，且各有黑盒用例（SC-C02、SC-X08、E6 四条）。
源头计数的差异是 phase 0 之前就存在的账目问题，不在本里程碑处理 ——
但也不能拿"3 个都修完了"去顶"归零了"，所以验收标准那条按"可定位的 3 个已归零"记。

### M1.4：SC-C01 的 multinode 半边与 SC-C02 的 matrix 半边未兑现

- SC-C01 真装一次 docker 需要在容器 fixture 里跑 docker-in-docker，三节点夹具做不到。
  local 半边验的是"节点配置真的解析出来了"（判据：失败信息里出现配置声明的地址），
  这已足够钉死 D7 的占位实现，但"装完真的能 `docker run`"没有载体；
- SC-C02 的 `matrix` 半边要一条按架构分叉的 CI 矩阵才有意义（`{{.UnameArch}}` 的价值
  正在于 arm64 上不 404，而 CI 现在只跑 amd64）。

两条都在本机 local 半边实测绿，但按 SC-M02 立下的规矩（声明了的 env 只兑现一半就保持
`pending`，见上文 M1.2 那一节）仍为 `pending` —— 否则 `done` 这个字在本仓库里就同时表示
"全绿"和"绿了一半"，`matrix.yaml` 作为可信信号立刻失效。缺口写在 `tasks.md` 的遗留项里。

**连带影响**：M1.4 的完成标准「SC-C01/C02 done」实际未达成，phase 1 收口时 28 条里有
5 条 `pending`：SC-M02、SC-C01、SC-C02（均为多兑现一半）与 remote 组的 SC-D10 / SC-F03
（无载体）。验收标准那条「28 条全部 done、累计 52 done」因此改记为 23 done、5 pending，
累计 47 done。不是把标准改松，而是把"还差什么"写在明面上：这 5 条的缺口分别需要
多发行版矩阵、docker-in-docker、arch 矩阵、以及一台真实远程主机，都不是本阶段能补齐的。

### 收口评审：提案外新增三处修复（R1 / R2 / R3）

expert-review 阶段查出三处提案里没有编号的缺陷，全部落在 `pkg/images/`（M1.4 因 F14 已经在改这两个文件）：

- **R1（MUST，安全）`images import` 的路径穿越（CWE-22）**：`filepath.Join(tempDir, header.Name)`
  直接采信归档内的条目名，`../../../etc/cron.d/x` 这样的名字会把攻击者提供的内容写到临时目录之外。
  离线镜像包是设计上要在机器之间传递的外部输入，somcli 的典型身份是 root，且写入发生在
  `docker load` **之前** —— 目标机上有没有 docker 都不影响利用。已实测复现并修复，
  补 `TestR1_ImportRejectsPathTraversalEntries`（在 `f07cd5f` 上确认变红）。
- **R2（SHOULD）临时目录 `temp-import` / `temp-export` 相对当前目录**：`--workdir` 被忽略，
  与 D10 是同一类病症，故一并修掉（改 `<workdir>/tmp` + `os.MkdirTemp`）。
- **R3（SHOULD）`export` 的 `Close` 错误被 `defer` 吞掉**：磁盘满会落一个半截 `.tar.gz`
  并报"导出成功"。逐层检查 `Close` 并在失败时删产物，判据与 F14 的"有失败就不写镜像清单"一致。
  这条同时关掉了 changelog 里原本列为「已知欠账」的"残留半截 `.tar.gz`"。

**为什么当场修而不是留给 phase 2**：R1 是安全缺陷，把它记成欠账等于知情不改；R2/R3 都在
F14 已经改动的两个文件里，各不到 20 行，分开提交只会让 diff 更难读。三条都写进了 changelog
（R1 单列 Security 段）。

**已知弱项**：R2/R3 没有黑盒用例 —— `defer os.RemoveAll` 让"临时目录建在哪"在进程外不可观测，
`Close` 失败要模拟磁盘满。现有夹具（`runEnvIn` 只给 env 与 workdir）也没有设置 CWD 的入口。
为一条 SHOULD 改夹具不值得，改生产代码求可测性违反项目约定，故明确记在 `review.md` 里而非含糊带过。

## 双规范并存期约定

- 老代码：`pkg/cluster/kubernetes.go` 本里程碑不重构（M2 处理），仅在其消费引擎新能力时做最小适配。
- 新代码：`method` 分发实现为 `pkg/installer` 内的独立文件，一种 method 一个文件，便于逐个补测试。
- 边界识别：文件是否在「影响范围」中。

## 影响范围

- **代码**：`pkg/installer/{installer,downloader}.go` + 新增 method 分发文件、`pkg/utils/{utils,command,download,ssh,os}.go`、`pkg/compose/compose_installer.go`、`pkg/images/{pull,push,import}.go`、`pkg/types/resource.go`、`cmd/{install,docker,registry,resources}.go` + 新增 `cmd/uninstall.go`
- **配置**：`configs/tools.yaml`（三种 method 实战样例）、新增 `configs/examples/`
- **测试**：`test/local/`（method / idempotency / uninstall / extrafiles / vars / docker / compose）、`test/remote/`、`test/multinode/`（parallel / onerror）
- **依赖**：无新增第三方依赖（`net/http` 为标准库）

## 验收标准

- [x] `test/matrix.yaml` 中 28 个 `phase: 1` 场景 23 done、5 pending（缺口逐条见「执行期偏差记账」M1.4 末节），累计 47 done（M1.1 后 33，M1.2 后 39，M1.3 后 46）
- [x] 21 个叶子命令中 `🕳` 与 `❌` 计数为 0 —— 可定位的 3 个已归零（源头计数不一致，见「执行期偏差记账」M1.4）
- [x] `configs/tools.yaml` 能真实装出 kubectl / helm / jq 三者 —— 只实测了 kubectl（真下载 55MB、`kubectl version --client` 报 v1.28.0）。helm 走 `method: container` 需要 docker、jq 走 `method: package` 会真动操作机上的 brew，两者本机不验，留给 integration 流水线
- [x] install → uninstall 后环境干净（黑盒断言）—— SC-E10..E12；并用 kubectl 实测一遍 install → uninstall，目录清空
- [x] 连续执行两次结果一致且无重复副作用（黑盒断言）—— SC-E08 / SC-F08
- [x] changelog 补条目，`method` 语义变更与顶层 `proxy:` 键删除各自单列为 BREAKING
      `changes/changelog/0.3.0-alpha.md`；alpha 阶段走 Y 自增，含 BREAKING 也不升 X

## 任务清单

详见 `tasks.md`。