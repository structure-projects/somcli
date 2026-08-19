# 任务清单：M1 引擎补全

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选 + 更新 `test/matrix.yaml` 对应场景的
> `status` 与 `files`。用例一律黑盒（跑二进制），命名含场景 ID（如 `TestSC_E11_ExtraFilesWritten`）是约定而非机器校验。

## 准备

- [x] M0 已归档，`current-proposal` 已切到本提案
- [x] 阅读 `proposal.md` 与技术附录 §4.2 / §5.2 Phase 1 / §6.4
- [x] ~~切 `feat-engine-completion` 分支~~ → 沿用 `feat-arch-convergence`，理由见 proposal「执行期偏差记账」

## M1.1 基础语义修正

- [x] E5：统一 `ParseStr` 与 `ParseTargetPath` 的变量集，补 `Filename`/`Ext`（SC-D06）
      两处各写一份结构体改为共用 `TemplateContext`，从此不可能再漂移
- [x] E3：`ResourceConfig` 新增 `vars` map，合入模板上下文；`--set k=v` 覆盖（SC-E12）
      走 `{{.Vars.xxx}}` 命名空间，用户变量不可能遮掉内置变量；模板加 `missingkey=error`
      守住 SC-F06（map 上下文默认渲染 `<no value>` 而不报错）
- [x] F3：绝对路径 `target` 时 `LocalPath` 不再无条件 `filepath.Join(cacheDir, ...)`（SC-D05）
      顺带修掉"只建 cacheDir 不建 target 父目录"，`target: bin/nested/tool.sh` 才能落盘
- [x] F6：`CopyToRemote` 展开 `~`，与 `RunCommandOnNode` 行为一致（SC-F03）
      M0 已作为 D13 修掉，本里程碑只欠 remote 组用例
- [x] F5：下载器改 `net/http` 主实现，~~wget/curl 仅作降级~~ → 只有 `net/http` 一条路径（SC-D11）
      降级路径砍掉的理由见「执行期偏差记账」：`http.DefaultTransport` 本就认
      `HTTP_PROXY`/`NO_PROXY`，留着等于留一条永不触发、无人覆盖的分支。
      改完 SC-D01/D03/D04 在 macOS 与开发机上不再需要 `t.Skip`
- [x] 下载器补齐：checksum 不符即删残留（SC-D02）、代理只对 github.com 生效（SC-D07）、离线命中（SC-D08）、离线缺失明确报错（SC-D09）、远程同 hash 跳过传输（SC-D10）
      SC-D10 属 remote 组，见下方遗留项；顺带发现并修掉 D14（`download` 与 `install` 代理配置键不一致）
- [x] F12 相关的离线开关一致性：`SOMCLI_OFFLINE` 与 `--offline`（SC-X03）
- [ ] 遗留：remote 组的 SC-D10 与 SC-F03 用例（`test/remote/`）
- [x] 黑盒用例齐备并把上述场景置 done（local 组；remote 两条待补）
      每条新用例都在 M1 前的提交上跑过并确认变红，且红的原因各自归属其缺陷

## M1.2 method 分发 / extra_files / uninstall

- [x] E1：`method` 分发骨架，一种 method 一个文件（`pkg/installer/method_*.go`）
      方法本身不 exec，只把动作编译成命令列表交给 `utils.RunScripts` —— 远程执行、
      日志、失败传播只有一处实现，M1.3 的 `--parallel` / `on_error` 自动共享。
      认不出的 method 必须报错并列出可用值（静默当脚本跑正是 E1 的病症）
- [x] `method: script`（SC-M05）→ 现有行为收敛为显式语义
      与不写 `method` 行为一致，作为兼容性守卫；此条按定义无法在旧版本上证伪
- [x] `method: binary`（SC-M01）→ 解压 + 放置可执行文件 + chmod
      新增 `install_dir`（缺省 `/usr/local/bin`）与 `files`（归档内筛选，留空装所有可执行文件）
- [x] `method: package`（SC-M02）→ 调用发行版包管理器
      优先级表 apt-get / dnf / yum / zypper / apk / brew，探测写在生成的 shell 里而非
      Go 的 `LookPath` —— 判断必须发生在目标节点上。**local 半边完成，matrix 半边未兑现，
      故 SC-M02 仍为 pending**，理由见「执行期偏差记账」
- [x] `method: container`（SC-M03）→ 消费 `Resource.Image`
      不止拉镜像：在 `install_dir` 生成同名包装脚本，脚本自己探测 docker / podman
- [x] `method: source`（SC-M04）→ 拉源码编译
      新增 `build:`，缺了直接报错（不猜构建方式：猜错的代价是"报告装好了、其实什么都没编出来"）
- [x] 补 `Resource.Package` 字段，~~`source_url` 统一到 `urls`~~ → `source` 直接读 `urls`，无 `source_url` 字段可迁
- [x] D3：`extra_files` 渲染落盘（内容 / 权限 / 父目录创建），struct tag 改 `extra_files`（SC-E11）
      权限定死 0644；先在缓存目录落暂存件再 `install` 就位，远程走与下载产物完全相同的分发路径
- [x] E2：`somcli uninstall -f <file>` 逆序执行 `remove_scripts`（SC-E10）
      入口加在 `cmd/install.go` 而非新建 `cmd/uninstall.go`：与 install 共用 `-f`/`-n` 标志与错误措辞
- [x] `configs/tools.yaml` 改为三种 method 的可运行实战样例（kubectl / helm / jq）
      `method: manifest` 明确报"尚未实现"，与未知 method 分开报（它是路线图上的合法取值）
- [x] `pkg/cluster/kubernetes.go` 六处 `Method` 最小适配为 `script`
      那六个资源的动作都在自己的 `post_install` 里，分发一旦生效会二次安装 / 装不存在的包
- [x] 黑盒用例：`test/local/{method,extrafiles,uninstall}_test.go`（`vars_test.go` 属 M1.1）
      27 条用例，在 M1.1 提交 `43c6769` 上 26 红 1 绿；红的归因分三类，见「执行期偏差记账」
- [ ] 遗留：SC-M02 的 matrix 半边（真实发行版上真装一次包）

## M1.3 幂等 / 状态 / 并发 / 容错

- [x] E4-a：状态文件 `somwork/state.json` 记录"哪个节点装了什么版本"+ `somcli status`（SC-E16）
      逐节点记账（不声明 `hosts` 时记空串、显示 `(local)`）；损坏即降级为无状态执行并出声，
      本轮成功记录把文件重建；写入走临时文件 + rename，半截 JSON 会被读成"损坏"而丢掉整份账本
- [x] E4-b：幂等 guard —— `Resource.check` **退出 0** 则跳过整个资源（SC-E08）
      提案原文"非零即跳过"写反了，见「执行期偏差记账」；探针失败不是错误，就是"还没装"
- [x] E4-c：`version` 变更后重新实施（SC-E09）
      判据是 `name@version` 而非 `name`，否则升级永远装不上
- [x] 提案外新增 `--force`：越过状态记录**与** `check` 探针
      状态一旦参与决策就必须留出口，否则用户只能去删 `state.json`
- [x] E4-d：`on_error: abort|continue`（SC-E15）
      作用域限于脚本阶段，文件分发失败一律致命；`continue` 之后**仍以非 0 退出**；
      失败的资源/节点不记入状态（记了下次会被当成已安装跳过）
- [x] E4-e：节点级并发 `--parallel N`，结果与串行一致（SC-E14）
      每条脚本一道栅栏（各节点跑完第 i 条才开始第 i+1 条），结果按声明顺序打印；
      `limit <= 1` 走原串行路径，单机默认行为不受影响
- [x] 多节点部分失败按策略处理并汇总错误（SC-F05）
      逐目标隔离：失败的目标从后续阶段剔除，存活目标走完整个资源
- [x] 中途失败后重跑可继续，不因残留而失败（SC-F08）
- [x] 四个 method 安装器签名改为 `([]string, error)`，收口到一处执行
      为把逐目标失败隔离穿到 method 阶段；顺带让代码与 M1.2 已写下的设计一致
- [x] 黑盒用例：`test/local/{idempotency,state,onerror}_test.go`、`test/multinode/{parallel,onerror}_test.go`
      local 17 条已在 `91110e6` 上证伪（16 红 1 绿，归因见提案）；
      multinode 5 条本机无 docker，只做了推断，待 CI 复核

## M1.4 子系统修复

- [ ] D8/F13：重写 compose 安装器 —— 真正落盘 + chmod + 校验；修 `docker-comopose` 拼写；使用入参版本；URL 用 `{{.Arch}}`；去掉非法 `{{}}` 模板（SC-C02）
- [ ] D7：实现 `loadNodesFromFile`，或让 docker 子系统复用统一的 `nodes:` 解析并删除这条独立路径（SC-C01）
- [ ] F14：images `pull`/`push`/`import` 聚合每张镜像的错误并影响退出码；失败时不写出镜像清单
- [ ] E6：`registry uninstall` 补齐自身标志（`-H` 等），不再依赖 install 的包级变量
- [ ] E7：`delete` 注册 `-n/--namespace`
- [ ] 黑盒用例：`test/local/{docker,compose}_test.go`

## 测试

- [ ] `go test ./...` 全绿
- [ ] `go test ./test/local/... -tags=remote`、`./test/multinode/... -tags=multinode` 全绿
- [ ] 每个 D/E/F 编号缺陷在修复前能复现失败

## 评审

- [ ] 通过 expert-review（产出 `review.md`）
- [ ] 修复所有 MUST fix 项
- [ ] SHOULD fix 项已评估

## 归档

- [ ] changelog 补条目，`method` 语义变更、顶层 `proxy:` 键删除、compose 缓存目录变更各自单列
- [ ] `git mv changes/proposals/20260818-migrate-phase1-engine-completion/ changes/archive/`
- [ ] `current-proposal` 切到 `20260818-migrate-phase2-cluster-as-config`

## 提交与推送

- [ ] 通过 ci-gate
- [ ] commit message 符合 Conventional Commits
- [x] 分支为 ~~`feat-engine-completion`~~ `feat-arch-convergence`（见「执行期偏差记账」）
- [ ] 推送需用户确认