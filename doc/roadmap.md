# 路线图 / 未实现设计归档

> 本文件记录"设计过、但当前代码没有消费"的配置形态。留作重启这些方向时的参考，
> 不是可用功能。当前唯一受支持的 schema 见 `doc/02-资源编排.md` 与各示例配置。

## 1. 声明式应用流程（`kind: App` + `flows` + `depends_on`）

早期 `configs/kubernetes-cluster.yaml` 的 `---` 第五段是一份 `kind: App` 文档，
用 `flows` 描述"按什么顺序、在哪些角色上装哪些资源"，用 `depends_on` 表达步骤依赖：

```yaml
kind: App
Apps:
  - name: kubernetes
    flows:
      - name: "安装基础依赖"
        resource: "base-dependencies"
        nodes: "all"
      - name: "初始化 Master"
        resource: "k8s-master-init"
        nodes: "role:master"
      - name: "安装网络插件"
        resource: "flannel"
        nodes: "role:master"
        depends_on: "k8s-master-init"
      - name: "加入 Worker"
        resource: "k8s-worker-join"
        nodes: "role:worker"
        depends_on: "flannel"
```

**现状**：加载器只读第一个 YAML 文档，`---` 之后的整段被静默丢弃；没有任何代码读取
`Apps` / `flows` / `depends_on`。资源的安装顺序由 `resources:` 列表的书写顺序决定，
节点选择由每个资源的 `hosts:` 点名，不存在"按角色批量选节点 + 声明式依赖"。

**重启方向**：若要做，需要显式的多文档加载、DAG 解析与按角色选节点；
在此之前，`depends_on` 表达的顺序靠手工排列 `resources` 顺序实现。

## 2. 节点的 `roles`（复数）

同一版草案里，节点与资源都用 `roles: ["master", "manager"]`（复数列表）描述角色，
配合上面 `nodes: "role:master"` 的写法按角色选节点。

**现状**：`nodes[]` 只有 `role`（单数字符串），见 `pkg/types/nodes.go`。复数 `roles`
不被任何代码识别；严格解析（`UnmarshalStrict`）下，写 `roles:` 会直接报未知键。

## 3. 资源级 `files` / `extra_files`

草案里 `files:` 列出归档内要安装的文件，`extra_files:` 是"目标路径 → 文件内容"的映射，
用于落 systemd unit 这类配置文件。

**现状**：
- `files:` 已实现，语义收窄为 `method: binary` 时指定归档内要装哪些可执行文件
  （见 `pkg/types/resource.go` 的 `Files`）。
- `extra_files:` 已实现，键值都过模板，用于在目标节点落任意文件（见 `ExtraFiles`）。
- 但草案里那种"用 `files:` 罗列已下载产物路径"的写法不存在：下载产物路径由
  `target:` 决定，安装脚本通过 `{{.CacheDir}}` 引用。

## 4. `kind: source` 软件源分发

草案最后一段是 `kind: source`，试图把换源脚本作为一类可分发资源：

```yaml
kind: source
sources:
  - name: aliyun
    url: path
    type: install
    script:
      - "/scripts/CentosAliyunMirrors.sh"
```

**现状**：换源由顶层 `source:` 块驱动（`official` / `aliyun` / `iso` 三种模式），
在 `method: package` 装包前于**目标节点**上执行，见 `doc/03-环境初始化.md`。
不存在把换源脚本作为独立资源分发的 `sources:` 列表。

## 5. 旧 schema 逐条改写指引

下表把旧草案里的写法对到当前受支持的写法。旧文件本身已在 M0 改写到统一 schema，
这里给手里还留着旧配置的用户一份迁移参照。

| 旧写法 | 当前写法 |
|---|---|
| 多个 `---` 文档 + `kind: Download/Resource/App/Node/source` | 单文档，顶层 `resources:` / `nodes:` / `cluster:` / `images:` 各段并列 |
| `kind: App` + `flows` + `depends_on` 排顺序 | 直接按顺序写 `resources:`，顺序即安装顺序；卸载自动逆序 |
| `flows[].nodes: "role:master"` 按角色选节点 | 在每个资源的 `hosts:` 里点名节点（host 或 IP） |
| 节点 `roles: ["master","worker"]`（复数） | `role: "master"`（单数） |
| 资源级 `roles: ["master"]` | 资源的 `hosts:` 点名对应节点 |
| `kind: source` + `sources:` 列表 | 顶层 `source:` 块，`mode: official/aliyun/iso` |
| `files:` 罗列下载产物 | `target:` 决定产物路径，脚本用 `{{.CacheDir}}` 引用 |

`cluster:` 段的形态从"单对象"统一为"列表"（一份文件可描述多套集群，用
`--cluster-name` / `--cluster-type` 选中）。`kubernetes-cluster.yaml` 那种
把安装步骤全写在 `resources[].post_install` 里的做法仍然可用；更推荐的是
`cluster create` + 内置安装清单（`configs/k8s/*.yaml`），集群版本、runtime、CNI
由 `k8sConfig` 声明，安装内容下沉到清单。
