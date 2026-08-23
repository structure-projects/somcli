# 任务清单：M4 配置归一与文档重建

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选。
> 原则：命令与标志一律不手写，只引用生成文件；示例配置一律引用 `configs/examples/` 真实文件。

## 准备

- [x] M3 已归档，`current-proposal` 已切到本提案
- [x] 阅读 `proposal.md` 与技术附录 §4.4 / §5.2 Phase 4 / §7
- [x] 切 `feat-config-and-docs` 分支
- [x] 定论总纲待决事项 5（`doc/设计.md` 处置）：保留为历史记录，页首加注"部分内容已实现/已过时，命令名以 11-命令参考 为准"

## M4.1 生成器与防漂移

- [x] `hack/gendoc`：由 `cobra/doc` 生成 `doc/11-命令参考.md`（导出 `cmd.NewRootCommand` 供工具遍历）
- [x] `hack/genmatrix`：由命令树 + `test/matrix.yaml` 渲染 `doc/00-功能清单与矩阵.md`（分组从 matrix 的 `# --- SC-X 标题 ---` 注释提取，不硬编码）
- [x] 两份生成文件页首标注"本文件由 hack/ 生成，请勿手改"
- [x] `ci.yml` 增 `docs` job：重新生成 + `git diff --exit-code`；Makefile 增 `docs/gendoc/genmatrix` 目标
- [ ] `test/local/docs_test.go`：校验全部场景 ID（当前 91 个）在场景文档中出现 —— 挪到 M4.3 写完场景文档后再启用，否则现在必红

## M4.2 配置归一（G1 / G2 / G7）

- [x] schema 归一到 `config.yaml` 形态；`kubernetes-cluster.yaml` 的 `flows`/`depends_on`/`Apps`/`sources` 移入 `doc/roadmap.md`
- [x] `nodes[].roles`（复数）与 `role`（单数）统一（代码只有单数 `role`；复数 `roles` 归入 roadmap 未实现设计）
- [x] 新增 `configs/examples/remote-3node.yaml`：**真实可运行的远程编排样例**（由 multinode 组 SC-E07 真实执行）
- [x] G7 清理：`config.yaml` 重复的 cri-dockerd 已删（tools.yaml/swarm-cluster.yaml 核查为干净，无历史残留）
- [x] 配置校验用例扩充：引用完整性（`k8sConfig.resources`/`cni` 名称在清单中、`hosts` 命中 `nodes`），SC-X05 覆盖
- [x] 旧 schema 逐条改写指引写入 `doc/roadmap.md` §5

## M4.3 文档重建（按核心优先顺序，每份独立提交）

- [x] `doc/02-资源编排.md`（核心，填 G3）：字段全表、两个模板上下文的变量表、`method` 六种语义、`hosts`/`nodes` 对应、`pre/post/remove_scripts`、`vars`、幂等与 `on_error`
- [x] `doc/01-概念与架构.md`：资源 / 节点 / 编排模型、四步生命周期、引擎与上层命令的关系
- [x] `doc/03-环境初始化.md`：换源、离线模式、缓存目录布局
- [x] `doc/04-场景-工具编排.md`：kubectl / helm / jq 三种 method 实战
- [x] `doc/05-场景-业务服务编排.md`：部署/升级/回滚/多环境参数化
- [x] `doc/06-场景-Kubernetes.md`（重写）：runtime 与 1.24 分界、CNI、多 master、版本矩阵
- [x] `doc/07-场景-Swarm.md`（重写）
- [ ] `doc/08-镜像管理.md` / `doc/09-仓库管理.md` / `doc/10-离线部署.md`（原 images/registry/offline 修正并改名）
- [ ] `doc/12-平台兼容矩阵.md`：与 M3 实测结果对应，逐格标支持级别（完整/实验/不支持）
- [ ] `doc/13-故障排查.md`：常见错误与定位方法
- [ ] `doc/roadmap.md`：归档未实现设计
- [ ] `doc/设计.md` 与 `doc/提案-架构收敛与测试体系.md` 页首加注为历史记录
- [ ] `README.md` 重写为导航 + 5 分钟上手，含旧文档链接映射；删除不存在的 `internal/`
- [ ] 修 G6 遗留：k8s 版本描述与 configs 一致

## M4.4 防漂移机制收口（5 项全部在 CI 生效）

- [ ] `doc/11` 由 `cobra/doc` 生成 + `git diff --exit-code`
- [ ] `doc/00` 由命令树 + `matrix.yaml` 渲染 + `git diff --exit-code`
- [ ] `doc_commands_test.go` 校验文档示例命令与标志真实存在（跑二进制 `--help`）
- [ ] `doc_scenarios_test.go` 校验 82 个场景 ID 在场景文档中出现（纯文本比对，不 import `pkg/`）
- [ ] 场景文档示例配置必须是 `configs/examples/` 下真实文件（禁止内联独立 YAML）

## 测试

- [ ] `go test ./...` 全绿
- [ ] `configs/**` 全部通过二进制校验（严格解析 + 模板渲染 + 引用完整性）
- [ ] `configs/examples/remote-3node.yaml` 被 multinode 组真实执行

## 评审

- [ ] 通过 expert-review（产出 `review.md`）
- [ ] 修复所有 MUST fix 项
- [ ] SHOULD fix 项已评估

## 归档

- [ ] changelog 补条目，文档文件名调整与 schema 归一单列
- [ ] `git mv changes/proposals/20260818-migrate-phase4-config-and-docs/ changes/archive/`
- [ ] 归档总纲提案 `20260818-migrate-arch-convergence`
- [ ] `current-proposal` 置空

## 提交与推送

- [ ] 通过 ci-gate
- [ ] commit message 符合 Conventional Commits
- [ ] 分支为 `feat-config-and-docs`
- [ ] 推送需用户确认