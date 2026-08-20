# 任务清单：补齐 phase 1 的 5 条待验证场景

> 与 `proposal.md` 同目录。每完成一项 MUST 立即勾选 + 更新 `test/matrix.yaml` 对应场景的
> `status` 与 `files`。用例一律黑盒（跑二进制）。
> 本机自检一律 `go test ./... -count=1 -race`（与 CI 对齐，M1 的教训）。

## 准备

- [x] M1 已归档，`current-proposal` 切到本提案
- [x] 沿用 `feat-arch-convergence` 分支

## 1. 远程组两条（SC-D10 / SC-F03）

- [x] SC-D10：远程已存在同 hash 文件则跳过传输 —— 落在 `test/multinode/dispatch_test.go`
      （**不是** remote 组，理由见 proposal「偏差 1」：回环别名与操作机同一个文件系统，
      放在 remote 组这条必绿且什么都没验）
      判据取节点上文件 mtime 不变，日志里的"跳过复制"只作辅证
- [x] SC-F03：`sshKey` 写 `~/.ssh/id_rsa` 时，执行与分发两条路径都要认 —— `test/remote/sshkey_test.go`
      两条都断言：D13 修的就是"一条认一条不认"
- [ ] 两条都在补齐前的提交上确认变红（本机既无 docker 也连不上 sshd，只能由 CI 复核）
- [ ] matrix 置 done —— **待 Integration 流水线绿**

## 2. arm64 上不 404（SC-C02 的 matrix 半边）

- [x] 补一条只在 CI 上跑的真实网络用例（本机跳过）：从 somcli 自己发出的请求里还原真实地址再发 HEAD
- [x] 本机带 `CI=1` 跑通；故意改错架构映射确认它会变红
- [ ] 确认它在 amd64 与 arm64 两种 runner 上都绿 —— **待 CI 流水线绿**
- [ ] matrix 置 done

## 3. 真实发行版上真装一次包（SC-M02 的 matrix 半边）

- [x] `integration.yml` 新增 `distros` 作业：ubuntu / debian / fedora / alpine / opensuse
- [x] 每个容器里用 `method: package` 真装 `jq`，判据是装完 `jq --version` 能跑
- [ ] matrix 置 done —— **待 Integration 流水线绿**

## 4. 真节点上装 docker（SC-C01 的 multinode 半边）

- [x] 结论：**保持 pending**，不加 `privileged: true`、不写 `test/multinode/docker_test.go`
- [x] 理由写进 matrix 里 SC-C01 的注释与 proposal「偏差 2」：`docker-manager.sh` 的必经步骤含
      `systemctl start docker`，容器里没有 PID 1 的 systemd，装到那一步必然失败，
      收窄后的三条断言压根到不了

## 5. `--sudo` 开关

- [x] `cmd/install.go` 注册 `--sudo`，默认关（`viper.BindPFlag`）
- [x] 前缀加在**生成的 shell 里**；带环境变量的写成 `sudo env VAR=v cmd`
      （`sudo VAR=v cmd` 不合法，`VAR=v sudo cmd` 会被 sudo 的环境清洗丢掉）
- [x] 权限失败时补提示，身份用 `id -u` 在目标节点上判，并原样传回退出码
- [x] 用例：不加时无 sudo 前缀、加了才有、非 root 失败时有提示
      （其中「不加时无前缀」是兼容性守卫，不算证伪）

## 测试

- [x] `go test ./... -count=1 -race` 全绿
- [x] `-tags=remote`、`-tags=multinode` 编译与 vet 通过；两组本机跑不了（无 docker、连不上 sshd），
      由 Integration 流水线复核
- [x] 新用例在补齐前的提交上能复现失败 —— `--sudo` 两条在 worktree 的 `f1be310` 上确认，
      SC-C02 那条用故意改错架构映射确认；SC-D10 / SC-F03 只能由 CI 复核

## 归档

- [x] changelog 补 `0.3.1-alpha` 条目
- [x] 通过 expert-review（`review.md`：1 条 MUST 已修 —— `tildeKey` 在 CI 上也会静默 skip）
- [ ] 通过 ci-gate（两条流水线全绿）
- [ ] `git mv` 到 `changes/archive/`，`current-proposal` 切到 phase 2
- [ ] 推送需用户确认
