# 任务清单：M0 建立可信信号

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选，并更新 `test/matrix.yaml` 中对应场景的
> `status` 与 `files`。清单由人维护、人评审，没有校验它的元测试。
>
> 所有用例一律黑盒：编译二进制 → 真实配置 → 真实命令 → 断言退出码 / 输出 / 落盘产物 / 节点状态。
> `test/` 下禁止 import `github.com/structure-projects/somcli/...`。

## 准备

- [x] 立验收清单：`test/matrix.yaml` 82 场景（`env` 分组：local / remote / multinode / cluster / matrix）
- [x] 阅读 `proposal.md` 与技术附录 `doc/提案-架构收敛与测试体系.md` §4 / §5.2
- [ ] 切 `feat-arch-convergence` 分支（commit-msg hook 禁止在 master/develop 直接提交）
- [ ] 定论待决事项 6：是否新增 `somcli validate -f <file>` 只读校验命令

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

- [ ] `style: gofmt 格式化全部 Go 文件`（纯格式，独立提交）
- [ ] `chore: 安装 structure-agent-rules 规则与 changes 骨架`
- [ ] `docs(changes): 新增变更提案 20260818-migrate-arch-convergence 及 5 个阶段子提案`
- [ ] `fix(utils): RunScripts 与 GetNode 不再吞错与兜底`（D2 + D1）
- [ ] `fix(utils): 模板改用 text/template 避免 shell 字符被转义`（F1）
- [ ] `fix(installer): 下载失败不再打印成功且影响退出码`（F4 + G8）
- [ ] `fix(compose): 查看帮助不再触发下载与安装`（D9）
- [ ] `fix(config): 未知配置键改为报错而非静默忽略`（严格解析 + `configs/**` 示例修正）
- [ ] `test: 建立场景清单与黑盒用例`
- [ ] `ci: 用 ci.yml 取代恒绿的 test.yml`

## M0.3 local 组黑盒用例（默认 `go test ./...` 就跑）

公共 helper（`test/local/main_test.go`）：编译二进制一次、`run(args...) (code, out)`、`writeConfig`、
`tree(dir)` 目录快照。全部只用 `os/exec` 与 `os`，不 import `pkg/`。

- [ ] `engine_test.go` SC-E01：单资源，`post_install` 写标记文件 → 断言文件存在且内容正确
- [ ] `engine_test.go` SC-E02：三个资源各 `echo name >> order.txt` → 断言行序等于 `resources` 数组顺序
- [ ] `engine_test.go` SC-E07：`install -f cfg -n b` → 断言只有 b 的标记文件存在，a/c 未执行
- [ ] `template_test.go` SC-E13：URL / target / 脚本三处上下文渲染同一组变量；脚本回显渲染值到文件后断言（含 `&&`、`|` 不被转义 —— F1 回归）
- [ ] `template_test.go` SC-F06：配置写 `{{.NoSuchVar}}` → 退出码非 0，错误含变量名
- [ ] `node_resolve_test.go` SC-F01：声明 node-a，`hosts: [node-x]` → 退出码非 0、错误含 `node-x`、**本机与 workdir 无任何副作用**（D1 回归）
- [ ] `failure_test.go` SC-F04：`pre_install` 退出 3 → 非 0 退出、`post_install` 标记文件不存在、输出无 `[SUCCESS]`
- [ ] `failure_test.go` SC-F02：`nodes` 指向 `192.0.2.1`（TEST-NET-1 保证不可达）→ 错误信息含节点名 / 用户 / IP
- [ ] `download_test.go` SC-D01：`httptest` 起本地源 + 正确 checksum → 产物落盘、内容一致
- [ ] `download_test.go` SC-D03：源返回 500 → 退出码非 0、输出无 `[SUCCESS]`（F4/G8 回归）
- [ ] `download_test.go` SC-D04：相对路径 `target` → 产物落在 workdir 下的预期相对位置
- [ ] `workdir_test.go` SC-X01：两次不同 `--workdir` → 产物各自隔离；仓库目录无 `somwork` 新增（快照对比）
- [ ] `config_test.go` SC-X05：`configs/` 下每个示例经二进制校验（依赖待决事项 6 的结论）
- [ ] `config_test.go` SC-F07：未知字段 / 拼错键 / 重复键三类 → 均退出码非 0，错误指出键名
- [ ] `help_test.go` D9 回归：对全部叶子命令跑 `--help`，前后目录树快照一致、退出码 0
- [ ] `doc_commands_test.go` SC-X04：抽 `README.md` + `doc/*.md` 的 ```bash 块中 `somcli ...` 调用 → 对二进制执行 `<cmd> --help` 断言退出码 0；标志断言出现在 `--help` 输出中
- [ ] 修文档中不存在的命令与标志（G4/G5，仅改名不改结构）：`docker-images`→`images`、`offline download`→`download`、`cluster deploy`→`cluster create`、`registry install -h`→`-H`；删除未注册的 `docker uninstall --force` / `apply -f` / `images --username|--password` / compose `-v|-p`
- [ ] 每条用例在缺陷未修版本（`git stash` 或 checkout 修复前提交）上确认会失败

## M0.4 remote 组（SC-E03）

- [ ] `test/remote/` 建包，build tag `remote`，未满足前置时 `t.Skip` 并说明原因
- [ ] `dispatch_test.go` SC-E03：单远程节点，走真实 `scp` 分发 + SSH 执行 → 断言远端路径有文件、内容一致
- [ ] CI 内 SSH 自连接准备步骤（`ssh-keygen` + `authorized_keys` + `ssh-keyscan`）
- [ ] `test/matrix.yaml` 将 SC-E03 置 done

## M0.5 multinode 组（SC-E04 / SC-E05 / SC-E06）

- [ ] `test/fixtures/multinode/compose.yaml`：3 个 systemd-enabled sshd 容器 + 自定义网络
- [ ] `test/multinode/` 建包，build tag `multinode`
- [ ] `dispatch_test.go` SC-E04：多节点同一资源 → 每个节点都有产物
- [ ] `dispatch_test.go` SC-E05：`hosts` 定向 —— **目标节点有文件 且 运行 somcli 的容器无文件**（D1 终极回归）
- [ ] `dispatch_test.go` SC-E06：混合本机 / 远程编排
- [ ] `test/matrix.yaml` 将 SC-E04/E05/E06 置 done

## M0.6 流水线

- [ ] `.github/workflows/integration.yml`：`remote` 与 `multinode` 两个 job，失败时收集容器日志
- [ ] 确认 `go test ./...` 默认仍为秒级（build tag 隔离生效）
- [ ] 确认 `ci.yml` 三操作机矩阵（ubuntu / ubuntu-arm / macos）上 local 组全绿

## 测试

- [ ] `go test ./...` 全绿（只含 local 组）
- [ ] `go test ./test/remote/... -tags=remote` 全绿（CI）
- [ ] `go test ./test/multinode/... -tags=multinode` 全绿（CI）
- [ ] `grep -rn "structure-projects/somcli" test/` 无结果
- [ ] `grep -n "MIN_COVERAGE\|coverpkg" .github/workflows/ci.yml` 无结果

## 评审

- [ ] 通过 expert-review（产出 `review.md`）
- [ ] 修复所有 MUST fix 项
- [ ] SHOULD fix 项已评估（不修复需说明理由）

## 归档（MUST 在推送前完成）

- [ ] `changes/changelog/<version>.md` 补条目，两项 BREAKING（严格解析、退出码语义）单列
- [ ] `git mv changes/proposals/20260818-migrate-phase0-trusted-signal/ changes/archive/`
- [ ] `changes/config.yaml` 的 `current-proposal` 切到 `20260818-migrate-phase1-engine-completion`

## 提交与推送

- [ ] 通过 ci-gate（归档 + commit-msg + 编译 + local 组用例）
- [ ] commit message 符合 Conventional Commits
- [ ] 分支为 `feat-arch-convergence`
- [ ] 推送需用户确认