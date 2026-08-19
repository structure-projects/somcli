# 任务清单：M0 建立可信信号

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选，并更新 `test/matrix.yaml` 中对应场景的
> `status` 与 `files`。清单由人维护、人评审，没有校验它的元测试。
>
> 所有用例一律黑盒：编译二进制 → 真实配置 → 真实命令 → 断言退出码 / 输出 / 落盘产物 / 节点状态。
> `test/` 下禁止 import `github.com/structure-projects/somcli/...`。

## 准备

- [x] 立验收清单：`test/matrix.yaml` 82 场景（`env` 分组：local / remote / multinode / cluster / matrix）
- [x] 阅读 `proposal.md` 与技术附录 `doc/提案-架构收敛与测试体系.md` §4 / §5.2
- [x] 切 `feat-arch-convergence` 分支（commit-msg hook 禁止在 master/develop 直接提交）
- [x] 定论待决事项 6：新增 `somcli validate -f <file>` 只读校验命令（用户定论）

## 已完成的代码修复（工作区未提交，待落库）

- [x] D2 `RunScripts` 聚合并向上返回错误（`pkg/utils/command.go`）
- [x] D1 `SetNode` 接线 + `GetNode` 去兜底改返回 error（`pkg/cluster/common.go:51`、`pkg/utils/utils.go`）
- [x] F1 `html/template` → `text/template`（`pkg/utils/utils.go`）
- [x] F4 `PrintSuccess` 移入成功分支（`pkg/installer/downloader.go:94`）
- [x] G8 下载失败影响退出码（`pkg/installer/downloader.go` + `cmd/install.go:102`）
- [x] D9 compose 根命令前置拦截 `--help`/`-h`，帮助无副作用（`cmd/compose.go`）
- [x] 三处解析改 `yaml.UnmarshalStrict`（utils / cluster / downloader）
- [x] `gofmt -w ./...`（`gofmt -l .` 输出为空）
- [x] `ci.yml` 取代无效的 `test.yml`

## M0.1 清除方向错误的产物（MUST 先于新增用例）

- [x] 删除 `test/unit/` 全部 7 个文件（白盒：直接调 `pkg/` 导出函数）
- [x] 删除 `test/coverage_matrix_test.go`（元测试：测的是测试自己）
- [x] 删除 `test/contract/config_test.go`（绕过二进制，且含"欠账清单自校验"）
- [x] `git mv test/contract test/local`，包名改 `local`
- [x] `ci.yml` 去掉 `MIN_COVERAGE` 与 `-coverpkg=./... -coverprofile=... -covermode=atomic`，保留 `-race`
- [x] `ci.yml` 的 `static` job 增一条守卫：`test/` 下不得出现 `structure-projects/somcli` import
- [x] `test/matrix.yaml` 把失去承载的 13 个场景回退为 `pending`，`layer` 改为 `env`
- [x] 三个已知不合法示例（`config.yaml` / `kubernetes-cluster.yaml` / `tools.yaml`）的欠账记入提案「兼容性保证」，不留白名单代码

## M0.2 落库（按语义拆提交）

- [x] `style: gofmt 格式化全部 Go 文件`（纯格式，独立提交）
- [x] `chore: 安装 structure-agent-rules 规则与 changes 骨架`（含 `wiki/`，拆为两提）
- [x] `docs(changes): 新增变更提案 20260818-migrate-arch-convergence 及 5 个阶段子提案`
- [x] `fix(utils): RunScripts 与 GetNode 不再吞错与兜底`（D2 + D1）
- [x] `fix(utils): 模板改用 text/template 避免 shell 字符被转义`（F1）
- [x] `fix(installer): 下载失败不再打印成功且影响退出码`（F4 + G8）
- [x] `fix(compose): 查看帮助不再触发下载与安装`（D9）
- [x] `fix(config): 未知配置键改为报错而非静默忽略`（严格解析 + `configs/**` 示例修正）
- [x] `test: 建立场景清单与黑盒用例`
- [x] `chore(ci): 用 ci.yml 取代恒绿的 test.yml`（hook 的合法 type 不含 `ci`，用 `chore(ci)`）

## M0.3 local 组黑盒用例（默认 `go test ./...` 就跑）

公共 helper（`test/local/main_test.go`）：编译二进制一次、`run(args...) (code, out)`、`writeConfig`、
`tree(dir)` 目录快照。全部只用 `os/exec` 与 `os`，不 import `pkg/`。

- [x] `engine_test.go` SC-E01：单资源，`post_install` 写标记文件 → 断言文件存在且内容正确
- [x] `engine_test.go` SC-E02：三个资源各 `echo name >> order.txt` → 断言行序等于 `resources` 数组顺序
- [x] `engine_test.go` SC-E07：`install -f cfg -n b` → 断言只有 b 的标记文件存在，a/c 未执行
      （用例逼出 D11：`-n/--name` 从未注册，`InstallTool` 是死代码 → 已在 `cmd/install.go` 接线）
- [x] `template_test.go` SC-E13：脚本上下文渲染变量并回显到文件后断言；F1 回归改为让 `'` 与 `&`
      出现在**值**里（workdir 路径与资源名）—— 原先拿字面量 `'a&b<c>d'` 测是假绿，html/template
      只转义插值结果，不动模板字面量，缺陷版本上那条断言照样通过
- [ ] `template_test.go` SC-E13 余下部分：URL / target 两处上下文
      → 结转 M1。它要断言的是"模板变量在 URL 与 target 里也渲染"，而这两处都在下载器
      那条路径上，M1 本来就要把下载器从 exec wget 改成 net/http 重写一遍。
      本里程碑不勾：CI 绿证明的是 SC-D01/D03/D04，不含这一条。
- [x] `template_test.go` SC-F06：配置写 `{{.NoSuchVar}}` → 退出码非 0，错误含变量名
- [x] `node_resolve_test.go` SC-F01：声明 node-a，`hosts: [node-x]` → 退出码非 0、错误含 `node-x`、**本机与 workdir 无任何副作用**（D1 回归）
- [x] `failure_test.go` SC-F04：`pre_install` 退出 3 → 非 0 退出、`post_install` 标记文件不存在、输出无 `[SUCCESS]`
- [x] `failure_test.go` SC-F02：`nodes` 指向 `192.0.2.1`（TEST-NET-1 保证不可达）→ 错误信息含节点名 / 用户 / IP
      （耗时 30s，等的是 somcli 自己的 `ConnectTimeout=30`，已 `t.Parallel()`）
- [x] `download_test.go` SC-D01：`httptest` 起本地源 + 正确 checksum → 产物落盘、内容一致
- [x] `download_test.go` SC-D03：源返回 500 → 退出码非 0、输出无 `[SUCCESS]`（F4/G8 回归）
- [x] `download_test.go` SC-D04：相对 / 绝对 `target` 两种语义各自落位
      上面三条在无 `wget` 的机器上 `t.Skip`（F5：下载器 shell out 到 wget，无回退），
      但 Linux 上缺 wget 改为**硬失败** —— 否则 `ci.yml` 不带 `-v`，日志里"跳过"和"通过"
      长得一模一样，这三条等于永远不会红的空壳。ubuntu / ubuntu-arm 两格绿即证明真跑了。
- [x] `workdir_test.go` SC-X01：两次不同 `--workdir` → 产物各自隔离；仓库目录无 `somwork` 新增（快照对比）
      判据取"本次运行前后仓库侧无变化"而非"somwork 不存在"，否则开发机上遗留的 somwork 会误报
- [x] `config_test.go` SC-X05：`configs/` 下每个示例经 `somcli validate -f` 校验 —— 断言退出码 0，且
      `validate` 声称只读则 workdir 前后快照必须一致；配套 `SC-X05_ValidateRejectsBrokenConfig`
      正向确认坏配置会被拒（未知键 / 模板变量不存在）
      （用例逼出 D12 `--source` 类型错、G9 一批从未消费的字段与 5 文档静默丢弃）
- [x] `config_test.go` SC-F07：未知字段 / 拼错键 / 重复键三类 → 均退出码非 0，错误指出键名
- [x] `config_test.go` SC-X07（新增场景）D9 回归：对 26 条叶子命令跑 `--help`，workdir 与 HOME 前后快照一致、退出码 0
      （用例逼出 D10：全局 flag 在 `--help` 之前时 `args[0]` 不是 `--help`，帮助拦截失效 → `isHelpRequest` 改为先跳过 root 的全局 flag）
- [x] `doc_commands_test.go` SC-X04：抽 `README.md` + `doc/*.md` 的 ```bash 块中 `somcli ...` 调用 → 对二进制执行 `<cmd> --help` 断言退出码 0；标志断言出现在 `--help` 输出中
- [x] 修文档中不存在的命令与标志（G4/G5，仅改名不改结构）：`docker-images`→`images`、`offline download`→`download`、`cluster deploy`→`cluster create`、`registry install -h`→`-H`；删除未注册的 `docker uninstall --force` / `apply -f` / `images --username|--password` / compose `-v|-p`
- [x] 每条用例在缺陷未修版本上确认会失败：`git worktree` 检出 `48530cc`（全部 fix 之前）+ 覆盖当前
      `test/local/*.go` → 20 个用例转红，且红的正是各自负责回归的缺陷

## M0.7 统一配置 schema（BREAKING，用户定论"一份配置所有场景都能加载"）

原先每类命令各认一套 schema：`install -f` 读 `resources:`，`cluster create -f` 读的是另一套
以 `cluster:` 为根的映射，`images --custom-file` 又是第三种。同一份文件换个场景必然解析失败
（严格解析之后更是直接报未知键）。改为单一 `types.ResourceConfig`：全局设置 + `resources` +
`nodes` + `cluster` + `images` 共存，各命令只取自己那一段，别人的段落原样放着不影响解析。

- [x] `types.ResourceConfig` 纳入 `Clusters []ClusterSpec` 与 `Images []Image`
- [x] `cluster:` 由映射改为**列表**（用户定论）：一份文件可同时描述 my-swarm 与 my-k8s
- [x] 拆出 `types.ClusterSpec`，`ClusterConfig` 保留为"已选定的那一套"（存量 `config.Cluster.X` 访问不变）
- [x] `cluster.LoadConfig(file, name, type)` 按 name → type → 唯一匹配 依次判定，定不下来就报错并列出候选，不默默取第一套
- [x] `cluster create/remove` 增 `--cluster-name`，`remove` 补 `--cluster-type`
- [x] `images --custom-file` 兼容三种输入：统一配置的 `images:` 段 / 裸列表 / 纯文本 `name:tag` 行
- [x] 8 个示例配置全部改到新 schema 并经 `validate` 通过；`configs/config.yaml` 同时含四段作为"一份配置覆盖所有场景"的样例
- [x] `configs/kubernetes-cluster.yaml` 由 5 个 YAML 文档（后 4 段被静默丢弃）合成单文档
- [x] `validate` 拒绝多文档文件，明说 `---` 之后的内容不会生效
- [x] changelog 第三项 BREAKING：`cluster:` 映射 → 列表

## M0.4 remote 组（SC-E03）

- [x] `test/remote/` 建包，build tag `remote`，未满足前置时 `t.Skip` 并说明原因（CI 里改为硬失败，不让"跳过"冒充"通过"）
- [x] `dispatch_test.go` SC-E03：单远程节点，走真实 `scp` 分发 + SSH 执行 → 断言远端路径有文件、内容一致
- [x] 目标地址取 `127.0.0.2`：somcli 把 `localhost`/`127.0.0.1` 当本机 `sh -c`，只有回环别名能逼它走 ssh
- [x] 用 `$SSH_CONNECTION` 作判据：自连接时文件系统是共享的，"远端有文件"证不了走了 SSH
- [x] 顺带修 D13：`sshKey` 的 `~` 只在 `RunCommandOnNode` 展开，`scp`/`ssh` 那几个入口拿的是原样字符串 → 照示例配置写的人分发一律失败
- [x] CI 内 SSH 自连接准备步骤（`ssh-keygen` + `authorized_keys` + 可达自检）
- [x] `test/matrix.yaml` 将 SC-E03 置 done（integration 流水线已绿）

## M0.5 multinode 组（SC-E04 / SC-E05 / SC-E06）

- [x] `test/fixtures/multinode/`：3 个 sshd 容器 + 自定义网络固定 IP（somcli 写死 22 端口，只能靠独立地址而非映射端口）
- [x] `test/multinode/` 建包，build tag `multinode`，TestMain 负责起停容器
- [x] `dispatch_test.go` SC-E04：多节点同一资源 → 每个节点都有产物（比对各自 `/etc/hostname`，防"三次都跑在同一台"蒙过）
- [x] `dispatch_test.go` SC-E05：`hosts` 定向 —— **目标节点有文件 且 未点名节点与操作机都没有**（D1 终极回归）
- [x] `dispatch_test.go` SC-E06：混合本机 / 远程编排，两个方向都断言
- [x] `test/matrix.yaml` 将 SC-E04/E05/E06 置 done（integration 流水线已绿）
- [x] 修掉用例自己的 ssh helper：它用 `CombinedOutput`，而 `UserKnownHostsFile=/dev/null`
      让 ssh 每次连接都往 stderr 写 `Warning: Permanently added ...`，读回的"产物内容"
      于是变成 `Warning: ...\r\nnode-a`。三条用例首轮 CI 全红，红的不是 somcli
      （日志显示三节点各自执行、`hosts` 定向只命中 node-b、混排两个方向都对），
      是用例自己。改为只取 stdout，stderr 折进 error；remote 组同一处一并改
      （那边不禁 known_hosts，首连之后不再提示，恰好是绿的 —— 等于把断言正确性交给运行顺序）

## M0.6 流水线

- [x] `.github/workflows/integration.yml`：`remote` 与 `multinode` 两个 job，失败时收集容器日志
- [x] 确认 `go test ./...` 默认仍为秒级（build tag 隔离生效）：实测 34s，全部耗时在 local 组的真实等待上
- [x] 修掉 `ci.yml` 里恒红的 import 守卫（原先裸匹配包名，连注释都判违规）
- [x] 两个 workflow 的 `push.branches` 加 `feat-*` / `fix-*`，`ci.yml` 补 `workflow_dispatch`
      （评审 MUST-1：原先只在 master/develop 上触发，feat 分支推上去不跑任何 job，
      而"归档在推送前"又要求先拿到绿信号 —— 构成死锁。用户定论走"改触发条件"）
- [x] 确认 `ci.yml` 三操作机矩阵（ubuntu / ubuntu-arm / macos）上 local 组全绿
- [x] 修掉 `configs/config.yaml` 缺文件尾换行（`new-line-at-end-of-file` 在 relaxed
      规则里是 error 而非 warning，静态检查因此红）；`line-length` 警告不动

## 测试

- [x] `go test ./...` 全绿（只含 local 组）
- [x] `go test ./test/remote/... -tags=remote` 全绿（CI run 32251103809）
- [x] `go test ./test/multinode/... -tags=multinode` 全绿（CI run 32251103809）
- [x] `grep -rn '"github.com/structure-projects/somcli' test/` 无结果
- [x] `grep -n "MIN_COVERAGE\|coverpkg" .github/workflows/ci.yml` 无结果

## 评审

- [x] 通过 expert-review（产出 `review.md`）—— 结论 ⚠️ 有条件通过，2 MUST / 5 SHOULD / 3 NIT
- [x] 修复所有 MUST fix 项
      - MUST-1 流水线在 feat 分支上无法触发 → 改触发条件（见 M0.6）
      - MUST-2 `loadCustomImageList` 纯文本分支把 tag 切在第一个冒号，
        `registry:5000/nginx:latest` 会静默产出 `name=registry` 去 pull。
        旧代码是"警告并跳过"，本次改动把它变成了"静默取错值" → 抽 `splitNameTag`
        改切最后一个冒号，最后一段含 `/` 判为无 tag
- [x] SHOULD fix 项已评估（5 项全修，无搁置项）
      - `installer.LoadDownloadConfig` 直接委托 `utils.LoadConfig`：此前 `download -f` 不应用
        全局设置，同一份文件 install 认、download 不认，正是 M0.7 要消灭的差异
      - `cluster.LoadConfig` 覆盖节点表补注释说明取舍（有意，非疏漏）
      - SC-X09 的 desc 收窄到用例真断言的三条命令，`images --custom-file` 那半条
        另立 SC-X10（需 docker，挂 M4）
      - `applyGlobalSettings` 注释改为与实现一致（挡住它的不只是命令行标志）
      - `cmd/validate.go` 的 `MarkFlagRequired` 补 `_ =`
- [x] NIT 已评估：`isHelpRequest` 对同名全局 flag 的取舍、`countDocuments` 数空尾文档
      两项不改（影响面极小且行为正确）；`cmd/docker.go` 的 `loadNodesFromFile`
      属老代码"写好没接线"，与 D1 同源，记入 M1 不在本里程碑动

## 归档（MUST 在推送前完成）

> 实际顺序与标题相反：本提案自己改了 workflow 触发条件（评审 MUST-1），绿信号只能从
> 推上远端的 feat 分支拿，于是真实次序是「推送 → 拿绿 → 置 done → 归档」。
> 这个死锁的记账见 proposal.md「评审阶段补记」。

- [x] `changes/changelog/0.2.0-alpha.md` 补条目，三项 BREAKING 单列（严格解析、退出码语义、`cluster:` 列表化）
- [x] `git mv changes/proposals/20260818-migrate-phase0-trusted-signal/ changes/archive/`
- [x] `changes/config.yaml` 的 `current-proposal` 切到 `20260818-migrate-phase1-engine-completion`

## 提交与推送

- [x] 通过 ci-gate（`gofmt -l` 空、`go vet` 干净、local 组全绿、CI 与 Integration 两条流水线全绿）
- [x] commit message 符合 Conventional Commits
- [x] 分支为 `feat-arch-convergence`
- [x] 推送经用户确认（用户定论"直接通过 gh 推送分支在远程分支跑 ci 流水线来验证"）