# 任务清单：M2 集群安装归位

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选 + 更新 `test/matrix.yaml`。
> **顺序不可颠倒**：先建 E2E 断言 → 再修行为 → 再外置结构 → 最后加多 master。

## 准备

- [ ] M1 已归档，`current-proposal` 已切到本提案
- [ ] 阅读 `proposal.md` 与技术附录 §4.1 / §4.3 / §5.1 / §5.2 Phase 2 / §6.6
- [ ] 切 `feat-cluster-as-config` 分支

## M2.1 先建断言

- [ ] `hack/mkclusterconfig`：按 `--k8s/--runtime/--cni/--node/--ssh-key` 生成集群配置
- [ ] `test/cluster/` 建包，build tag `e2e`
- [ ] `test/fixtures/k8s-nodes/compose.yaml`：privileged systemd 容器（1 master + 1 worker）
- [ ] `.github/workflows/e2e.yml`：nightly + `workflow_dispatch`，失败时收集 kubelet/containerd 日志与 `somwork/`
- [ ] `test/cluster/single_node_test.go` 骨架可运行（此时预期红，作为修复前的复现证据）

## M2.2 修行为（k8s 第一次真的能装上）

- [ ] D4：改用 `kubeadm token create --print-join-command`，替代只取首行的 `extractJoinCommand`（SC-K03）
- [ ] D5：CNI 部署 —— calico 与 flannel 各作为一条 `method: manifest` 资源（SC-K06/K07/K09）
- [ ] D6：按 k8s 版本选 `--cri-socket`，≥1.24 拒绝 dockershim、启用 cri-dockerd（引用已有 `service/cri-docker.*`）（SC-K02/K15）
- [ ] F7：containerd `config.toml` 设 `SystemdCgroup = true`（SC-K01）
- [ ] F8：写 `/etc/sysctl.d/k8s.conf`（`bridge-nf-call-iptables`、`ip_forward`）+ 内核模块（SC-K14）
- [ ] F10（部分）：把 `configureFirewall` 纳入 k8s 流程
- [ ] NodePort 从宿主可访问（SC-K10）
- [ ] `test/local/cluster_kubernetes_test.go`：`SC_K15` socket 选择、join 命令解析（真实两行续行输出）

## M2.3 外置结构（单独提交，最高风险点）

- [ ] 把 `pkg/cluster/kubernetes.go` 中四个硬编码 `types.Resource` 搬到 `configs/k8s/*.yaml`（先保持等价，不改行为）
- [ ] 跑一次 E2E 确认仍绿
- [ ] `ClusterConfig`/`K8sConfig` 补 `resources` 字段；`cluster create` 改为按名称引用组装并交给引擎执行
- [ ] `SwarmConfig` 的 `DefaultAddrPool`/`SubnetSize`/`DataPathPort` 接线或明确移除
- [ ] `method: manifest` 走 `kubectl apply`（SC-M06）、`apply` 命令覆盖（SC-C06）
- [ ] `install.sh` / `Makefile` / `go.yml` 打包带上 `configs/k8s/`
- [ ] 断言 `pkg/cluster` 内 `types.Resource` 字面量为 0
- [ ] 再跑一次 E2E 确认仍绿

## M2.4 多 master / 生命周期 / Swarm / 清理

- [ ] F9：`kubeadm init` 加 `--control-plane-endpoint` + `--upload-certs`，实现 `joinMaster`（SC-K04）
- [ ] F9：无 VIP/LB 时明确拒绝并给出指引，local 组用例覆盖：配置声明 3 个 master 且无 VIP 时，`cluster create` 必须非 0 退出并给出指引（SC-K05）
- [ ] 版本矩阵 1.28 / 1.29 / 1.30（SC-K08）
- [ ] `cluster remove` 后环境干净，无 `/etc/kubernetes` 残留（SC-K11）
- [ ] 扩容新增 worker（SC-K12）、缩容 drain + delete + reset（SC-K13）
- [ ] Swarm 全生命周期多节点覆盖（SC-S01..S04）
- [ ] F11：清理 `generateKubeadmConfig`、`SSHExec`、`SSHExecWithOutput`、`SSHClient`、`RsyncCopy`、`GetDownloadURL`、`NormalizeVersion` 等死代码（删除前 grep 确认零引用，独立提交）

## 测试

- [ ] `go test ./...` 全绿（默认层仍为秒级）
- [ ] `go test ./test/cluster/... -tags=cluster` 在 CI 全绿
- [ ] D4/D5/D6/F7/F8/F9 均有回归用例，且在 M2.2 前能复现失败

## 评审

- [ ] 通过 expert-review（产出 `review.md`）
- [ ] 修复所有 MUST fix 项
- [ ] SHOULD fix 项已评估

## 归档

- [ ] changelog 补条目，配置键变更（`resources` 生效、`runtime`→`containerRuntime`）单列为 BREAKING
- [ ] `git mv changes/proposals/20260818-migrate-phase2-cluster-as-config/ changes/archive/`
- [ ] `current-proposal` 切到 `20260818-migrate-phase3-platform-compat`

## 提交与推送

- [ ] 通过 ci-gate
- [ ] commit message 符合 Conventional Commits
- [ ] 分支为 `feat-cluster-as-config`
- [ ] 推送需用户确认