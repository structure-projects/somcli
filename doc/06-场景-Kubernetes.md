# 场景：Kubernetes 集群

> 覆盖验收场景：SC-K01、SC-K02、SC-K03、SC-K04、SC-K05、SC-K06、SC-K07、SC-K08、SC-K09、SC-K10、SC-K11、SC-K12、SC-K13、SC-K14、SC-K15、SC-K16、SC-K17、SC-C06。

`somcli cluster create` 按配置在多台节点上装一套 Kubernetes。集群"是什么样"由
`cluster[].k8sConfig` 声明，"装什么"由内置安装清单 `configs/k8s/*.yaml` 决定——
你不需要手写 kubeadm/containerd 的安装脚本。

完整示例：

- 单 master：[`configs/examples/k8s-single.yaml`](../configs/examples/k8s-single.yaml)
- 多 master 高可用：[`configs/examples/k8s-multi.yaml`](../configs/examples/k8s-multi.yaml)
- 手写每一步（不走内置清单）：[`configs/kubernetes-cluster.yaml`](../configs/kubernetes-cluster.yaml)

```bash
somcli validate -f configs/examples/k8s-single.yaml
somcli cluster create -f configs/examples/k8s-single.yaml
```

## 集群配置（`k8sConfig`）

| 字段 | 必填 | 说明 |
|---|---|---|
| `version` | 是 | Kubernetes 版本，**不带 `v` 前缀**（写成 `1.30.0`，不是 `v1.30.0`） |
| `podNetworkCidr` | 是 | Pod 网段，如 `10.244.0.0/16` |
| `serviceCidr` | 是 | Service 网段，如 `10.96.0.0/12` |
| `containerRuntime` | 是 | `containerd`（默认/推荐）或 `docker` |
| `cni` | | `flannel`（默认）或 `calico` |
| `controlPlaneEndpoint` | 多 master 必填 | apiserver 稳定入口 `VIP:6443` |
| `imageRepository` | | 镜像仓库，国内用 `registry.aliyuncs.com/google_containers` |
| `resources` | | 覆盖默认安装的清单资源名列表（见下） |
| `containerdVersion` / `runcVersion` / `cniPluginsVersion` / `dockerVersion` / `cniVersion` | | 留空取内置默认 |

节点在 `cluster[].nodes` 下声明，字段同顶层 `nodes`（`host`/`ip`/`role`/`user`/`sshKey`），
`role` 取 `master` 或 `worker`。至少一个 master。

## 容器运行时与 1.24 分界

- **containerd** 是默认且推荐的运行时，所有支持的 k8s 版本都可用。
- **docker**：somcli 用的是 `--cri-socket unix:///var/run/dockershim.sock`，而 dockershim
  在 **k8s 1.24** 已从 kubelet 移除。因此 `containerRuntime: docker` 只在 k8s **1.23.x 及以下**
  可用；1.24+ 配 docker 会在连节点之前被拒绝。需要 docker 作为运行时请在节点上自行装 cri-dockerd
  提供 CRI 端点（somcli 尚不内置这条路径）。

## CNI 网络插件

- `flannel`（默认）：简单的叠加网络，版本默认 `0.25.6`。
- `calico`：网络策略与 BGP，版本默认 `3.28.2`。
- 不装 CNI 的节点会永久 NotReady，所以这里没有"不装"这个选项。CNI 在 kubeadm init 之后、
  worker 加入之前部署；它夹在这个时点上，不写在 `resources` 列表里。

## 多 master 高可用

配置多个 `role: master` 的节点时，**必须**提供 `controlPlaneEndpoint`（VIP 或负载均衡地址，
形如 `10.0.0.10:6443`）。否则在连节点之前直接报错——三个 master 各有各的 IP，证书与
kubeconfig 必须指向一个不随单机存亡的入口，缺了它装出来的仍是单点集群。

多 master 初始化时会加 `--upload-certs`，把控制面证书存成 kube-system 的 Secret，
后续 master 凭 certificate-key 取回，无需手工拷 `/etc/kubernetes/pki`。

## 版本矩阵与默认值

| 组件 | 默认版本 |
|---|---|
| Kubernetes | 由 `k8sConfig.version` 指定（无默认） |
| containerd | 1.7.22 |
| runc | 1.1.14 |
| CNI plugins | 1.5.1 |
| docker | 20.10.24（仅 1.23 及以下） |
| flannel | 0.25.6 |
| calico | 3.28.2 |

- `version` 不带 `v` 前缀（URL 模板里已经有一个 `v`），写 `v1.30.0` 会拼成 `vv1.30.0` 导致下载 404。
- 其余版本键留空取上表默认值；填了就用你填的。
- 架构：x86_64/amd64 完整支持；aarch64/arm64 可装但标记为实验性。
- OS：仅 Linux。

## 自定义安装内容（`resources`）

默认按运行时装一套清单（containerd 运行时：`base-dependencies`、`cni-plugins`、`runc`、
`containerd`、`kubernetes`；docker 运行时：`base-dependencies`、`docker`、`kubernetes`）。
想加自己的东西，把默认列表抄到 `k8sConfig.resources` 再往后追加——资源名必须存在于内置清单
（`configs/k8s/*.yaml`，`somcli validate` 会校验）。

## 集群运维

- `somcli cluster add-node -f <file> --node <host>`：把配置里已声明的节点加入集群。
- `somcli cluster remove-node -f <file> --node <host>`：drain、reset、删除 Node 对象；
  拒绝删除第一个 master。
- `somcli cluster remove -f <file>`：拆除整套集群。
- `--cluster-name` / `--cluster-type` 在一份配置有多套集群时选定操作对象。

完整命令与标志见 [`11-命令参考.md`](11-命令参考.md)。
