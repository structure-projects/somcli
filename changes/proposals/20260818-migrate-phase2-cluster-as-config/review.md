# 评审报告：20260818-migrate-phase2-cluster-as-config

| 字段 | 值 |
|---|---|
| 评审日期 | 2026-08-21 |
| 评审人 | AI（expert-review skill） |
| 范围 | M2 提案对应的提交：`b8f8dbd`..`c2b2c16`（M1 归档点 `f1be310` 之后） |
| 结论 | ⚠️ 有条件通过 |

结论的前提：本机层（`go test ./...`、五组 `go vet`、gofmt、三条静态守卫）全绿；
真正装集群的两组（`test/cluster`、`test/swarm`）按提案约定随每晚 `e2e.yml` 跑，
**尚未观察到一次转绿**，matrix 里这些场景保持 `pending`。本报告不替那条流水线背书。

## 符合性

提案 M2.1–M2.4 的目标逐项核对：

- **D4（join 命令）**：改为 `kubeadm token create --print-join-command` 现场生成，
  不再从 init 输出按行刮、不再落盘存过期命令。✅
- **D5（CNI）**：flannel/calico 作为 `method: manifest` 资源在 init 之后、worker 之前 apply。✅
- **D6 前半**：`containerRuntime: docker` 且 k8s ≥ 1.24 在校验阶段拒绝；cri-dockerd 后半按提案留到后续。✅
- **F7/F8/F10**：SystemdCgroup、sysctl/modules、防火墙接入流程。✅
- **外置结构**：8 个资源搬到 `configs/k8s/*.yaml` 并 go:embed，`pkg/cluster` 内 Resource 字面量为 0（有 CI 守卫）。✅
- **F9 多 master**：`--upload-certs` + 现场重新上传证书 key + `--control-plane` 加入，fixture 用 haproxy 验证。✅
- **SC-K08 版本矩阵 / SC-K11 remove 干净 / SC-K12/K13 扩缩容 / SC-S01..S06 swarm**：均有对应用例。✅
- **F11 死代码**：独立提交清理，go.mod 随之 tidy。✅

一处提案自述的 BREAKING「`runtime`→`containerRuntime`」与代码现状对不上：master 上
类型字段已经叫 `containerRuntime`，不存在叫 `runtime` 的旧字段。changelog 按提案原文
保留了这条迁移说明（严格 YAML 下写 `runtime` 确实会被未知字段拒绝），但它不是这一版
新引入的改名。不影响行为。

## 测试覆盖

- 测试一律黑盒，`test/` 下无 `github.com/structure-projects/somcli` 的 import（CI 守卫确认）。
- 关键产品路径都有真节点用例：单节点、CNI 双选、双节点互通+NodePort、HA 双 master、
  版本矩阵、remove 后重装、扩缩容、manifest apply、preflight；swarm 侧生命周期+扩缩容。
- 拒绝类用例落在 `test/local`，每条都能在本机证伪（评审中对版本校验做了关校验即全红的确认）。
- **缺口**：没有 k8s ≥1.24 + docker 被拒绝之外的"docker 路线真装一遍"用例——该路线本就只支持 1.23，
  e2e 矩阵里没有，提案也把 cri-dockerd 留到了后续。可接受。

## 安全

- 配置文件是受信本地输入（操作者自己的集群清单），节点 `host` 会被拼进 `kubectl`/`docker`/`kubeadm`
  命令经远程 shell 执行。当前 host 未做字符集校验，恶意/拼错的主机名理论上可注入。
  评级 NIT：这是本地运维 CLI，能写配置的人本来就有所有节点的 SSH 权限，不构成越权；
  若将来支持外部来源的配置再收紧。
- 没有发现密钥落日志、敏感信息外泄。swarm join token 只在内存中传递（不落盘读旧文件）。

## 性能 / 可靠性

- 扩容只动新节点：依赖安装、节点准备按节点列表收窄，健康节点只重写 `/etc/hosts`，不重跑 swapoff/sysctl。✅
- join/remove 都有后置再查（`nodeInCluster`/`swarmNodeInCluster`），不以"命令退 0"当作成功。✅
- reset 顺序正确（worker 先、首 master 最后），保证 apiserver 活着时每个节点能摘掉自己的 etcd 成员/Node 对象。✅
- kube-proxy iptables 规则故意不清（会误伤 docker NAT），重装时由 kube-proxy 重建，判断合理。

## SHOULD fix（评审中已修）

- [x] **`RemoveK8sCluster` 没补默认值**：`resetK8sNode` 用 `containerRuntime` 拼 `--cri-socket`，
  而 remove 路径没调 `applyK8sDefaults`。配置里没写 runtime（常见写法）时 reset 端点会按
  containerd 拼，docker 集群的 reset 会因多 socket 未显式指定而失败。已在 remove 开头补
  `applyK8sDefaults(config)`。
- [x] **worker join 命令带前导空格**：`" "+joinCommand` 经 SSH 执行虽不报错，但属无意义残留，
  两处（创建流程、扩容流程）已去掉。

## SHOULD fix（留作已知限制，不在本版处理）

- [ ] **摘除 manager/master 不检查 raft/etcd 法定人数**。`remove-node` 只拦"第一台"，
  在一个 3 控制面集群里连续摘两台会让集群失去法定人数且无法自恢复。正确做法是摘之前查
  etcd member 列表 / manager 的 Reachable 状态并在会导致丢多数时拒绝。这是独立的健壮性改进，
  建议单开任务；本版用例不覆盖连续摘除。
- [ ] **swarm remove 按配置顺序 `swarm leave --force`**：若配置把第一台 manager 排在前面，
  它会先于其他 manager 离开（其余 manager 仍能因 `--force` 离开，但顺序不理想）。
  生产影响小（拆整个集群本就是终点操作），记录在此。

## NIT

- 节点 `host` 未做 shell 安全字符校验（见安全段），受信输入下风险可接受。
- `pkg/cluster/swarm.go` 仍混有中英文日志（"Creating Docker Swarm Cluster" 等），与 k8s 侧中文不一致；
  非本提案范围。

## 总评

M2 把"装不上也报成功"的核心问题修实了：CNI、join、cgroup 驱动、多 master、扩缩容、remove
清理每一处都有"真节点上能不能干活"级别的判据，而不是只看退出码。外置安装内容让换版本/加组件
不再需要改代码。代码质量符合规范，注释集中讲"为什么"，死代码已清。

建议：合入前等一次 `e2e.yml`（cluster + swarm 两个 job）转绿并把 matrix 对应场景置 done；
上面两条 SHOULD（法定人数检查）可以作为紧随其后的小提案，不必阻塞本版。
