# configs/k8s —— k8s 集群装什么

`somcli cluster create` 装的每一样东西都在这个目录里，一个资源一个文件。
以前这些内容是 `pkg/cluster/kubernetes.go` 里的 Go 字面量：换一个版本、加一条 sed、
改一个下载地址都要改代码重新编译，而"这套集群到底装了什么"只能靠读 Go 代码回答。

## 文件与资源名的关系

文件名就是资源名，集群配置里 `k8sConfig.resources` 按这个名字引用：

| 文件 | 装什么 |
|---|---|
| `base-dependencies.yaml` | kubeadm preflight 要求的包、内核参数、内核模块、关 swap |
| `cni-plugins.yaml` | `/opt/cni/bin` 下那批二进制（不是网络方案本身） |
| `runc.yaml` | runc |
| `containerd.yaml` | containerd + config.toml（含 SystemdCgroup） |
| `docker.yaml` | docker 静态包（仅 k8s 1.23 及以下可用） |
| `kubernetes.yaml` | kubeadm / kubelet / kubectl 与 kubelet.service |
| `flannel.yaml` | flannel 网络插件清单 |
| `calico.yaml` | calico 网络插件清单 |

`k8sConfig.resources` 留空时按容器运行时取默认组合，见 `pkg/cluster/catalog.go`。
网络插件不在这个列表里：它由 `k8sConfig.cni` 选定，且必须在 worker 加入之前部署。

## 版本从哪来

这些文件里的 `version:` 只是兜底。真正生效的是集群配置里的对应键
（`containerdVersion` / `runcVersion` / `cniPluginsVersion` / `dockerVersion` /
`version` / `cniVersion`），由 `cluster create` 覆盖进来。

## 模板里能用什么

除了引擎通用的 `{{.Version}}` `{{.CacheDir}}` `{{.Filename}}` 之外，
`cluster create` 会额外注入这些集群级变量：

| 变量 | 含义 |
|---|---|
| `{{.Vars.k8sVersion}}` | 集群的 k8s 版本 |
| `{{.Vars.podNetworkCidr}}` | Pod 网段，与 kubeadm `--pod-network-cidr` 同一个值 |
| `{{.Vars.serviceCidr}}` | Service 网段 |
| `{{.Vars.containerRuntime}}` | `containerd` 或 `docker` |
| `{{.Vars.cni}}` | 网络插件名 |
| `{{.Vars.imageRepository}}` | 镜像仓库，未配置时是空串 |
| `{{.Vars.pauseImageVersion}}` | pause 镜像版本，未配置时是空串 |
| `{{.Vars.controlPlaneEndpoint}}` | apiserver 稳定入口，未配置时是空串 |

这几个值不能被 `--set` 覆盖：它们必须与实际下发给 kubeadm 的参数一致，
否则清单里的网段和集群网段对不上，表现是跨节点 Pod 不通而不是"配错了"。

"没配这一项"要用 shell 判空表达（`if [ -n '{{.Vars.imageRepository}}' ]; then ... fi`），
不要用模板的 `{{if}}`：命令是一条条下发的，模板渲染成空串会下发一条空命令。

## 怎么改

这些文件是编译进二进制的（`go:embed`），所以装好的 somcli 也带着它们，
不需要额外分发配置。要改的话有两条路：

1. 改仓库里的文件重新编译；
2. 不改二进制：把整个目录拷出去改，然后
   `SOMCLI_K8S_CATALOG=/path/to/k8s somcli cluster create -f cluster.yaml`。

单个文件也能直接喂给通用安装器，便于单独验一样东西：

```bash
somcli install -f configs/k8s/containerd.yaml
```
