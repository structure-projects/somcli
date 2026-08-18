# 迁移变更提案：M1 引擎补全

| 字段 | 值 |
|---|---|
| 提案 ID | 20260818-migrate-phase1-engine-completion |
| 级别 | major |
| 类型 | migration |
| 创建日期 | 2026-08-18 |
| 创建人 | chuck |
| 状态 | draft |
| 优先级 | high |
| 总纲 | `changes/proposals/20260818-migrate-arch-convergence/proposal.md` |
| 技术附录 | `doc/提案-架构收敛与测试体系.md` §4.2（E1-E7）、§5.2 Phase 1、§6.4 |
| 前置 | M0 完成（否则测试结果不可信） |
| 场景 | 27 个（`test/matrix.yaml` 中 `phase: 1`），当前全部 pending |


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
| F5 | 下载器 shell out 到 `wget`，无 curl 回退、未用 `net/http` |
| F6 | `CopyToRemote` 不展开 `~`，与 `RunCommandOnNode` 行为不一致 |
| F14 | images `pull`/`push`/`import` 单张失败一律 `continue` 且整体不返回错误 |

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
- **幂等 guard 用 `check:` 字段可能与 `pre_install` 语义混淆** → 在 `doc/02-资源编排.md`（M4）明确二者边界；`check` 非零即跳过整个资源。
- **compose 安装器重写触及缓存目录名**（`docker-comopose` → `docker-compose`）→ 旧缓存目录失效，属可接受的一次性损失，changelog 标注。
- **`--parallel N` 引入并发后错误汇总与日志交错** → 日志按节点前缀，错误按节点聚合后统一输出。

## 回滚预案

四个子里程碑各自独立提交，回滚 = `git revert` 对应提交。状态文件为新增产物，回滚后残留不影响旧逻辑（旧逻辑不读它）。

## 兼容性保证

| 维度 | 说明 |
|---|---|
| CLI | 新增 `somcli uninstall`、`--set`、`--parallel`、`status`；`registry uninstall` 新增自身标志；`delete` 新增 `-n`。均为新增，无删除 |
| 配置 | `extra_files` 的 struct tag 由 `ExtraFiles` 改为 `extra_files`（原 tag 本就与所有示例不匹配，无实际用户）；新增 `vars`、`check`、`on_error` 均为可选字段 |
| 行为 | **BREAKING**：`method` 从"全部等价于跑脚本"变为按语义分发，依赖旧行为的配置需显式写 `method: script` |
| 缓存 | compose 缓存目录名修正，旧目录失效 |

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

- [ ] `test/matrix.yaml` 中 27 个 `phase: 1` 场景全部 done，累计 49 done
- [ ] 21 个叶子命令中 `🕳` 与 `❌` 计数为 0
- [ ] `configs/tools.yaml` 能真实装出 kubectl / helm / jq 三者
- [ ] install → uninstall 后环境干净（黑盒断言）
- [ ] 连续执行两次结果一致且无重复副作用（黑盒断言）
- [ ] changelog 补条目，`method` 语义变更单列为 BREAKING

## 任务清单

详见 `tasks.md`。