# 迁移变更提案：M3 平台兼容

| 字段 | 值 |
|---|---|
| 提案 ID | 20260818-migrate-phase3-platform-compat |
| 级别 | major |
| 类型 | migration |
| 创建日期 | 2026-08-18 |
| 创建人 | chuck |
| 状态 | draft |
| 优先级 | medium |
| 总纲 | `changes/proposals/20260818-migrate-arch-convergence/proposal.md` |
| 技术附录 | `doc/提案-架构收敛与测试体系.md` §4.3（F10/F12）、§4.5（工程基线）、§5.2 Phase 3、§6.7 |
| 前置 | M2 完成（`method: package` 的发行版分发需要引擎与集群路径都已归位） |
| 场景 | 12 个（`test/matrix.yaml` 中 `phase: 3`），当前全部 pending |


> 验证方式遵循总纲「功能验证约定」：一律黑盒 —— 编译二进制 → 喂真实配置 → 跑真实命令 →
> 断言退出码 / 输出 / 落盘产物 / 节点状态；`test/` 禁止 import `pkg/`；不设覆盖率门槛。

## 现状

| 编号 | 问题 |
|---|---|
| F10 | 仅支持 `yum`，硬编码 `yum install`，Debian/Ubuntu 直接失败 |
| F12 | `--source` 语义是"镜像源列表"却声明为 `BoolVar`，`viper.GetStringSlice` 得到 `["false"]`；`utils.InitSource` 的 `.sh`/`.iso` 两个分支都是空函数体 → 换源能力完全不存在 |
| — | arm64 全线不通：`kubernetes.go` 显式拒绝非 amd64、下载 URL 硬编码 amd64、compose 安装器硬编码 `x86_64`（后者已在 M1 修） |
| — | `build.sh` / `Makefile build-all` / `go.yml` / `ci.yml` 四处构建矩阵互相矛盾（Makefile 缺 darwin-arm64、多 windows；go.yml 不上传 windows） |
| — | `install.sh` 的 arm64 识别与 `README` 的 `aarch64` 映射不一致；`install.sh`/`build.sh` shebang 位置有问题 |
| — | registry / images 子系统尚未有集成覆盖（SC-C03/C04/C05） |

## 目标状态

- 发行版抽象：`yum`/`dnf`/`apt`/`zypper`/`apk` 探测与分发，`method: package` 在 5 个发行版镜像上均成功；
- 架构参数化：下载 URL 用 `{{.Arch}}`，解除 `kubernetes.go` 的 amd64 硬限；
- 换源能力：`--source` 改 `StringSliceVar` 并真正实现，或明确移除该能力并同步文档（二者择一，不留空实现）；
- 构建矩阵单一来源，四处不再互相矛盾；
- registry / images 子系统有黑盒用例覆盖。

## 迁移策略

渐进改造。发行版抽象作为一个独立的探测层引入（`pkg/utils` 内），`method: package` 消费它；
先在容器矩阵里把"基础依赖安装"这一条最小路径跑通，再逐步扩大到 k8s 组件。

`arm64` 采取"分级支持"策略：引擎与工具编排完整支持；k8s 集群安装因上游 cri-dockerd 与部分 CNI 资产不齐，标注为实验性并在兼容矩阵中如实说明（见总纲待决事项 2）。

## 阶段规划

| 里程碑 | 范围 | 完成标准 |
|---|---|---|
| M3.1 | 发行版探测抽象 + `method: package` 分发 | 5 个容器镜像上 `test/fixtures/base-deps.yaml` 均成功（SC-P01..P06） |
| M3.2 | 架构参数化 + arm64 分级支持 | SC-P07 done；`kubernetes.go` 无 amd64 硬限 |
| M3.3 | 换源与离线（F12）+ 操作机矩阵 | SC-P08/P09 done；`--source` 有真实行为或已移除 |
| M3.4 | 构建矩阵归一 + 安装脚本修正 | 单一来源；`install.sh` arm64 识别正确 |
| M3.5 | registry / images 黑盒覆盖 | SC-C03/C04/C05 done |

## 风险评估

- **发行版抽象容易滑向"支持一切"的过度设计** → 只做包管理器探测 + 命令模板映射，不做包名归一化；包名差异由配置侧用 `{{.PkgManager}}` 区分或按发行版给不同资源。
- **CentOS 7 镜像已 EOL，`yum` 源不可用导致 CI 红** → 容器矩阵内先配置 vault 源或改用 `centos:7` 的存档镜像；若无法稳定，降级为"允许失败"的 job 并在兼容矩阵中标注。
- **`--source` 修正是 BREAKING**（`bool` → `StringSlice`）→ 原类型即错误，无人能真正使用；changelog 单列。
- **移除 windows 产物会影响既有下载者** → 见总纲待决事项 1，需先定论；若移除，在 changelog 与 README 显著标注。
- **arm64 标注实验性可能被理解为不支持** → 在 `doc/12-平台兼容矩阵.md`（M4）用支持级别（完整/实验/不支持）逐格说明。

## 回滚预案

五个子里程碑独立提交。发行版抽象是新增层，回滚后 `method: package` 退回 M1 的 yum/apt 两路支持，不影响其他能力。

## 兼容性保证

| 维度 | 说明 |
|---|---|
| CLI | **BREAKING**：`--source` 由 `bool` 改为 `StringSlice`（原类型错误）；若移除该能力则同时删除标志并在文档说明替代做法 |
| 配置 | 下载 URL 中的架构写法由硬编码改为 `{{.Arch}}`，旧配置若写死 `amd64` 仍可用 |
| 产物 | windows 产物去留见待决事项；构建矩阵归一后 `Makefile`/`build.sh`/CI 产物名统一 |
| 平台 | 新增 dnf/zypper/apk 支持；arm64 分级支持 |

## 双规范并存期约定

- 老代码：`install.sh` / `build.sh` / `quick_start.sh` 属脚本层，本里程碑做必要修正但不重写。
- 新代码：发行版探测抽象放 `pkg/utils`，一个包管理器一组命令模板，禁止在业务代码里再出现裸 `yum`/`apt`。
- 边界识别：文件是否在「影响范围」中。

## 影响范围

- **代码**：`pkg/utils/{os,download}.go` + 新增发行版探测、`pkg/installer` 的 `method: package` 分发、`pkg/cluster/kubernetes.go`（解除架构硬限）、`cmd/root.go`（`--source` 类型）
- **配置**：`configs/**` 的 URL 架构参数化；`test/fixtures/base-deps.yaml`
- **测试**：`test/local/{registry,images}_test.go`、`test/local/utils_offline_test.go`、发行版容器矩阵
- **CI**：`integration.yml` 的 `distro-matrix` job；`ci.yml` 的操作机矩阵已具备
- **脚本**：`install.sh`、`build.sh`、`Makefile`、`.github/workflows/go.yml`

## 验收标准

- [ ] `test/matrix.yaml` 中 12 个 `phase: 3` 场景全部 done，**累计 82/82 done**
- [ ] 5 个发行版镜像上 `method: package` 均成功（SC-P01..P06）
- [ ] arm64 目标节点可完成工具编排（SC-P07）；k8s 集群安装的 arm64 支持级别已明确标注
- [ ] macOS 操作机（无 wget）可正常下载（SC-P09）
- [ ] `--source` 有真实行为或已明确移除，不留空实现
- [ ] 构建矩阵单一来源，四处不再矛盾
- [ ] 总纲待决事项 1/2 已定论并记录
- [ ] changelog 补条目，`--source` 类型变更与 windows 产物决定单列

## 任务清单

详见 `tasks.md`。