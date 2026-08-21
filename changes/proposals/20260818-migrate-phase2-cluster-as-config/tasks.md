# 任务清单：M2 集群安装归位

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选 + 更新 `test/matrix.yaml`。
> **顺序不可颠倒**：先建 E2E 断言 → 再修行为 → 再外置结构 → 最后加多 master。

## 准备

- [x] M1 已归档，`current-proposal` 已切到本提案
- [x] 阅读 `proposal.md` 与技术附录 §4.1 / §4.3 / §5.1 / §5.2 Phase 2 / §6.6
- [x] 切 `feat-cluster-as-config` 分支

## M2.1 先建断言

- [x] `test/local/cluster_reject_test.go`：拒绝类用例（SC-K05、SC-K15），写完先跑成三红两绿再改产品
      —— 见 proposal「偏差 1」，这一档提到 cluster 骨架之前做，因为它是本机唯一能给出可信集群信号的部分
- [x] ~~`hack/mkclusterconfig`~~ 不做，改由用例自己写配置并打印路径与内容（见 proposal「偏差 4」）
- [x] `test/cluster/` 建包，build tag **`cluster`**（不是 `e2e`：与 `test/matrix.yaml` 的 `env: [cluster]`
      以及现有 `remote`/`multinode` 的"标签同目录名"惯例对齐；proposal 与本文件原写的 `e2e` 作废）
- [x] `test/fixtures/k8s-nodes/`：privileged systemd 容器（1 master + 1 worker）
- [x] `.github/workflows/e2e.yml`：nightly + `workflow_dispatch`，失败时收集 kubelet/containerd 日志
- [x] `test/cluster/single_node_test.go`（SC-K01）骨架可运行（此时预期红，作为修复前的复现证据）
- [x] `ci.yml` 补 `go vet -tags=…`：三个带标签的测试组不在 `go vet ./...` 视野里，
      写错一个字最坏要等到第二天早上（e2e 每晚一次）才知道

## M2.2 修行为（k8s 第一次真的能装上）

- [x] D4：改用 `kubeadm token create --print-join-command`，替代只取首行的 `extractJoinCommand`（SC-K03）
      —— 连带不再把 join 命令存文件（token 24 小时过期，扩容时读到的是过期命令），`extractJoinCommand` 已删
- [x] D5：CNI 部署 —— calico 与 flannel 各作为一条 `method: manifest` 资源（SC-K06/K07/K09）
      —— 新增配置键 `cni` / `cniVersion`；`method: manifest` 提前到本里程碑实现（见 proposal「偏差 7」）
- [x] D6（前半）：`containerRuntime: docker` 且版本 ≥ 1.24 时在校验阶段拒绝，报错点明 1.24 分界与两条出路（SC-K15）
      顺带把 `configs/config.yaml` 里装不成的 `1.28.2 + docker` 示例改成 `containerd`
- [ ] D6（后半）：启用 cri-dockerd（引用已有 `service/cri-docker.*`），届时把上面的拒绝放宽为
      "缺 cri-dockerd 才拒绝"，并删掉错误文案里"somcli 尚不支持"那句（SC-K02）
- [x] F7：containerd `config.toml` 设 `SystemdCgroup = true`（SC-K01）—— 改完 `grep` 验一遍，sed 没匹配上也会退 0
- [x] F8：写 `/etc/sysctl.d/k8s.conf`（`bridge-nf-call-iptables`、`ip_forward`）+ `/etc/modules-load.d/k8s.conf`（SC-K14）
- [x] F10（部分）：把 `configureFirewall` 纳入 k8s 流程；顺带把写死的 `yum install` 换成
      shell 侧的包管理器分派（借了 M3 一小片，见 proposal「偏差 10」）
- [x] NodePort 从宿主可访问（SC-K10）—— 用例落在 `test/cluster/multi_node_test.go`（单 master 有污点，调度不上 Pod）
- [x] ~~`test/local/cluster_kubernetes_test.go`：join 命令解析~~ 作废：修完之后已无"解析 init 输出"这件事
      （见 proposal「偏差 9」），join 成不成由 cluster 组双节点用例回答
- [x] 顺带修：空版本拼出 404 URL（`applyK8sDefaults`）、`imageRepository` 为空时改坏 containerd 配置、
      `cni-plugins` 资源与 containerd 同名撞幂等状态（见 proposal「偏差 11」）

## M2.3 外置结构（单独提交，最高风险点）

- [x] 把 `pkg/cluster/kubernetes.go` 中四个硬编码 `types.Resource` 搬到 `configs/k8s/*.yaml`（先保持等价，不改行为）
      —— 实际是 8 个资源：base-dependencies / cni-plugins / runc / containerd / docker / kubernetes / flannel / calico
- [ ] 跑一次 E2E 确认仍绿（每晚一次，流水线还没跑到）
- [x] `ClusterConfig`/`K8sConfig` 补 `resources` 字段；`cluster create` 改为按名称引用组装并交给引擎执行
      —— 名字写错在连节点之前就拒绝，报错列出可用名单（SC-K16）
- [x] `SwarmConfig` 的 `DefaultAddrPool`/`SubnetSize`/`DataPathPort` 接线或明确移除
      —— 接线，顺带修 `--advertise-addr`/`--listen-addr` 无条件拼接（见 proposal「偏差 18」）
- [x] ~~`method: manifest` 走 `kubectl apply`（SC-M06）~~ 已在 M2.2 实现（D5 要用它装 CNI），
      本阶段只补 `apply` 命令的覆盖（SC-C06）—— `test/cluster/manifest_test.go`，见 proposal「偏差 19」
- [x] ~~`install.sh` / `Makefile` / `go.yml` 打包带上 `configs/k8s/`~~ 改为 `go:embed` 编译进二进制
      + `SOMCLI_K8S_CATALOG` 覆盖（SC-K17），见 proposal「偏差 13」
- [x] 断言 `pkg/cluster` 内 `types.Resource` 字面量为 0 —— `ci.yml` 静态守卫，另加一条模板变量守卫（「偏差 14」）
- [x] 顺带处理原「待办」：`cluster create --force` 接线（「偏差 17」）
- [ ] 再跑一次 E2E 确认仍绿（同上）

## M2.4 多 master / 生命周期 / Swarm / 清理

- [x] F9：`kubeadm init` 加 `--upload-certs`，实现 `joinMaster`（SC-K04）
      —— `joinMasterNodes` 现场重新上传证书取 key，join 时带 `--control-plane`
      与本机 `--apiserver-advertise-address`；失败不再只打警告。
      fixture 加了第二台 master 与一台 haproxy 充当稳定入口（见 proposal「偏差 20」）
      —— `--control-plane-endpoint` 已在 M2.1 接上（配置键 `controlPlaneEndpoint`），
      但 `joinMaster` 仍是空壳，多 master 目前只是"不再假装成功"
- [x] F9（前半）：无 VIP/LB 时明确拒绝并给出指引，local 组用例覆盖：配置声明 3 个 master 且无 VIP 时，
      `cluster create` 在连节点之前非 0 退出并指名 `controlPlaneEndpoint`（SC-K05）
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

- [ ] changelog 补条目，配置键变更（`resources` 生效、`runtime`→`containerRuntime`）单列为 BREAKING；
      新增键 `controlPlaneEndpoint` 与"多 master 缺 VIP 由假成功改为拒绝"一并记入
- [ ] `git mv changes/proposals/20260818-migrate-phase2-cluster-as-config/ changes/archive/`
- [ ] `current-proposal` 切到 `20260818-migrate-phase3-platform-compat`

## 提交与推送

- [ ] 通过 ci-gate
- [ ] commit message 符合 Conventional Commits
- [ ] 分支为 `feat-cluster-as-config`
- [ ] 推送需用户确认