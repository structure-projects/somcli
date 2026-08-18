# 任务清单：M1 引擎补全

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选 + 更新 `test/matrix.yaml` 对应场景的
> `status` 与 `files`。用例一律黑盒（跑二进制），命名含场景 ID（如 `TestSC_E11_ExtraFilesWritten`）是约定而非机器校验。

## 准备

- [ ] M0 已归档，`current-proposal` 已切到本提案
- [ ] 阅读 `proposal.md` 与技术附录 §4.2 / §5.2 Phase 1 / §6.4
- [ ] 切 `feat-engine-completion` 分支

## M1.1 基础语义修正

- [ ] E5：统一 `ParseStr` 与 `ParseTargetPath` 的变量集，补 `Filename`/`Ext`（SC-D06）
- [ ] E3：`ResourceConfig` 新增 `vars` map，合入模板上下文；`--set k=v` 覆盖（SC-E12）
- [ ] F3：绝对路径 `target` 时 `LocalPath` 不再无条件 `filepath.Join(cacheDir, ...)`（SC-D05）
- [ ] F6：`CopyToRemote` 展开 `~`，与 `RunCommandOnNode` 行为一致（SC-F03）
- [ ] F5：下载器改 `net/http` 主实现，wget/curl 仅作降级（SC-D11）
- [ ] 下载器补齐：checksum 不符即删残留（SC-D02）、代理只对 github.com 生效（SC-D07）、离线命中（SC-D08）、离线缺失明确报错（SC-D09）、远程同 hash 跳过传输（SC-D10）
- [ ] F12 相关的离线开关一致性：`SOMCLI_OFFLINE` 与 `--offline`（SC-X03）
- [ ] 黑盒用例齐备并把上述场景置 done

## M1.2 method 分发 / extra_files / uninstall

- [ ] E1：`method` 分发骨架，一种 method 一个文件（`pkg/installer/method_*.go`）
- [ ] `method: script`（SC-M05）→ 现有行为收敛为显式语义
- [ ] `method: binary`（SC-M01）→ 解压 + 放置可执行文件 + chmod
- [ ] `method: package`（SC-M02）→ 调用发行版包管理器（探测抽象在 M3 完善，本里程碑先支持 yum/apt）
- [ ] `method: container`（SC-M03）→ 消费 `Resource.Image`
- [ ] `method: source`（SC-M04）→ 拉源码编译
- [ ] 补 `Resource.Package` 字段，`source_url` 统一到 `urls`
- [ ] D3：`extra_files` 渲染落盘（内容 / 权限 / 父目录创建），struct tag 改 `extra_files`（SC-E11）
- [ ] E2：新增 `cmd/uninstall.go`，`somcli uninstall -f <file>` 逆序执行 `remove_scripts`（SC-E10）
- [ ] `configs/tools.yaml` 改为三种 method 的可运行实战样例（kubectl / helm / jq）
- [ ] 黑盒用例：`test/local/{method,extrafiles,uninstall,vars}_test.go`

## M1.3 幂等 / 状态 / 并发 / 容错

- [ ] E4-a：状态文件 `somwork/state.json` 记录"哪个节点装了什么版本"+ `somcli status`（SC-E16）
- [ ] E4-b：幂等 guard —— `Resource.check` 非零则跳过整个资源（SC-E08）
- [ ] E4-c：`version` 变更后重新实施（SC-E09）
- [ ] E4-d：`on_error: abort|continue`（SC-E15）
- [ ] E4-e：节点级并发 `--parallel N`，结果与串行一致（SC-E14）
- [ ] 多节点部分失败按策略处理并汇总错误（SC-F05）
- [ ] 中途失败后重跑可继续，不因残留而失败（SC-F08）
- [ ] 黑盒用例：`test/local/idempotency_test.go`、`test/multinode/{parallel,onerror}_test.go`

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

- [ ] changelog 补条目，`method` 语义变更与 compose 缓存目录变更单列
- [ ] `git mv changes/proposals/20260818-migrate-phase1-engine-completion/ changes/archive/`
- [ ] `current-proposal` 切到 `20260818-migrate-phase2-cluster-as-config`

## 提交与推送

- [ ] 通过 ci-gate
- [ ] commit message 符合 Conventional Commits
- [ ] 分支为 `feat-engine-completion`
- [ ] 推送需用户确认