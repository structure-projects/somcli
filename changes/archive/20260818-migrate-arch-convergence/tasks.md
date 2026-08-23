# 任务清单：架构收敛、功能验证与文档重建（总纲）

> 里程碑级任务。逐阶段任务见各子提案的 `tasks.md`。
> 与 `proposal.md` 同目录。每个里程碑完成后勾选，并把 `changes/config.yaml` 的
> `current-proposal` 切到下一个子提案。

## 准备

- [x] 立验收清单：`test/matrix.yaml`（82 场景，人工维护）
- [x] 定功能验证约定：一律黑盒、`test/` 禁止 import `pkg/`、无覆盖率门槛、无元测试
- [x] 阅读技术附录 `doc/提案-架构收敛与测试体系.md`
- [x] 切 `feat-arch-convergence` 分支（`.git/hooks/commit-msg` 禁止在 master/develop 直接提交）
- [x] 提交本提案与各子提案（`docs(changes): 新增变更提案 20260818-migrate-arch-convergence`）

## 里程碑

- [x] **M0** `20260818-migrate-phase0-trusted-signal`：建立可信信号，22 个场景 done
      验收：失败路径退出码非 0 且日志无 `✓`；`somcli <任意命令> --help` 无文件系统与网络副作用
      已归档（`changes/archive/`）
- [x] **M1** `20260818-migrate-phase1-engine-completion`：引擎补全，累计 ~~49~~ **47** 场景 done
      验收：`🕳 空壳` 与 `❌ 不可用` 命令计数归零；每个新增能力都有跑真二进制的用例
      已归档。累计 47 而非 49：phase 1 的 28 个场景里 5 条保持 `pending`（SC-M02 / SC-C01 /
      SC-C02 各只兑现了声明的两个环境之一，SC-D10 / SC-F03 缺一台真实远程主机）。
      缺的是环境而非实现，逐条理由见该提案的「执行期偏差记账」与 `review.md`。
      **M2 的"累计 70"因此顺延为 68**，或在 M2 期间补上这 5 条再回到 70
- [x] **M2** `20260818-migrate-phase2-cluster-as-config`：集群归位
      验收：`pkg/cluster` 内 `types.Resource` 字面量为 0；E2E 单节点矩阵全绿 + 双节点 Join 通过。已归档。
- [x] **M3** `20260818-migrate-phase3-platform-compat`：平台兼容
      验收：6 个发行版镜像上 `method: package` 均成功；构建矩阵单一来源（`build.sh`）。已归档。
- [x] **M4** `20260818-migrate-phase4-config-and-docs`：配置与文档
      验收：`doc/00`、`doc/11` 与代码/场景清单零 diff；全部 93 个场景 ID 在人工文档中均有说明
      （SC-X12 守住，多于原定的 82 个）。已归档。

## 全局验收（每个里程碑都要复核）

- [ ] 该里程碑场景在 `test/matrix.yaml` 中全部 `status: done` —— **未全部完成，按设计保留**：
      93 个场景中 58 done、35 pending。pending 均为需真集群/多节点/远程主机的 E2E
      （SC-K/Swarm 集群组、multinode D10、remote F03 等），由 CI 的 cluster/integration
      job 或后续环境承载；本机 local 组可跑的部分全部 done。转绿前不臆造为已验证。
- [x] 每条 `files` 指向的用例真实存在且跑二进制
- [x] `gofmt -l .` 输出为空；`go vet ./...` 干净
- [x] `test/` 下无任何 `github.com/structure-projects/somcli` import（CI `static` job 守）
- [x] 该里程碑涉及的 D/E/F/G 缺陷均有回归用例，且在修复前能复现失败
- [x] changelog 已补条目，BREAKING 项单列

## 待决事项定论（不定论不得进入 M3 收尾）

- [x] windows 产物去留 —— M3 移除，不产 Windows 二进制（依赖 sh/ssh/scp/systemctl）
- [x] arm64 支持级别（引擎 vs 集群安装）—— 引擎/工具编排完整，k8s 集群安装实验性
- [x] 多 master VIP 方案 —— 用户自备 LB/VIP，somcli 只校验 `controlPlaneEndpoint` 必填
- [ ] E2E nightly 成本策略 —— 延期：属 CI 运维排期决策（按 k8s 版本轮转 vs 每晚全跑），不阻塞本次迁移，留待后续运维提案
- [x] `doc/设计.md` 处置 —— M4 定论：保留为历史记录，页首加注，命令名以 11-命令参考为准
- [x] 是否新增 `somcli validate -f <file>` 只读校验命令 —— M0 已新增（影响 SC-X05 / SC-F07）

## 归档

- [x] 各子提案完成后逐个 `git mv changes/proposals/<id>/ changes/archive/`
- [x] 全部里程碑完成后归档本总纲提案
- [x] 技术附录 `doc/提案-架构收敛与测试体系.md` 在 M4 保留为历史技术基线（页首已注明）