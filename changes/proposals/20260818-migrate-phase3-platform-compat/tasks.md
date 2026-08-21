# 任务清单：M3 平台兼容

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选 + 更新 `test/matrix.yaml`。
> 本里程碑收尾时 `test/matrix.yaml` 应达到 82/82 done。

## 准备

- [x] M2 已归档，`current-proposal` 已切到本提案
- [x] 阅读 `proposal.md` 与技术附录 §4.3 / §4.5 / §5.2 Phase 3 / §6.7
- [x] 切 `feat-platform-compat` 分支
- [x] 定论总纲待决事项 1（windows 产物）与 2（arm64 支持级别）
  - 按总纲倾向：windows 产物在 M3.4 移除；arm64 引擎/工具编排完整支持，k8s 集群安装标注实验性

## M3.1 发行版抽象（F10）

- [x] `pkg/utils` 新增包管理器探测：`yum` / `dnf` / `apt` / `zypper` / `apk`
- [x] 一个包管理器一组命令模板（install / remove / update / query），禁止业务代码里再出现裸 `yum`
- [x] `method: package` 消费该抽象
- [x] `test/fixtures/base-deps.yaml`：跨发行版的最小基础依赖清单
- [x] `integration.yml` 增 `distro-matrix` job：centos7 / rockylinux9 / ubuntu22.04 / debian12 / opensuse-leap15（SC-P01..P06）
- [x] CentOS 7 源不可用时的处置（vault 源或标注允许失败）

## M3.2 架构参数化

- [x] 下载 URL 模板统一用 `{{.Arch}}`，配置侧同步修正
- [x] 解除 `pkg/cluster/kubernetes.go` 的非 amd64 拒绝
- [ ] arm64 目标节点跑通工具编排（SC-P07，由 ci.yml 的 ubuntu-24.04-arm 承载，转绿前保持 pending）
- [x] k8s 集群安装的 arm64 支持级别按定论标注（完整/实验性）

## M3.3 换源与离线（F12）

- [x] `--source` 由 `BoolVar` 改为 `StringSliceVar`（或按定论移除该标志）
  - 定论：直接移除。该能力从未真正实现（`InitSource` 的 .sh/.iso 是空函数体），留着是 BREAKING 假象；changelog 单列。
- [x] 实现 `utils.InitSource` 的 `.sh` / `.iso` 两个分支（或移除并同步文档，不留空函数体）
- [x] `SOMCLI_OFFLINE` 与 `--offline` 语义一致性复核（SC-X03 三子项 + 不开离线均绿）
- [x] 操作机矩阵：Linux amd64 / Linux arm64 / macOS arm64（SC-P08/P09，由 ci.yml test matrix 承载，转绿前保持 pending）

## M3.4 构建矩阵归一

- [x] `build.sh` / `Makefile build-all` / `go.yml` / `ci.yml` 收敛为单一来源
  - `build.sh` 为唯一来源（4 目标，cmd.Version ldflags）；`Makefile build-all` 与 `go.yml` 均委托它；`ci.yml` 交叉编译矩阵同步为 4 目标
- [x] 按定论处理 windows 产物（保留则四处一致；移除则同步 README 与 changelog）
  - 已移除：Makefile 不再产出 .exe，ci.yml 矩阵删除 windows/amd64，go.yml 本就不上传；README 标注支持范围；changelog 于归档时单列 BREAKING
- [x] 修 `install.sh` 的 arm64 / `aarch64` 识别；修 `install.sh`/`build.sh` shebang 位置
- [x] `README` 安装脚本段与实际产物名一致（补 aarch64→arm64 映射）

## M3.5 周边子系统覆盖

- [x] `test/local/registry_test.go`：harbor 安装（SC-C03）、镜像同步（SC-C04）
  - harbor install：hostname/version 校验、docker/docker-compose 前置依赖检查
  - sync：入参校验、pull→tag→push→rmi 编排、E6（uninstall 不再蹭 install 校验）回归
- [x] `sync` 的两个既有问题复核：
  - 单张失败不中断：SyncAll 各镜像独立 goroutine，错误聚合后整体非 0 退出（已加测试锁定）
  - cleanup：`docker rmi` 只删**本地**标签，目标 registry 中已推送镜像不受影响；且仅在 push 成功后执行（已加测试锁定）
- [x] `test/local/images_test.go`：pull / save / load / list 全生命周期（SC-C05）
  - export(save) 产合法 gzip、失败删半截归档；import(load) 逐条 docker load；坏归档拒绝

## 测试

- [ ] `go test ./...` 全绿
- [ ] `test/matrix.yaml` 达到 **82/82 done**

## 评审

- [ ] 通过 expert-review（产出 `review.md`）
- [ ] 修复所有 MUST fix 项
- [ ] SHOULD fix 项已评估

## 归档

- [ ] changelog 补条目，`--source` 类型变更与 windows 产物决定单列
- [ ] `git mv changes/proposals/20260818-migrate-phase3-platform-compat/ changes/archive/`
- [ ] `current-proposal` 切到 `20260818-migrate-phase4-config-and-docs`

## 提交与推送

- [ ] 通过 ci-gate
- [ ] commit message 符合 Conventional Commits
- [ ] 分支为 `feat-platform-compat`
- [ ] 推送需用户确认