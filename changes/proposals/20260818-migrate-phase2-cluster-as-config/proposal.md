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

- [x] `grep -c "types.Resource{" pkg/cluster/` 结果为 0
- [ ] `test/matrix.yaml` 中 21 个 `phase: 2` 场景全部 done，累计 70 done
- [ ] E2E 单节点在选定矩阵内全绿；双节点 worker Join 成功（D4）且跨节点 Pod 互通（D5）
- [ ] `cluster remove` 后无 `/etc/kubernetes` 残留
- [x] 多 master 无 VIP 时明确拒绝并给出指引（F9）
- [ ] F11 死代码清零（`go vet` + 人工 grep 双确认）
- [x] ~~`configs/k8s/` 随发布产物一起打包~~ 改为编译进二进制，见「偏差 13」
- [ ] changelog 补条目，配置键变更单列为 BREAKING

## 任务清单

详见 `tasks.md`。

## 执行记录（与方案的偏差）

### 偏差 1：先做 local 组的拒绝类用例，再搭 cluster 组骨架

方案里 M2.1 的第一件事是 cluster 组 E2E 骨架。实际先落了 `test/local/cluster_reject_test.go`
（SC-K05、SC-K15）。

理由：cluster 组要跑真实 kubeadm，本机没有条件，用例写完也只能等流水线给结论 ——
而"这份配置结构上就装不成"这件事的判断发生在连节点之前，本机就能给出可信信号。
先把这一档做掉，等于在没有集群环境的机器上也拿到了两条真结论，且它们正是
D6 与 F9 的**可本机证伪**部分。cluster 组骨架仍在 M2.1 范围内，顺序调整不改内容。

「先建断言、再动结构」的次序没有被破坏：这两条用例写完先跑成红（三红两绿），
再改产品让它们变绿。

### 偏差 2：新增配置键 `k8sConfig.controlPlaneEndpoint`

原「兼容性保证」只列了 `resources` 生效与 `runtime`→`containerRuntime`，没有这一项。
SC-K05 要求"多 master 无 VIP 时拒绝并给出指引"，指引必须指向一个真实存在的键，
因此这个键在 M2.1 就得定下来，而不是等 M2.4。

- 不是 BREAKING：新增可选键，单 master 留空即可，老配置逐字不变仍然可用。
- 但**有行为变化**：此前"三个 master 无 VIP"会装完所有节点、init 第一个 master、
  对另外两个打一句 warning 后宣布成功（用户以为自己有 HA），现在在动手前直接拒绝。
  这是把假成功换成明确失败，方向上是修 F9 的前半。
- `kubeadm init` 已接上 `--control-plane-endpoint`（证书 SAN 与 admin.conf 需要它），
  但 `joinMaster` 仍是空壳 —— **多 master 目前仍装不成，只是不再假装成功**。
  真正让另外几个 master 加入属于 M2.4（含 `--upload-certs` 与证书密钥传递）。

### 偏差 3：D6 在本阶段只兑现"拒绝"，不兑现 cri-dockerd

目标状态里写了"cri-dockerd 是一条资源，按 k8s 版本选 socket"。本阶段的实现是：
`containerRuntime: docker` 且版本 ≥ 1.24 时**在配置校验阶段拒绝**，报错点明 1.24 分界、
dockershim 已移除、以及两条出路（改 containerd / 装 cri-dockerd）。

理由：cri-dockerd 作为资源要等 M2.3 的外置（`configs/k8s/*.yaml`）才有落点；
在那之前，不拒绝就意味着用户会装完 docker、装完 kubeadm，在 `kubeadm init` 那一步失败，
机器已经被改过一遍。拒绝是代价最小的正确行为。

M2.3 支持 cri-dockerd 之后，这条拒绝要相应放宽为"缺 cri-dockerd 才拒绝"，
错误文案里"somcli 尚不支持"那句必须同步删掉 —— 否则会变成一句过期的谎话。

顺带修了 `configs/config.yaml`：示例集群段写的正是 `1.28.2` + `docker`，
也就是随产品发布的示例配置本身结构上装不成。已改为 `containerd`。

### 偏差 4：不做 `hack/mkclusterconfig`

方案里列了这个生成器，用意是"手工复现一次 E2E 的安装"。实际不做。

理由：`test/` 不能 import 本仓库的包，所以生成器与用例只能各写一份配置，两份必然漂移 ——
到时候手工复现出来的是生成器的配置，不是用例真正喂给 somcli 的那一份，复现的意义就没了。
改为用例自己写配置，并把**路径与全文**打进测试输出（`clusterConfig`）；
要复现就拿那份配置直接喂 somcli。

### 偏差 5：cluster 组的 build tag 定为 `cluster`，不是 `e2e`

方案正文写 `e2e`，`tasks.md` 里两处写法不一致（一处 `e2e`、一处 `-tags=cluster`），
`test/matrix.yaml` 用的是 `env: [cluster]`。统一取 **`cluster`**：
与目录名、与 matrix 的 env 名一致，也延续 `remote` / `multinode` 的"标签同目录名"惯例。
`e2e` 这个名字只留给流水线文件名（`e2e.yml`）。

### 偏差 6：`ci.yml` 增加带标签测试组的 `go vet`（方案未列）

`go vet ./...` 看不见带 build tag 的包。`test/cluster` 的载体是每晚一次的 e2e.yml，
写错一个字最坏要到第二天早上才知道；`test/remote` / `test/multinode` 同理只有
integration.yml 一个入口。因此在 `ci.yml` 的静态检查里补三条 `go vet -tags=…`，
让编译期错误在 push 时就暴露。这不改产品行为，只是让信号来得及时。

### 偏差 7：`method: manifest` 提前到 M2.2 实现（原属 M2.3）

D5 要求 CNI 是一条 `method: manifest` 资源，而该 method 此前返回"尚未实现"。
不实现它，D5 就只能在 `pkg/cluster` 里手写 `kubectl apply` —— 那是又一处没人走的分支，
且 M2.3 外置时还要拆掉重来。于是把它提到 M2.2：`pkg/installer/method_manifest.go`，
生成 `kubectl apply -f <清单>`，清单来自 `urls`（下载物）或 `extra_files`（配置内联）。

- kubeconfig 的选择写在**生成的 shell 里**（`/etc/kubernetes/admin.conf` 优先，
  再 `$HOME/.kube/config`）：apply 发生在节点上，操作机有没有 admin.conf 与它无关；
  且刚 init 出来的 master 上只有 admin.conf，两个都认才能在"init 完立刻装 CNI"这个时点工作。
- 清单顺序按路径排序：`extra_files` 是 map，遍历顺序随机，不排的话两次执行的 apply
  顺序不同，清单间有依赖时表现为偶发失败。
- `test/local/method_test.go` 里那条"必须报未实现"的占位守卫随之作废，
  换成两条真断言：缺清单来源要明确报错；用 PATH 上的假 kubectl 验证真的调了
  `kubectl apply -f <那个文件>`。改之前后者是红的（报"尚未实现"），改之后绿。

### 偏差 8：CNI 是配置项，新增键 `cni` 与 `cniVersion`

原方案只说"CNI 是一条 `method: manifest` 资源"，没说选哪个、版本从哪来。
`configs/config.yaml` 里也没有这个键。新增两个可选键：

- `cni`：`flannel`（默认）/ `calico`。取值认不出来在**连节点之前**拒绝。
- `cniVersion`：网络方案自身的版本（与 `cniPluginsVersion` 不是一回事，后者是
  `/opt/cni/bin` 下的二进制插件包）。留空取内置默认值。

不是 BREAKING：老配置逐字不变仍可用，留空即 flannel。

两份清单都要按 `podNetworkCidr` 改网段，且改完必须 `grep` 验一遍：
清单换了键名时 `sed` 匹配不上也退 0，那就成了"改过了"的假象，而错要到跨节点不通时才暴露。
calico 的 `CALICO_IPV4POOL_CIDR` 在官方清单里是注释掉的，不显式设它会用自己的默认
`192.168.0.0/16` —— 与 kubeadm 的 `--pod-network-cidr` 对不上。

### 偏差 9：join 命令改为在 master 上现场生成，不再存文件

D4 的方案是"改用 `kubeadm token create --print-join-command`"，这里连带去掉了
"把 join 命令写进 `somwork/` 再读回来"这一步。除了两行续行会被刮掉半条命令之外，
存文件还有第二个问题：默认 token 24 小时过期，扩容时读到的是一条过期命令。

因此 `tasks.md` 里那条 `test/local/cluster_kubernetes_test.go`（join 命令解析，
喂真实两行续行输出）作废 —— 修完之后已经没有"解析 init 输出"这件事了，
本机也无从黑盒观察 master 上生成的命令。join 到底成不成由 cluster 组的双节点用例回答。

### 偏差 10：借了 M3 一小片 —— 包管理器判断下沉到生成的 shell

F10 原计划只"把 `configureFirewall` 纳入流程"，发行版抽象整体留给 M3。
但基础依赖那一步写死了 `yum install -y socat conntrack ebtables ipset`，
而 E2E 的节点是 ubuntu —— 集群安装在第一步就失败，后面的修复一条都验不到。

改为在**生成的 shell 里**按 `command -v` 依次认 apt-get / dnf / yum / apk / zypper，
认不出来明确报错。判断必须发生在目标节点上：拿操作机的发行版去挑包管理器，
一到远程就是错的（与 `--sudo` 同一个道理）。完整的发行版抽象仍属 M3。

### 偏差 11：顺带修的两处（不在方案清单里，但不修就装不成）

- **空版本拼出 404**：`containerdVersion` / `runcVersion` / `cniPluginsVersion` 等留空时，
  下载 URL 渲染成 `.../download/v/cni-plugins-linux-amd64-v.tgz`。新增 `applyK8sDefaults`
  统一填默认值，并把每一个填进去的值打印出来 —— 装上的东西与配置不一致时，
  用户得能在日志里看到到底装的是哪个版本。
- **`imageRepository` 为空时把 containerd 配置改坏**：原先无条件拼两条 sed，
  空值时生成 `sed 's|k8s.gcr.io||g'` 与 `sandbox_image = "/pause:"`，
  报错指向"沙箱镜像拉取失败"，根因看不出来。改为仅在配了仓库时才拼。
- **`cni-plugins` 资源与 containerd 同名**：幂等状态按名字+版本记，两条资源共用
  `Name: "containerd"`，先装的那条会被后装的覆盖，"这台机器装过 cni-plugins 没有"
  永远查不到正确答案。改名为 `cni-plugins`。

### 偏差 12：cluster 组用例共用一个 workdir，并自行清场

一条用例一个 `--workdir` 的话，每条都要重新下 containerd + runc + cni-plugins +
kube 三件套（几百兆），一晚上的额度全花在下载上。改为全包共用一个 workdir ——
"装过的不再装"正是引擎的幂等能力，用例之间只用 `kubeadm reset` 抹掉 k8s（`resetCluster`），
不重装二进制。

清场用 `kubeadm reset` 而不是 `somcli cluster remove`：用例之间的清场必须与被测行为无关，
否则 remove 有问题时坏掉的是下一条用例，报错指向的地方全是错的。
`cluster remove` 自身由 SC-K11 单独验。

SC-K10（NodePort）从 `single_node_test.go` 移到 `multi_node_test.go`：
单 master 带 control-plane 污点，普通 Pod 调度不上去，NodePort 后面没有后端可言。

### 偏差 13：安装清单编译进二进制 + 环境变量覆盖，取代"打包带上 configs/k8s"

方案的 M2.3 写的是「`install.sh` / `Makefile` / `go.yml` 打包带上 `configs/k8s/`」。
实际改为 `//go:embed configs/k8s/*.yaml` 编译进二进制，另给一个 `SOMCLI_K8S_CATALOG`
环境变量指向替代目录。

理由：`install.sh` 只拷一个二进制，从来不分发 `configs/`。改成"必须有配置文件"的话，
装好的 somcli 第一次 `cluster create` 就会因为找不到 `configs/k8s` 而失败，
而用户完全看不出自己少拿了什么东西 —— 这比"要改就得重编译"糟得多。

- 嵌入的代码只能写在仓库根的 `package main`：`go:embed` 的路径不能穿 `..`，
  而这批 YAML 必须留在 `configs/` 下（用户要能看见、能照着改）。
  `main` 通过 `cluster.SetBuiltinK8sCatalog` 注入，`pkg/cluster` 自己不嵌入任何东西。
- `SOMCLI_K8S_CATALOG` 是**取代**而不是合并：合并的语义下，用户以为自己换掉了
  containerd 的装法、实际用的还是内置那份，而两者的差别要到节点上才暴露。
- 指向一个不存在的目录当场报错，不静默退回内置清单。
- 由此「完成标准」里的「`configs/k8s/` 随发布产物一起打包」不再适用：产物就是二进制本身。

### 偏差 14：新增两条 CI 静态守卫

外置之后有两类错在本机与 PR 门禁上都看不见，`ci.yml` 的 `static` 各补一条：

- **`configs/k8s` 用到的 `{{.Vars.xxx}}` 必须有人注入**：模板渲染开着 `missingkey=error`，
  用了一个 `cluster create` 不注入的变量，整条命令会在节点上报 `map has no entry for key`，
  而那要等每晚的 e2e 才看得到。守卫把 `configs/k8s/*.yaml` 里的变量名与
  `setK8sTemplateVars` 对齐。（只查 `*.yaml`：同目录 README 也写着这些变量名，那是文档。）
- **`pkg/cluster` 内不得有 `types.Resource{` 字面量**：这正是本里程碑的验收项
  「资源定义字面量为 0」，写成守卫才不会被下一次改动悄悄破坏。

两条都验过会红：临时给 flannel.yaml 加一个 `{{.Vars.nosuchvar}}`、临时留一个字面量，
各自都能让流水线失败。

### 偏差 15：模板变量多一层优先级 `computedVars`

外置的清单要拿到 Pod 网段、镜像仓库这些集群级的值，于是 `pkg/utils` 的变量合并
从两层（配置文件 `vars:` < `--set`）变成三层，somcli 自己推导出来的值排最高。

理由：允许 `--set podNetworkCidr=...` 覆盖，就等于允许 CNI 清单里的网段与实际下发给
`kubeadm --pod-network-cidr` 的值对不上，而那种错的表现是跨节点 Pod 不通，
不是"配错了"。

配套一条写法约定（记在 `configs/k8s/README.md`）：**"没配这一项"要用 shell 判空表达**
（`if [ -n "{{.Vars.x}}" ]`），不要用模板的 `{{if}}` —— 命令是逐条下发的，
渲染成空串的那条会变成一条空命令下发到节点。

### 偏差 16：`docker` 资源仍然强制重装，只是记下来

`base-dependencies` 与 `docker` 两条不看幂等状态、每次 `cluster create` 都重跑
（`alwaysApply`）。前者是必须的：`swapoff` 与 `modprobe` 的效果重启就没了，
状态库却会说"装过了"，结果是重启后再装一次集群，kubeadm 在 preflight 被 swap 挡住。

后者看着像是当年顺手写的，没有同样的理由。但这次只搬家不改行为，原样保留并记在此处。

### 偏差 17：`cluster create --force` 接线（原「待办」）

与 `install --force` 同义：不看幂等状态重装一遍。原来的说明写的是
"Force creation even if prechecks fail"，但它从来不影响预检（跳预检是 `--skip-precheck`），
说明与行为都改掉了。

`CreateSwarmCluster` 的 `force` 参数仍然没有消费点 —— Swarm 那条路的资源安装还没走
通用引擎，留到 M2.4 的 Swarm 那一档一起看。

### 偏差 18：`swarmConfig` 顺带修了 advertise/listen 无条件拼接

方案只要求给 `defaultAddrPool` / `subnetSize` / `dataPathPort` 接线（这三个键此前在
类型里有、却从没人读，配了完全没有效果，而这三件事装完就改不了了）。实际发现
`--advertise-addr` 与 `--listen-addr` 是无条件拼上去的：配置里不写就成了
`docker swarm init --advertise-addr  --listen-addr `，docker 报的是标志缺参数，
看不出是配置少了两个键。一并改为逐项判空。

`subnetSize` 只在给了地址池时才拼：单独给掩码长度，docker 直接报错。

### 偏差 19：`apply` 的覆盖（SC-C06）只能在 cluster 组

`apply` 先认当前机器上是什么集群，认不出直接退 1 —— 在操作机上永远只能验到
"没有集群"这一个分支。而 somcli 是在**节点上**才看得到集群的（kubeconfig 在 master 的
`$HOME/.kube` 下），因此 `test/cluster/manifest_test.go` 把二进制送进 master 再在那儿执行，
与用户登上 master 敲命令是同一回事。

判据取"集群里真的多了这个对象"，不取退出码：打印 "Resources applied successfully"
这件事本身不需要任何东西真的生效。成对留了反面（喂一份不是清单的文件必须非 0 退出）。

节点上那份二进制单独按 `GOOS=linux` 编：TestMain 编的那份是给操作机用的，
操作机可能是 macOS，否则这条用例会挂在"二进制格式不对"这种与被测行为无关的地方。

### 待办：`cluster create --force` 是个死标志

`cmd/cluster.go` 读了 `--force` 并传进 `CreateK8sCluster`，但该参数在整个 k8s
安装流程里没有任何消费点 —— 加了 `--force` 与不加完全一样，装过的资源仍按状态跳过。
本里程碑不动它（改动会牵到 `installer.Force` 的传递路径），记在此处待 M2.3 外置时一并处理。

**M2.3 已处理**：见「偏差 17」。

### E2E 前置：宿主上做的三件事

节点是共享宿主内核的容器，因此下面三件事只能在 `e2e.yml` 里对宿主做，
**不是** somcli 该做的事，也不能算进它的验收：

- `swapoff -a`：kubeadm 读的 `/proc/swaps` 是宿主全局的，容器里关不掉；
- `modprobe br_netfilter overlay`：模块没在宿主加载，节点里连对应的 sysctl 都不存在；
- cgroup 用宿主命名空间（compose 的 `cgroup: host`）：私有命名空间下 kubelet 与
  containerd 看到的 cgroup 路径对不上，Pod 起不来。

somcli 该做的是在**节点上**写 `/etc/sysctl.d/k8s.conf` 与加载模块（F8），
这一条由 SC-K01 单独断言，不能被上面的宿主准备顶替掉。
