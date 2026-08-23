# 迁移变更提案：M4 配置归一与文档重建

| 字段 | 值 |
|---|---|
| 提案 ID | 20260818-migrate-phase4-config-and-docs |
| 级别 | major |
| 类型 | migration |
| 创建日期 | 2026-08-18 |
| 创建人 | chuck |
| 状态 | draft |
| 优先级 | medium |
| 总纲 | `changes/proposals/20260818-migrate-arch-convergence/proposal.md` |
| 技术附录 | `doc/提案-架构收敛与测试体系.md` §4.4（G1-G7）、§5.2 Phase 4、§7（文档重建方案） |
| 前置 | M3 完成（文档要描述的是最终能力，提前写必然重写） |
| 场景 | 无新增场景；负责让 82 个场景「可测 = 可用 = 有文档」三者对齐 |


> 验证方式遵循总纲「功能验证约定」：一律黑盒 —— 编译二进制 → 喂真实配置 → 跑真实命令 →
> 断言退出码 / 输出 / 落盘产物 / 节点状态；`test/` 禁止 import `pkg/`；不设覆盖率门槛。

## 现状

| 编号 | 问题 |
|---|---|
| G1 | **三套互不兼容的 cluster schema**：`swarm-cluster.yaml` + `doc/cluster.md`（单对象，唯一可解析）／`kubernetes-cluster.yaml`（多文档 `kind:` + `flows` + `depends_on`，实测报 `cluster type must be specified`）／`config.yaml`（数组 + `master:`/`worker:` + `resources:` 名称引用，意图最完整） |
| G2 | 所有示例的 `hosts:` 全是 `127.0.0.1`，无任何可运行的远程编排样例；带真实 IP 的 `k8s-host.yaml` 没有 `resources:`，两者从未组合 |
| G3 | **`doc/*.md` 中 `hosts` 零命中** —— 项目最核心的远程编排能力完全无文档 |
| G6 | `README.md` 列了不存在的 `internal/`；宣传 `make test` 而当时测试文件为 0；`doc/cluster.md` 称支持 k8s 1.23-1.25 而 configs 用 1.28 |
| G7 | `configs/tools.yaml` 用 `hosts:` 作节点清单键（类型只认 `nodes:`）；`config.yaml` 重复定义 cri-dockerd、`target:` 重复；`swarm-cluster.yaml` 末尾垃圾字符 `###u7dfrdta` |
| — | `configs/kubernetes-cluster.yaml` 的 `flows`/`depends_on`/`Apps`/`roles`/`sources` 全部静默忽略，是纯设计草案 |

M0 的 `doc_commands_test.go` 已把"文档命令必须存在"变成可自动检测（G4/G5），但文档**结构**与**内容缺口**（尤其 G3）仍未解决。

## 目标状态

- 配置 schema 归一到 `config.yaml` 形态，另两套写法给出等价改写示例；未实现的声明式流程（`flows`/`depends_on`）移入 `doc/roadmap.md`；
- 文档覆盖全部 26 个能力域与 82 个场景，远程编排（`hosts`/`nodes`）有专章；
- **命令参考与功能矩阵自动生成**，CI `git diff --exit-code` 校验，文档漂移从根上不可能悄悄发生；
- 每份场景文档的示例配置都是 `configs/examples/` 下被 CI 执行过的真实文件，禁止内联独立 YAML 片段。

目标文档结构（详见技术附录 §7.1）：

| 文件 | 定位 |
|---|---|
| `README.md` | 仅做导航 + 5 分钟上手 |
| `doc/00-功能清单与矩阵.md` | 自动生成：命令树 + `test/matrix.yaml` 渲染 |
| `doc/01-概念与架构.md` | 资源 / 节点 / 编排模型；四步生命周期 |
| `doc/02-资源编排.md` | **核心文档**，填 G3：字段全表、模板变量表、`method` 六种语义、`hosts`/`nodes`、`vars`、幂等与 `on_error` |
| `doc/03-环境初始化.md` | 换源、离线模式、缓存目录布局 |
| `doc/04-场景-工具编排.md` | `tools.yaml` 实战 |
| `doc/05-场景-业务服务编排.md` | 部署/升级/回滚/多环境参数化 |
| `doc/06-场景-Kubernetes.md` | 重写：runtime 与 1.24 分界、CNI、多 master、版本矩阵 |
| `doc/07-场景-Swarm.md` | 重写 |
| `doc/08-镜像管理.md` / `09-仓库管理.md` / `10-离线部署.md` | 原 images/registry/offline 修正 |
| `doc/11-命令参考.md` | 由 `cobra/doc` 自动生成 |
| `doc/12-平台兼容矩阵.md` | 与 M3 的平台矩阵实测结果对应，逐格标支持级别 |
| `doc/13-故障排查.md` | 常见错误与定位方法 |
| `doc/roadmap.md` | 归档 `flows`/`depends_on` 等未实现设计 |
| `doc/设计.md` | 保留为历史记录，页首加注 |
| `doc/提案-架构收敛与测试体系.md` | 保留为历史技术基线（本次迁移的事实来源） |

## 迁移策略

渐进改造，**先建生成器与校验，再写人工文档**：生成器先落地，人工文档才知道哪些内容不必手写（命令与标志一律不手写）。

配置归一采取"新增等价示例 + 保留旧文件一个版本 + 文档给改写指引"的方式，而非直接删除旧文件，避免用户配置突然报错找不到参照。

## 阶段规划

| 里程碑 | 范围 | 完成标准 |
|---|---|---|
| M4.1 | `hack/gendoc`（命令参考）+ `hack/genmatrix`（功能矩阵）+ CI `git diff --exit-code` | `doc/00`、`doc/11` 可重复生成且零 diff |
| M4.2 | 配置 schema 归一（G1）+ 真实远程示例（G2）+ 配置清理（G7、F2 残留） | `configs/**` 全部通过二进制校验；`configs/examples/remote-3node.yaml` 被 multinode 组执行 |
| M4.3 | 人工文档重建（`doc/01`~`doc/07`、`12`、`13`、`roadmap`） | 82 个场景 ID 在场景文档中均有说明（`doc_scenarios_test.go` 校验） |
| M4.4 | 防漂移机制收口 | 5 项机制全部在 CI 生效 |

## 风险评估

- **自动生成的文档与人工文档边界不清，导致生成器覆盖人工内容** → 生成文件（`doc/00`、`doc/11`）页首明确标注"本文件由 `hack/` 生成，请勿手改"，且不在其中放人工段落。
- **要求"每个场景 ID 都在文档中出现"可能催生凑数式文档** → 校验只保证覆盖，评审时按"读者能否照着做出来"判断质量；场景 ID 作为小节锚点而非罗列。
- **schema 归一会让旧配置报错** → 旧文件保留一个版本 + `doc/` 给逐条改写指引 + changelog 标注；下一个大版本再删。
- **文档量大（14 份），容易半途而废** → M4.3 按"核心优先"顺序：`02-资源编排` → `01-概念与架构` → 场景文档 → 辅助文档；每份文档独立提交。

## 回滚预案

文档与配置示例变更不影响二进制行为，回滚 = `git revert`。生成器为新增目录，回滚后 CI 的 docs job 需同步移除。

## 兼容性保证

| 维度 | 说明 |
|---|---|
| 配置 | schema 归一后 `config.yaml` 形态为唯一目标形态；`kubernetes-cluster.yaml` 的 `flows`/`depends_on` 移入 roadmap（原本静默忽略，无行为变化）；`nodes[].roles` 复数与 `role` 单数统一 |
| 文档 | 文件名整体调整（`images.md` → `08-镜像管理.md` 等），旧链接失效，README 提供映射 |
| 行为 | 无代码行为变更（除配置解析对归一后 schema 的支持） |

## 双规范并存期约定

- 老文档：`doc/设计.md` 与 `doc/提案-架构收敛与测试体系.md` 保留为历史记录，页首加注"部分内容已实现/已过时，命令名以 `11-命令参考` 为准"。
- 新文档：命令与标志一律不手写，只引用生成文件；示例配置一律引用 `configs/examples/` 真实文件。
- 边界识别：文件页首是否有"自动生成"标记。

## 影响范围

- **代码**：新增 `hack/gendoc`、`hack/genmatrix`；`pkg/types`（schema 归一涉及的字段与 tag）
- **配置**：`configs/config.yaml`、`configs/kubernetes-cluster.yaml`、`configs/swarm-cluster.yaml`、`configs/tools.yaml`、新增 `configs/examples/remote-3node.yaml`
- **测试**：`test/local/docs_test.go` 扩充（场景 ID 覆盖校验）、`test/local/config_test.go`（引用完整性）
- **CI**：`ci.yml` 增 `docs` job（重新生成并比对）
- **文档**：`README.md` + `doc/` 全量重建

## 验收标准

- [ ] `doc/11-命令参考.md`、`doc/00-功能清单与矩阵.md` 重新生成后 `git diff --exit-code` 零差异
- [ ] 文档中所有示例命令与标志在二进制上可验证存在
- [ ] `test/matrix.yaml` 的 82 个场景 ID 在场景文档中全部出现
- [ ] 每份场景文档的示例配置均为 `configs/examples/` 下被 CI 执行过的真实文件
- [ ] `configs/**` 全部通过二进制校验：严格解析 + 模板渲染 + 引用完整性（`cluster.resources` 引用的名称存在、`hosts` 引用的 IP 在 `nodes` 中）
- [ ] 远程编排有专章（G3 关闭），`hosts` 在文档中命中
- [ ] `doc/12-平台兼容矩阵.md` 与 M3 的实测结果一致
- [ ] 26 个能力域状态在 `doc/00` 中如实标注，未达成项不得标为可用
- [ ] 总纲待决事项 5（`doc/设计.md` 处置）已定论
- [ ] changelog 补条目，文档文件名调整与 schema 归一单列

## 任务清单

详见 `tasks.md`。