# 评审报告：20260818-migrate-phase3-platform-compat（M3 平台兼容）

| 字段 | 值 |
|---|---|
| 评审日期 | 2026-08-22 |
| 评审人 | AI（expert-review skill） |
| 评审范围 | `ebc79fc...HEAD`（M3.1–M3.5，6 个提交）相对 M2 归档点 |
| 结论 | ✅ 通过（无 MUST fix；S1 已修，S2 记后续，S3 为归档必做） |

## 符合性核对

| 提案目标 | 结论 | 证据 |
|---|---|---|
| F10 发行版抽象（yum/dnf/apt/zypper/apk） | ✅ | `pkg/utils/packagemanager.go`；`method_package.go` 瘦身到只取包名 + sudo；`integration.yml` distros job 覆盖 6 镜像 |
| F12 `--source` 有真实行为或移除 | ✅ | 评审时选择移除空壳；**评审后经用户要求改为重新实现**：删除从未生效的 `--source` 标志/`mirrors_source:`/`utils.InitSource`，新增可用的顶层 `source:` 配置块（official/aliyun/iso）+ `utils.RenderSourceSetup`，由 `method: package` 装包前自动前置。黑盒覆盖见 `test/local/source_test.go`。BREAKING，changelog 已列迁移写法 |
| arm64 架构参数化 | ✅ | `configs/**` 与 `configs/k8s/**` 改用 `{{.Arch}}`/`{{.UnameArch}}`；`kubernetes.go` 由 amd64 硬限改为 amd64/arm64 两档放行，arm64 k8s 集群安装标注实验性 |
| 构建矩阵单一来源 | ✅ | `build.sh` 为唯一来源（4 目标，cmd.Version ldflags，shebang 归位）；`Makefile build-all` 与 `go.yml` 委托它；`ci.yml` 矩阵同步为 4 目标；windows 移除。BREAKING，changelog 待补 |
| install.sh/README 架构识别 | ✅ | `install.sh` 用 case 映射 x86_64/amd64→amd64、aarch64/arm64→arm64；README 安装命令补 aarch64→arm64 |
| registry/images 黑盒覆盖 | ✅ | `test/local/registry_test.go`（SC-C03/C04，含 E6 回归）；`test/local/images_test.go` 扩 SC-C05 + SC-X10 |
| 总纲待决事项 1/2 定论 | ✅ | windows 移除；arm64 引擎完整、k8s 实验性，记录在 tasks.md |

## 维度评审

- **规范性**：命名/分层符合既有约定；包管理器探测正确下沉到 `pkg/utils`，业务代码再无裸 `yum`/`apt`（grep 已核）；`test/` 未 import 本仓库包（CI grep 守卫 + 人工核对）。
- **测试覆盖**：本地黑盒全绿（`go test ./...` 112s），四组 build tag（remote/multinode/cluster/swarm）`go vet` 通过；新增用例 `-race` 通过。矩阵本机可验部分 55/90 done，剩余 35 项全部为 CI/真集群/多节点承载，按既定方针保持 pending。
- **安全性**：包名经 `ShellQuote`（单引号包裹 + `'\''` 转义）后进入 shell，杜绝命令注入；离线镜像导入的 zip-slip 由 `secureJoin` 防护（R1 用例覆盖）；registry sync 经 `exec.Command` 传参不经 shell。未见 SQL/XSS/越权面（本仓库是 CLI 工具，无 Web 面）。
- **性能/并发**：`SyncAll` 的 goroutine + 信号量 + errChan 模式正确；单张失败不中断其余、错误聚合；`-race` 无数据竞争。
- **可读性**：关键决策（为何探测在远端 shell、sudo env 顺序、arm64 分级、半截归档删除）均有注释说明"为什么"。

## SHOULD fix（建议修复，不阻塞合并）

- [x] S1｜`build.sh` 硬编码 `VERSION="v1.0.0"` → 已改为 `git describe --tags --always --dirty`（与 Makefile 同源），发版产物版本号不再写死。
- [ ] S2｜SC-C03 harbor 安装的本地覆盖止于入参校验与 docker/docker-compose 前置依赖检查，真正的下载→解压→渲染 harbor.yml→跑 install.sh 这条链没有自动化覆盖。受限于"本地不联网、不起真 docker"的黑盒纪律可以理解，但应在 `integration.yml`/`e2e.yml` 里有一条带假 harbor 包或真 harbor 的用例兜底，否则这段重构后无人看守。
- [x] S3｜两个 BREAKING（用 `source:` 替换空壳 `--source`/`mirrors_source`、移除 windows 产物）的 changelog 条目已写入 `changes/changelog/0.5.0-alpha.md`，含迁移写法。

## 评审中发现并修复的问题

- **G9 追加修复**：`cmd/images.go` 让 pull/push/export/import 共用同一个包级变量接 `-o`/`-i`，
  pflag 在注册时即写入默认值，export/import 的默认 `images.tar.gz` 泄漏给 pull/push ——
  `images pull` 不带 `-o` 会在当前目录凭空写一个 YAML 清单（还被误命名成 .tar.gz），
  `images push` 不带 `-i` 不去用内置清单反而去读不存在的 `images.tar.gz`。已拆成
  `pullOutputFile`/`exportOutputFile`/`pushInputFile`/`importInputFile` 四个变量，
  并加 `TestG9_OutputInputDefaultsDontLeakBetweenSubcommands` 锁定（在临时 CWD 下断言）。
  这条是写 SC-X10 用例时被实测暴露的，不是为可测性改生产代码。

## NIT（可选）

- [ ] `go.yml` 用 `actions/setup-go@v4`，`ci.yml` 已用 v5，可顺手对齐。
- [ ] `pkg/registry/harbor.go` 的 `configureHarbor` 在证书生成失败时调 `log.Fatalf` 直接 `os.Exit`，而不是返回 error 让 cmd 层统一退出；与本文件其余错误返回风格不一致。预存问题，非本提案引入。
- [ ] `harbor.Uninstall` 调的是 `docker-compose`（v1 二进制），而仓库其余 compose 路径已转向 `docker compose`（v2 插件）；未来可统一。

## 评审意见

M3 的五个子里程碑边界清晰、提交独立，符合提案的渐进改造策略。发行版抽象没有滑向"包名归一化"的过度设计，arm64 采取分级支持而不是一刀切，`--source` 在"实现"与"移除"之间选择了诚实的移除——这些判断都与提案风险评估一致。两项 BREAKING 已在代码与 tasks.md 中明确，归档时补 changelog 即可发。

**建议**：合并前至少处理 S3（changelog，归档必做）；S1/S2 可记入后续提案但不应遗忘。无 MUST fix，准予进入 ci-gate。
