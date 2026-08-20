# 简化变更提案：补齐 phase 1 的 5 条待验证场景

| 字段 | 值 |
|---|---|
| 提案 ID | 20260821-phase1-leftover-verification |
| 级别 | minor |
| 创建日期 | 2026-08-21 |
| 状态 | draft |
| 分支 | `feat-arch-convergence`（沿用，与 M0/M1 一致） |

## 需求

phase 1 收口时有 5 条场景保持 `pending`，原因不是功能没做，而是**验证环境当时没搭**。
本提案把这 5 条验证补齐，让 `test/matrix.yaml` 的 phase 1 部分真正全绿；
顺带把 M1 遗留的一个未决设计问题（somcli 该不该自己 `sudo`）落定。

前置澄清：这 5 条**不是**"环境搞不到"，而是"当时没在 CI 里搭"。核对现有流水线后确认：
`integration.yml` 已经会装 sshd 并配好免密（远程组的载体）、已经用 docker compose 起三节点、
`ci.yml` 的功能验证已经在 `ubuntu-24.04-arm` 上跑。所缺的是**用例本身**与两个新的 CI 作业。

## 影响

- **新增用例**（不改产品代码）：`test/remote/{dispatch,sshkey}_test.go`、`test/multinode/docker_test.go`、
  `test/local/compose_test.go`（补一条真网络的 arch 断言）
- **CI**：`integration.yml` 新增发行版矩阵作业（SC-M02）；三节点 fixture 需要 privileged 才能装 docker（SC-C01）
- **产品代码**：仅一处 —— 新增 `--sudo` 开关（见下），影响 `pkg/installer/method_package.go` 与 `cmd/install.go`
- **文档**：`changelog/0.3.1-alpha.md`（alpha 阶段走 Y 自增，本提案不含破坏性变更故走 Z... 见「版本」）

## 方案

### 1. SC-D10：远程已存在同 hash 文件则跳过传输

补进 `test/remote/dispatch_test.go`。判据：同一份配置连跑两次，第二次的日志出现"跳过"
且远程文件的 mtime 不变。**不能只断言日志** —— 日志说跳过而实际重传了，用例照样绿；
mtime 才是"真的没传"的证据。

### 2. SC-F03：sshKey 波浪号展开（分发与执行两条路径一致）

新建 `test/remote/sshkey_test.go`。配置里把 `sshKey` 写成 `~/.ssh/id_rsa`，
断言执行（`RunCommandOnNode`）与分发（`CopyToRemote`）两条路径都能连上。
两条都测是关键：M0 的 D13 修的就是"执行认 `~`、分发不认"这种一半对一半错。

### 3. SC-C01 的 multinode 半边：在真节点上装 docker

三节点 fixture 加 `privileged: true`，用例在 node-a 上真装一次 docker、指定版本、再卸载。

**已知边界**：容器里没有 systemd，`systemctl start docker` 不成立。因此本条断言收窄为
"二进制到位、版本是指定的那个、卸载后消失"，**不断言守护进程真的起来**。
这个边界写进 matrix 的注释里；如果收窄后仍不足以称为兑现，则本条保持 `pending`
（宁可留着，也不要一个含义被悄悄改小的 `done`）。

### 4. SC-C02 的 matrix 半边：arm64 上不 404

`ci.yml` 的功能验证已经覆盖 amd64 与 arm64 两种机器。缺的是"真的去 GitHub 要一次"——
现有 local 用例用假服务器，验的是"URL 里的架构名对不对"，验不了"这个 URL 真实存在"。
补一条只在 CI 上跑（`CI=true` 时才执行，本机跳过）的用例：对当前架构的 compose 发布地址
发一次真实请求，断言不是 404。跑在两种机器上，等于同时覆盖 amd64 与 arm64。

*风险*：依赖外网与 GitHub 限流。缓解：只发 HEAD 不下载正文；失败重试一次；
仅此一条用例依赖外网，红了也能一眼看出是网络而非代码。

### 5. SC-M02 的 matrix 半边：真实发行版上真装一次包

`integration.yml` 新增作业，矩阵为 `ubuntu:24.04` / `debian:12` / `fedora:41` /
`alpine:3.20` / `opensuse/leap:15.6`，在每个容器里用 `method: package` 真装一次 `jq`，
装完 `jq --version` 必须能跑。容器内是 root，因此本作业不涉及 sudo。

### 6. `--sudo` 开关（已确认的设计决定）

`method: package` 与需要写 `/usr/local/bin` 的场合都要管理员权限，而生成的命令不带 sudo，
普通用户直接失败。决定：**加 `--sudo`，默认不开**。

- 默认行为与现在完全一致（不加 sudo）→ 不是破坏性变更
- 加了 `--sudo` 则在生成的命令前缀 `sudo `（前缀加在生成的 shell 里，与包管理器探测同理：
  提权必须发生在目标节点上，不能用操作机的身份去判断）
- 权限失败时的报错补一句提示："需要管理员权限，请加 `--sudo` 或用 root 运行"

*不选自动判断的理由*：非 root 就自动加 sudo，等于用户没要求提权却被提权了，
且远程节点上的行为更难预料。显式开关让"要不要提权"这件事永远是用户说的。

## 版本

`0.3.1-alpha`。本提案无破坏性变更（`--sudo` 默认关、其余全是用例与 CI），
按项目约定 alpha 阶段 BREAKING 才动 Y，故走 Z 自增。

## 回滚

- 用例与 CI 作业：删掉对应文件 / 作业即可，不影响产品行为
- `--sudo`：`git revert` 对应提交，默认关意味着回滚不改变任何既有行为

## 执行记录（与方案的偏差）

### 偏差 1：SC-D10 从 remote 组挪到 multinode 组

方案 §1 把它放在 `test/remote/dispatch_test.go`，落地时改到 `test/multinode/dispatch_test.go`。

理由：remote 组的"远端"是回环别名 `127.0.0.2`，与操作机是**同一个文件系统**。
源文件和目标文件是同一个文件，hash 永远相等 —— 用例必绿，且什么都没验。
只有三节点这样文件系统真正分离，"传过去了"与"没再传"才分得开。
matrix 的 `env` 与 `files` 已同步改。

### 偏差 2：SC-C01 的 multinode 半边不兑现，保持 pending

方案 §3 预留了"收窄后仍不足则保持 pending"，此处按该条执行：**不加 `privileged: true`，
不写 `test/multinode/docker_test.go`**。

证据：`somcli docker install` 是调外部 `docker-manager.sh`，该脚本的 `install_docker_main`
把步骤写成必经列表，其中 `start_service` 会 `systemctl daemon-reload / enable / start`，
任一步返回非零就走 `handle_install_failure` → `exit 1`。sshd 容器里没有 PID 1 的 systemd，
`privileged: true` 也不会凭空长出一个 —— 装到那一步必然失败，收窄后的那三条断言压根到不了。

要真正兑现，得先把 docker 安装从 shell 脚本收进 Go（阶段 2 之后的事）。这个原因已写进
matrix 里 SC-C01 的注释。

### 偏差 3：`--sudo` 只作用于 `method: package`

方案 §6 的开头提到"需要写 `/usr/local/bin` 的场合"也要权限，落地时**只**接到
`method: package` 上（与「影响」一节声明的两个文件一致）：`method: binary` 的安装目录由
`install_dir` 决定，用户可以指向自己有权限的目录，不存在"必须提权"；把 sudo 铺过去
等于多出一条没人走过的分支。真有需要时另开提案。

## 验证

- [x] `test/matrix.yaml` 中 phase 1 的场景：SC-C01 明确保持 `pending`（理由见偏差 2），
      其余 4 条待 CI 绿后置 `done`
- [x] 新增用例都在补齐前的提交上跑过并确认变红（`--sudo` 两条在 worktree 的 `f1be310` 上
      确认 `unknown flag: --sudo` / 提示缺失；SC-C02 那条用故意改错架构映射确认变红）。
      `TestSC_M02_SudoOffByDefault` 是兼容性守卫，不算证伪
- [ ] CI 与 Integration 两条流水线全绿，**本机自检一律带 `-race`**（M1 的教训）
- [x] `--sudo` 有用例：不加时命令无 sudo 前缀、加了才有；默认行为无变化
- [x] changelog 补 `0.3.1-alpha` 条目
