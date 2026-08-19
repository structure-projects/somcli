# 评审报告：20260818-migrate-phase0-trusted-signal

| 字段 | 值 |
|---|---|
| 评审日期 | 2026-08-19 |
| 评审人 | AI（**非人类专家评审**，关键项目 MUST 另行引入人类评审） |
| 评审范围 | `git diff master...HEAD`，31 个提交，198 文件，+25242 / -457 |
| 结论 | ⚠️ 有条件通过 |

> **前置条件未完全满足**：`tasks.md` 仍有 12 项未勾选，其中 7 个场景（SC-E03/E04/E05/E06、
> SC-D01/D03/D04）的载体只存在于 CI，本机缺 wget 与 docker，无法在评审现场验证。
> 本报告对这部分只评审代码与流水线定义，不声称已验证其行为。

## 评审维度结论

| 维度 | 结论 | 依据 |
|---|---|---|
| 符合性 | ⚠️ 部分 | 提案「验收标准」第 1 条（22 个 phase 0 场景全 done）未达成：15 done / 7 pending，且 pending 项的唯一载体当前无法触发（MUST-1） |
| 规范性 | ✅ 通过 | 提交按语义切分且符合 Conventional Commits；`test/` 黑盒约束有 CI 守卫兜底；注释一律解释 why 而非 what |
| 功能验证 | ✅ 通过（按本项目约定） | 本项目不设行覆盖门槛，进度以 `test/matrix.yaml` 场景数衡量。local 组 `go test ./...` 全绿（33.8s）；每条用例已在 `48530cc` 上确认转红 |
| 安全性 | ⚠️ 有既有弱点 | 本次未引入新风险；`StrictHostKeyChecking=no` 与 `sh -c` 拼接为老代码既有形态，见下 |
| 性能 | ✅ 无 P0 | 下载仍串行且 shell out `wget`（F5），已记 M1 |
| 可读性 | ✅ 通过 | 关键决策点都有注释说明"曾经错在哪"，新人可读 |

## MUST fix（必须修复）

- [ ] **流水线在当前分支上无法触发，7 个 pending 场景拿不到绿信号 —— 且与 `tasks.md` 的归档顺序构成死锁**
      `.github/workflows/ci.yml:4-8` 与 `integration.yml:16-21` 都只在 push / PR 到 `master`、`develop`
      时运行。当前分支 `feat-arch-convergence` 直接 push **不会触发任何 workflow**，而 `ci.yml`
      连 `workflow_dispatch` 都没有（`integration.yml` 有）。
      于是 `tasks.md`「归档 MUST 在推送前完成」+「确认矩阵全绿」两条无法同时满足：
      拿绿信号必须先开 PR，开 PR 必须先推送，推送前必须先归档，归档前必须先绿。
      建议二选一：① `ci.yml` 补 `workflow_dispatch`，两个 workflow 的 `push.branches` 加
      `feat-*`；② 在 `tasks.md` 中明确「归档在 PR 绿之后、合并之前」，并把该顺序写回提案。
      这是唯一阻塞提案自身验收标准的项。

- [ ] **`pkg/images/utils.go` 纯文本分支引入静默回归：带端口的镜像名被切错**
      新代码用 `strings.Cut(line, ":")` 在**第一个**冒号切分（`pkg/images/utils.go` 纯文本兜底段），
      `registry:5000/nginx:latest` 会得到 `name="registry"` / `tag="5000/nginx:latest"`。
      旧代码是 `strings.Split(line, ":")` + `len(parts) != 2` 判断，这一行会被警告并跳过。
      即：改动前是"拒绝并告知"，改动后是"静默拿错误镜像名去 pull"。这与本提案
      「未知/非法输入拒绝而非静默忽略」的主张正好相反。
      建议改用最后一个冒号，并在最后一段含 `/` 时判定为"无 tag"。

## SHOULD fix（建议修复）

- [ ] **两条配置加载路径行为不一致，与 M0.7 的主张相悖**
      `utils.LoadConfig` 会调 `applyGlobalSettings`，让配置里的 `offline` / `debug` / `workdir` /
      `github_proxy` / `mirrors_source` 真正生效；而 `installer.LoadDownloadConfig`
      （`pkg/installer/downloader.go:29`，`somcli download -f` 的唯一入口，见 `cmd/install.go:103`）
      只做 `UnmarshalStrict`，全局设置一律不生效。
      同一份文件 `install` 认、`download` 不认 —— 正是 M0.7「一份配置所有场景都能加载」
      要消灭的那类差异。建议 `LoadDownloadConfig` 直接委托 `utils.LoadConfig`。

- [ ] **`cluster.LoadConfig` 覆盖而非合并节点表**
      `pkg/cluster/common.go:48` 的 `utils.SetNode(spec.Nodes)` 执行时，`utils.LoadConfig` 已经把
      顶层 `nodes:` 写进了全局 `Config.Nodes`，这一行把它整体替换掉。
      一份同时写了顶层 `nodes:` 与 `cluster[].nodes` 的配置（`configs/config.yaml` 就是这形态），
      走 cluster 路径时顶层节点不可见。当前 cluster 流程不消费 `resources` 所以没暴露，
      但 M2「cluster as config」把两者放一起跑时会踩。至少补注释说明这是有意取舍。

- [ ] **SC-X09 的 desc 承诺大于用例实际覆盖**
      `test/matrix.yaml:718` 写「install / cluster / **images** 场景下都能加载」，
      而 `test/local/config_test.go:316` 的三个子用例只覆盖 validate / install / cluster，
      `images --custom-file` 读统一配置这条没有任何断言（它需要 docker）。
      矩阵是人工维护的唯一事实来源，一条 `done` 里夹着未验证的承诺会污染整份清单的可信度。
      建议把 desc 收窄到实际覆盖范围，`images` 那半条另立场景挂到需要 docker 的组。

- [ ] **`applyGlobalSettings` 注释与实现不符**
      `pkg/utils/utils.go` 注释称"命令行标志已显式设置的不覆盖"，实际判据是
      `viper.GetString(...) == ""` —— `~/.somcli.yaml` 里的同名值也会挡住资源配置。
      行为本身合理（flag > 全局配置 > 资源配置），但注释只说了一半。

- [ ] **`cmd/validate.go:50` `MarkFlagRequired` 返回值未处理**，同仓库 `cmd/cluster.go:96` 用的是 `_ =`。

## NIT（可选）

- [ ] `isHelpRequest`（`cmd/compose.go`）会把"只给了一个与 somcli 全局同名的 flag"判成要帮助：
      `somcli docker-compose --debug` 打的是 somcli 的帮助，compose 自己的 `--debug` 因此不可达。
      受影响的只有 6 个全局 flag 名，建议在注释里记一句已知取舍。
- [ ] `cmd/docker.go:228` `loadNodesFromFile` 丢弃 `utils.LoadConfig` 的返回值后直接返回"未实现"错误
      —— 一次纯副作用调用（会改写全局 `Config`）。属"写好了没接线"，与 D1 同源，本提案未触碰，
      按老代码容忍原则不强求本次修，建议记入 M1 缺陷清单。
- [ ] `countDocuments`（`cmd/validate.go:130`）把尾部空文档也计为一个文档，
      配置末尾多写一个 `---` 会得到"含 2 个 YAML 文档"的报错。信息正确但措辞会让人困惑。

## 安全性说明（均为既有形态，本次未引入）

- `RunCommandWithOutput("sh", "-c", runScript)`：配置内容即脚本，命令注入是本工具的设计前提，
  信任边界在"用户自己的配置文件"，不作为缺陷。
- `StrictHostKeyChecking=no`（`pkg/utils/ssh.go` 多处）：首次连接不校验主机指纹，存在 MITM 风险。
  老代码既有，本提案未引入也未修复，建议记入后续阶段。
- D13 的 `ExpandPath` 修复顺带消除了一类"读不到私钥就静默失败"的路径，方向正确。

## 评审意见

主体质量高于本仓库既有水位，且做对了两件难做对的事：

1. **先修错误传播再补验证**。D2/D1 修好之前写的任何用例都无法区分"真通过"与"吞错后的假通过"，
   提案把这个顺序当作硬前置，是本次改动可信的根本原因。
2. **删掉了上一轮的白盒产物并接受进度数字变小**（16 done → 3 done → 15 done）。
   用"矩阵场景数"替代"行覆盖率"作为进度口径，且每条用例都在 `48530cc` 上确认过转红 ——
   这是"验证力"而非"验证量"的口径，与项目约定一致。

被用例逼出来的 D10/D11/D12/D13/G9 五项，全部属于"代码写好了但没接线"，静态阅读看不出来。
这本身就是黑盒验证价值的直接证据，建议在回顾文档里单独记一笔。

**合并建议**：修掉 2 项 MUST fix 后可合并。其中 MUST-1 是流程性的（改 workflow 触发条件或改
`tasks.md` 的归档顺序），MUST-2 是一行改动。两项都不涉及架构调整。
7 个 pending 场景在 PR 绿之前 **MUST NOT** 在矩阵里置 `done` —— 那会让矩阵重新变成
"自己说自己通过了"的东西，正是本里程碑要消灭的。

## 下一步

1. 修 MUST-1（流水线触发 / 归档顺序）与 MUST-2（镜像名冒号切分）
2. SHOULD fix 逐项决策：修，或在提案「已知欠账」里写明不修的理由
3. 补 changelog（三项 BREAKING：严格解析、退出码语义、`cluster:` 映射 → 列表）
4. 开 PR 到 `develop` 取 CI 信号 → 绿后再把 7 个场景置 `done` → 归档
