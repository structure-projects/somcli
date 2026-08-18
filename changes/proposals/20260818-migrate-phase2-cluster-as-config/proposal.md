# 迁移变更提案：M2 集群安装归位

| 字段 | 值 |
|---|---|
| 提案 ID | 20260818-migrate-phase2-cluster-as-config |
| 级别 | major |
| 类型 | migration |
| 创建日期 | 2026-08-18 |
| 创建人 | chuck |
| 状态 | draft |
| 优先级 | high |
| 总纲 | `changes/proposals/20260818-migrate-arch-convergence/proposal.md` |
| 技术附录 | `doc/提案-架构收敛与测试体系.md` §4.1（D3-D6）、§4.3（F7-F11）、§5.1 目标形态、§5.2 Phase 2、§6.6 |
| 前置 | M1 完成（集群外置为配置的前提是引擎支持 `method: manifest`、`extra_files`、幂等） |
| 场景 | 21 个（`test/matrix.yaml` 中 `phase: 2`），当前全部 pending |


> 验证方式遵循总纲「功能验证约定」：一律黑盒 —— 编译二进制 → 喂真实配置 → 跑真实命令 →
> 断言退出码 / 输出 / 落盘产物 / 节点状态；`test/` 禁止 import `pkg/`；不设覆盖率门槛。

## 现状

**架构倒置**：`pkg/cluster/kubernetes.go` 把四个 `types.Resource` 硬编码在 Go 里，而不是作为随产品发布的配置去消费通用引擎。
`configs/config.yaml` 里 `cluster[].k8sConfig.resources`（集群 = 命名资源的组合）已经把正确的上层架构画出来了，但该键在类型中不存在，被静默忽略。

k8s 安装当前**结构上不可能成功**：

| 编号 | 问题 |
|---|---|
| D3 | `extra_files` 不落盘 → `containerd.service` / `daemon.json` 从未生成 → `systemctl enable --now containerd` 必失败（M1 已修引擎侧，M2 让 k8s 流程真正用上） |
| D4 | `extractJoinCommand` 只取首行 → kubeadm join 输出跨两行续行，丢 `--discovery-token-ca-cert-hash` → worker 加入必失败 |
| D5 | 完全没有 CNI 网络方案部署（calico/flannel 在 Go 代码零命中）→ 节点永久 NotReady |
| D6 | 使用 `--cri-socket unix:///var/run/dockershim.sock`，而 dockershim 在 1.24 已移除，configs 用 1.28；仓库已有 `service/cri-docker.*` 与 `scripts/getCriDockerd.sh` 但无代码引用 |
| F7 | containerd `config.toml` 未设 `SystemdCgroup = true` → 与 kubeadm 默认 systemd 驱动冲突 → kubelet 起不来 |
| F8 | 未写 `/etc/sysctl.d/k8s.conf`（`bridge-nf-call-iptables`、`ip_forward`）→ preflight 报错 |
| F9 | `joinMaster` 是空壳 `return nil`；`kubeadm init` 无 `--control-plane-endpoint`/`--upload-certs` → 多 master 结构上不可能 |
| F10 | 硬编码 `yum install`（发行版抽象在 M3，本里程碑先纳入 `configureFirewall` 到流程） |
| F11 | `generateKubeadmConfig` 等一批死代码从未被调用 |

## 目标状态

- **`pkg/cluster` 内 `types.Resource` 字面量数为 0**；安装内容全部下沉为 `configs/k8s/*.yaml`；
- `cluster create` 只做编排：按 `k8sConfig.resources` 的名称引用组装资源清单，交由通用引擎执行；
- CNI 是一条 `method: manifest` 资源；cri-dockerd 是一条资源，按 k8s 版本选 socket；
- 单节点 / 双节点 / 3 master HA 三条路径均有真实 E2E 断言；
- Swarm 全生命周期有多节点覆盖。

## 迁移策略

**先建断言，再动结构**。顺序不可颠倒：

1. 先落 cluster 组 E2E 骨架与单节点用例，让"集群能不能起来"变成机器可判定的；
2. 再修 D4/D5/D6/F7/F8 —— 这批修完 k8s 才第一次真的能装上，E2E 从红变绿；
3. 最后做外置（硬编码 Resource → `configs/k8s/*.yaml`），以 E2E 保持绿为唯一验收信号；
4. 多 master 与死代码清理放在外置之后，避免同时改结构与加能力。

## 阶段规划

| 里程碑 | 范围 | 完成标准 |
|---|---|---|
| M2.1 | cluster 组 E2E 骨架 + `e2e.yml` + `hack/mkclusterconfig` | 单节点用例能跑起来（此时预期红） |
| M2.2 | 修 D4/D5/D6/F7/F8，纳入 `configureFirewall` | SC-K01/K02/K03/K06/K07/K09/K10/K14/K15 done |
| M2.3 | 外置为 `configs/k8s/*.yaml`，`cluster create` 改为消费命名资源 | `pkg/cluster` 内 `types.Resource` 字面量为 0，E2E 保持绿；SC-M06、SC-C06 done |
| M2.4 | 多 master（F9）+ 扩缩容 + 生命周期 + Swarm + 清理 F11 死代码 | SC-K04/K05/K08/K11/K12/K13、SC-S01..S04 done |

## 风险评估

- **外置后行为回退且难以定位** → M2.3 之前 E2E 必须已绿；外置分两提交（先搬内容保持等价、再让 `cluster create` 走引擎），每提交后跑一次 E2E。
- **E2E 在 GitHub runner 上跑真实 kubeadm 不稳定（超时、资源不足）** → 单节点用 runner 本机 + SSH 自连接，双节点用 privileged systemd 容器；失败时强制收集 `journalctl -u kubelet`、`containerd config.toml`、`somwork/` 目录。
- **12 个 E2E job（3 版本 × 2 runtime × 2 CNI）成本高** → 见总纲待决事项 4，倾向 PR 只跑 1 组、nightly 轮转。
- **多 master 需要 VIP，CI 内无 LB** → SC-K05（无 VIP 时明确拒绝并给出指引）用 local 组黑盒用例覆盖（跑 `cluster create` 断言拒绝与提示文案）；SC-K04 真实 3 master 视待决事项 3 的结论决定是否用 keepalived+haproxy 由 somcli 自行编排。
- **删除 F11 死代码可能删掉别处隐式依赖** → 删除前 `grep` 全仓库确认零引用，且独立提交便于回滚。

## 回滚预案

四个子里程碑独立提交。M2.3 外置是最高风险点，回滚 = `git revert` 该提交即恢复硬编码路径；因此 M2.3 必须是单独一次提交，不与其他修复混合。

## 兼容性保证

| 维度 | 说明 |
|---|---|
| CLI | `cluster create` / `cluster remove` 的标志不变 |
| 配置 | **BREAKING**：`k8sConfig.resources` 从被静默忽略变为生效并成为组装依据；`k8sConfig.runtime` 键名修正为 `containerRuntime`（原键名写了无效）；`nodes[].roles` 复数写法在 M4 统一 |
| 行为 | k8s 安装从"报告成功但装不上"变为真实可用；`--cri-socket` 按版本选择，1.24+ 不再使用 dockershim |
| 产物位置 | 新增 `configs/k8s/*.yaml` 需随二进制发布，`install.sh` / `Makefile` 需同步打包 |

## 双规范并存期约定

- 老代码：`pkg/cluster/kubernetes.go` 在 M2.2 期间只做行为修复，不重构；M2.3 才做结构外置，届时该文件预计大幅缩减为纯编排。
- 新代码：`configs/k8s/*.yaml` 按资源一文件，命名与 `k8sConfig.resources` 引用一致。
- 边界识别：文件是否在「影响范围」中；`pkg/cluster` 之外的包本里程碑不动。

## 影响范围

- **代码**：`pkg/cluster/{kubernetes,common,detector,types}.go`、`pkg/resources/{kubernetes,swarm}.go`、`cmd/cluster.go`；新增 `hack/mkclusterconfig`
- **配置**：新增 `configs/k8s/*.yaml`（containerd / cri-dockerd / kubeadm 组件 / CNI）；`configs/config.yaml`、`configs/kubernetes-cluster.yaml` 的集群段
- **测试**：`test/cluster/`（build tag `e2e`：single_node / multi_node / ha / cni / lifecycle / scale / manifest）、`test/multinode/swarm_test.go`、`test/local/cluster_*_test.go`、`test/fixtures/k8s-nodes/compose.yaml`
- **CI**：新增 `.github/workflows/e2e.yml`（nightly + `workflow_dispatch`）
- **打包**：`install.sh`、`Makefile`、`go.yml` 需带上 `configs/k8s/`

## 验收标准

- [ ] `grep -c "types.Resource{" pkg/cluster/` 结果为 0
- [ ] `test/matrix.yaml` 中 21 个 `phase: 2` 场景全部 done，累计 70 done
- [ ] E2E 单节点在选定矩阵内全绿；双节点 worker Join 成功（D4）且跨节点 Pod 互通（D5）
- [ ] `cluster remove` 后无 `/etc/kubernetes` 残留
- [ ] 多 master 无 VIP 时明确拒绝并给出指引（F9）
- [ ] F11 死代码清零（`go vet` + 人工 grep 双确认）
- [ ] `configs/k8s/` 随发布产物一起打包
- [ ] changelog 补条目，配置键变更单列为 BREAKING

## 任务清单

详见 `tasks.md`。