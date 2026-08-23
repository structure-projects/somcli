# 场景：Docker Swarm 集群

> 覆盖验收场景：SC-S01、SC-S02、SC-S03、SC-S04、SC-S05、SC-S06、SC-C01。

`somcli cluster create` 在节点上初始化 Docker Swarm。完整示例：
[`configs/examples/swarm.yaml`](../configs/examples/swarm.yaml)
（单 manager 版见 [`configs/swarm-cluster.yaml`](../configs/swarm-cluster.yaml)）。

```bash
somcli validate -f configs/examples/swarm.yaml
somcli cluster create -f configs/examples/swarm.yaml
```

## 前置

所有节点必须**已装好 Docker**。Swarm 编排不负责装 Docker——需要时先
`somcli docker install` 或用 `method: binary`/你自己的工具装好，再创建 Swarm。

## 节点与角色

`cluster[].nodes` 下至少一个 `role: manager` 的节点；其余用 `role: worker`。
somcli 在第一个 manager 上执行 `docker swarm init`，取回 manager/worker 的 join token，
再在其余节点执行 `docker swarm join`。token 与 join 命令存在
`<workdir>/swarm-join-command.txt`。

## Swarm 配置（`swarmConfig`）

| 字段 | 说明 |
|---|---|
| `advertiseAddr` | manager 广播地址（其他节点连它的 IP） |
| `listenAddr` | 监听地址，默认 `0.0.0.0:2377` |
| `defaultAddrPool` | 自定义默认地址池，如 `10.20.0.0/16` |
| `subnetSize` | 子网掩码长度 |
| `dataPathPort` | 数据通道端口（VXLAN UDP），默认 4789 |

留空的字段不会出现在 `docker swarm init` 命令上——不会传空串标志。

## 运维

- `somcli cluster remove -f <file>`：拆除 Swarm。
- `somcli cluster add-node` / `remove-node`：加入/移除节点。
- `--cluster-name my-swarm`：一份配置里有多套集群时选定。

完整命令与标志见 [`11-命令参考.md`](11-命令参考.md)。
