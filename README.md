# somcli

somcli（structure-ops-cli）是一个"环境初始化 + 服务/工具编排"引擎：
用一份 YAML 声明要装什么、在哪些节点上装，somcli 负责下载、渲染、分发、执行和记录状态。
Docker / Kubernetes / Swarm 集群安装是它的一类应用场景，而不是全部。

## 5 分钟上手

```bash
# 1. 安装二进制（linux/darwin × amd64/arm64）
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
curl -L "https://github.com/structure-projects/somcli/releases/latest/download/somcli-${os}-${arch}" -o /usr/local/bin/somcli
chmod +x /usr/local/bin/somcli
somcli version

# 2. 挑一份示例配置，先校验再装
somcli validate -f configs/examples/k8s-single.yaml
somcli cluster create -f configs/examples/k8s-single.yaml
```

更多示例在 [`configs/examples/`](configs/examples/)：单/多 master k8s、swarm、
工具编排（kubectl/helm/jq）、业务服务、三节点远程编排。

## 常用命令

```bash
# 配置与编排
somcli validate -f config.yaml          # 只校验，不安装
somcli install -f config.yaml           # 装配置里的资源
somcli install -f config.yaml -n <name> # 只装某一个资源
somcli uninstall -f config.yaml         # 按 remove_scripts 卸载
somcli status                           # 看装过什么、装在哪

# 集群
somcli cluster create -f config.yaml
somcli cluster create --cluster-type swarm -f config.yaml
somcli cluster add-node -f config.yaml --node <host>
somcli cluster remove -f config.yaml

# 镜像与仓库
somcli images pull -f configs/examples/image-list.txt
somcli images export -f configs/examples/image-list.txt -o images.tar.gz
somcli images import -i images.tar.gz
somcli registry install -H harbor.example.com -v v2.5.0

# 离线
somcli download -f config.yaml          # 联网机预热缓存
somcli install --offline -f config.yaml # 内网机只认缓存
```

全局标志 `--config` / `--github-proxy` / `--workdir` / `--debug` / `--offline` /
`--set` 写在子命令后面。完整命令与标志见 [`doc/11-命令参考.md`](doc/11-命令参考.md)。

## 文档导航

| 文档 | 内容 |
|---|---|
| [00 功能清单与矩阵](doc/00-功能清单与矩阵.md) | 全部场景与验证状态（自动生成） |
| [01 概念与架构](doc/01-概念与架构.md) | 资源/节点模型、四步生命周期、代码结构 |
| [02 资源编排](doc/02-资源编排.md) | 配置字段全表、六种 method、模板变量、幂等 |
| [03 环境初始化](doc/03-环境初始化.md) | 换源、离线模式、工作目录布局 |
| [04 场景-工具编排](doc/04-场景-工具编排.md) | kubectl / helm / jq 实战 |
| [05 场景-业务服务编排](doc/05-场景-业务服务编排.md) | 部署/升级/回滚/多环境 |
| [06 场景-Kubernetes](doc/06-场景-Kubernetes.md) | runtime 分界、CNI、多 master、版本矩阵 |
| [07 场景-Swarm](doc/07-场景-Swarm.md) | manager/worker、swarmConfig |
| [08 镜像管理](doc/08-镜像管理.md) | pull/push/export/import |
| [09 仓库管理](doc/09-仓库管理.md) | Harbor 安装、镜像同步 |
| [10 离线部署](doc/10-离线部署.md) | 联网机准备 → 内网机安装 |
| [11 命令参考](doc/11-命令参考.md) | 所有命令与标志（自动生成） |
| [12 平台兼容矩阵](doc/12-平台兼容矩阵.md) | OS/架构/发行版支持级别 |
| [13 故障排查](doc/13-故障排查.md) | 常见报错与定位 |
| [roadmap](doc/roadmap.md) | 未实现设计的归档 |

`doc/设计.md` 与 `doc/提案-*.md` 是历史记录，命令名以 11-命令参考为准。

## 旧文档链接映射

旧的平铺文档已拆分重命名：

| 旧文件 | 新文档 |
|---|---|
| `doc/images.md` | [08-镜像管理](doc/08-镜像管理.md) |
| `doc/registry.md` | [09-仓库管理](doc/09-仓库管理.md) |
| `doc/offline.md` | [10-离线部署](doc/10-离线部署.md) |
| `doc/cluster.md` | [06-Kubernetes](doc/06-场景-Kubernetes.md) / [07-Swarm](doc/07-场景-Swarm.md) |

## 开发

```bash
make build        # 当前平台二进制
make build-all    # linux/darwin × amd64/arm64（委托 build.sh，单一来源）
make test         # go test ./...
make docs         # 重新生成 doc/00 与 doc/11
make fmt && make vet
```

项目结构：

```
somcli/
├── cmd/          # cobra 命令定义
├── pkg/          # 功能实现（installer / cluster / images / registry / utils ...）
├── configs/      # 内置安装清单与示例配置（examples/）
├── doc/          # 用户文档
├── test/         # 黑盒功能测试（local / remote / multinode / cluster / swarm）
├── hack/         # 文档与矩阵生成器
├── build.sh      # 构建矩阵单一来源
└── main.go
```

测试是黑盒功能验证：`test/` 不 import 本仓库 `pkg/`，通过编译出的二进制驱动；
不设覆盖率门槛。
