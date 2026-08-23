# 评审报告：20260818-migrate-phase4-config-and-docs

| 字段 | 值 |
|---|---|
| 评审日期 | 2026-08-24 |
| 评审人 | AI（expert-review skill） |
| 评审范围 | `feat-config-and-docs` 分支上 M4 的全部提交（d3890e1 M4.1 → b9d22ae M4.4） |
| 结论 | ✅ 通过（无 MUST fix；1 项 SHOULD 已在评审中修复，1 项 NIT） |

## 评审方法

按 proposal 的验收标准逐条核对，不局限于 diff：

- **代码**：通读 `hack/gendoc`、`hack/genmatrix`、`cmd/root.go`（NewRootCommand 导出）、
  `cmd/validate.go`（hosts/清单引用校验）、`pkg/cluster/catalog.go`（ValidateConfigRefs）。
- **文档准确性**：doc/06 默认版本、doc/12 构建矩阵与发行版矩阵、doc/13 报错字符串、
  doc/02 字段表与 method 语义，全部回到 `pkg/`、`configs/k8s/`、`build.sh`、
  `.github/workflows/` 比对源码，而非信任 diff。
- **测试**：`go test ./...` 全绿（local 组 116.8s）；remote/multinode/cluster/swarm
  四个 build tag 下 `go vet` 干净；确认 `test/` 无 `pkg/` 导入（黑盒红线）。

## 符合性（proposal 目标覆盖）

| 验收标准 | 结论 | 证据 |
|---|---|---|
| doc/11、doc/00 重生成零 diff | ✅ | `go run ./hack/gendoc`、`go run ./hack/genmatrix` 后 `git diff` 为空；CI docs job 守 `git diff --exit-code` |
| 文档命令/标志真实存在 | ✅ | SC-X04 `doc_commands_test.go` 跑二进制 `--help` 逐条验 |
| 全部场景 ID 在人工文档出现 | ✅ | SC-X12 `doc_scenarios_test.go`，当前 93 个 ID 全命中 |
| 示例配置为 configs/examples/ 真实文件 | ✅ | SC-X13 禁止内联独立 YAML；SC-E07 在三节点 fixture 真跑 remote-3node.yaml |
| configs/\*\* 通过二进制校验 | ✅ | SC-X05 严格解析+模板渲染+引用完整性；`configs/k8s/` 为编译进二进制的清单片段，非独立配置，测试显式 SkipDir |
| 远程编排有专章、hosts 命中（G3） | ✅ | doc/02 §3「hosts 与 nodes 的对应」+ doc/04「远程装工具」+ remote-3node 示例 |
| doc/12 与 M3 实测一致 | ✅ | 4 产物/build.sh、3 runner、6 发行版逐格核对 ci.yml 与 integration.yml |
| 未实现设计归档 roadmap | ✅ | flows/depends_on/Apps/roles/sources 入 doc/roadmap.md 并附改写指引 |
| doc/设计.md 处置定论 | ✅ | 加历史滞后横幅，指向 01/11/roadmap |

proposal 写的是「82 个场景」，实际 matrix 已增至 93（各阶段持续补入）。SC-X12 强制覆盖
**当前全部** ID，比原目标更强，不构成偏差。

## 代码评审要点

- **M4.1 生成器**：`NewRootCommand` 用 `sync.Once` 包住 version/docker/compose 三个
  构造式子命令，避免 gendoc 与 Execute 重复 AddCommand；其余子命令在各 `cmd/*.go` 的
  `init()` 里注册，包加载即齐全。genmatrix 的分组从 matrix.yaml 的
  `# ---------- SC-X 标题 ----------` 注释反射，不硬编码，新增分组无需改工具。两份生成
  文件页首均有自动生成标记，关闭了 cobra 的 AutoGenTag 日期（否则每次重生成必有 diff）。
- **M4.2 引用校验**：`validateHostRefs` 的判定（host/ip 命中、localhost 三个字面量
  白名单、无 nodes 时单独报错文案）与运行时 `utils.GetNode` 完全一致，不存在「validate
  放过、install 翻车」或反向的缝隙。`ValidateConfigRefs` 复用了集群安装期已有的
  `validateK8sResourceNames`，没有另写一套易漂移的清单名判断。
- **schema 归一**：`LoadConfig` 用 `yaml.UnmarshalStrict`，旧 schema 的
  flows/depends_on/roles(复数)/Apps/sources 现在是明确报错而非静默忽略；
  `kubernetes-cluster.yaml` 已用真实字段重写，旧写法的逐条改写指引在 roadmap。
- **SC-E07**：把仓库里的 remote-3node.yaml 原样喂给三节点 fixture（只替换 sshKey 路径），
  逐节点断言 node.txt/target.txt 落点，是真正的黑盒端到端，不是「能解析就算」。

## MUST fix

无。

## SHOULD fix

- [x] **场景锚点归位**（评审中已修复，未单独提交）：SC-C01（docker 安装/卸载）原挂在
  doc/07 Swarm，而权威说明在 doc/docker.md；SC-C02（compose 安装）doc/docker-compose.md
  无锚点。已给 doc/docker.md 加 SC-C01、doc/docker-compose.md 加 SC-C02，并把 SC-C01
  从 doc/07 移除（doc/12 保留 SC-C02，因其跨架构维度确实属于兼容矩阵）。修复后
  SC-X12/SC-X13/SC-X04 全绿。

## NIT

- `hack/genmatrix` 的 `statusEmoji` 函数名误导：它返回文本 `"done"`/`"pending"`，
  并不产出 emoji。可改名 `statusLabel`。不影响行为与防漂移，留给后续顺手清理。

## 安全性

无新增攻击面。生成器只读写仓库内 doc/；validate 新增的校验只读配置与内置清单、不连节点、
不写盘。既有 `images import` 的路径穿越防护在本分支之前已修好，未被回归。

## 测试覆盖

关键路径（命令树生成、引用校验、示例可执行、文档防漂移）均有黑盒用例守住：SC-X04/X05/
X12/X13（local）、SC-E07（multinode）。符合「黑盒功能验证、不设覆盖率门槛、不为可测性
改生产代码」的项目约定。

## 评审意见

M4 达成了 proposal 的核心目标——命令参考与功能矩阵由代码/测试驱动生成，文档漂移从
「靠评审盯」变成「CI 直接红」；schema 归一用严格解析+引用校验把原本要装到一半才暴露的
配置错误前移到 validate；场景文档的示例全部指向被 CI 真实执行过的文件。代码改动小而内聚，
文档内容经源码抽查准确。无 MUST 项，建议进入归档阶段。
