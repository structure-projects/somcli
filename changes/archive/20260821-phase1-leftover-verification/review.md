# 评审报告：20260821-phase1-leftover-verification

| 字段 | 值 |
|---|---|
| 评审日期 | 2026-08-21 |
| 评审人 | AI（未经人类复核） |
| 评审范围 | `42f3f55` 起本提案的全部变更（相对 `f1be310`） |
| 结论 | ⚠️ 有条件通过 —— 条件是两条流水线绿，且 matrix 的 4 条在绿之前不许置 `done` |

## 符合性

提案的 6 项方案，落地情况：

| 方案项 | 落地 | 说明 |
|---|---|---|
| §1 SC-D10 | ✅ 有偏差 | 挪到 multinode 组，理由已写回提案「偏差 1」 |
| §2 SC-F03 | ✅ | 执行与分发两条路径都断言 |
| §3 SC-C01 | ✅ 按预留条款不兑现 | 提案 §3 本就写了"不足则保持 pending"，此处走的是这一条，不是缩水 |
| §4 SC-C02 | ✅ | 只在 CI 上跑，本机跳过 |
| §5 SC-M02 | ✅ | 五发行版容器作业 |
| §6 `--sudo` | ✅ 范围收窄 | 只接 `method: package`，理由已写回「偏差 3」 |

三处偏差都写回了提案，符合"超出提案的范围先写回提案"。

## MUST fix（必须修复）

- [x] **`tildeKey` 在 CI 上也会静默 skip** —— `test/remote/sshkey_test.go`。
      同文件的 `sshTarget` 遵守本仓库的规矩：CI 里连不上是硬失败，因为"环境不具备"在
      受控的流水线里不成立。而 `tildeKey` 无条件 `t.Skipf`。CI 的私钥是流水线自己
      `ssh-keygen` 到 `~/.ssh/id_rsa` 的，构造不出 `~` 形式只可能是准备步骤坏了 ——
      静默跳过会让一条标成 `done` 的场景实际什么都没跑，正是本提案要杜绝的事。
      **已修**：抽 `giveUp`，`CI` 非空时 `Fatalf`。

## SHOULD fix（建议修复）

- [ ] **SC-D10 的 `stagedExtraPath` 在用例里复算了暂存路径的拼法**
      （`<workdir>/download/<名>/<版本>/extra/<压平的目标路径>`）。产品改了目录布局，
      这条会以"节点上没有暂存件"报错，而不是指出布局变了。
      现状可接受：这个路径是黑盒能观察到的产物约定，不是内部实现；且备选方案（从日志里
      抓路径）会把判据换成"日志说了什么"，比现在更弱。**记为已知取舍，不改。**
- [ ] **SC-C02 那条依赖外网与 GitHub 限流。** 提案已列此风险并给了缓解（只发 HEAD、重试一次、
      非 404 的异常状态码明确报"当作网络异常"）。剩余风险：GitHub 限流时这条会红，
      需要人判断是网络还是代码。**接受，不改。**

## NIT（可选）

- [ ] `distros` 作业里 Go 版本写死 `go1.24.0`，与 `setup-go` 的 `'1.24'` 不是同一处维护。
      五个镜像上都得手装 tarball，没有比写死更省的写法；升 Go 时记得两处一起改。

## 规范性

- `test/` 无 `import github.com/structure-projects/somcli/...`（CI 守卫仍然成立）
- 无覆盖率门槛、无为可测性改动的产品代码：产品侧只有 `--sudo` 一处，是提案要求的功能
- `gofmt -l .` 空；`go vet ./...` 与 `-tags=remote` / `-tags=multinode` 均通过
- 版本 `0.3.1-alpha`：无 BREAKING 故走 Z 自增，符合项目约定

## 安全性

- `--sudo` 是提权开关，默认关，且默认路径生成的命令与此前逐字相同 —— 不引入未经要求的提权
- 包名进 shell 前过 `shellQuote`（沿用既有实现），提示串同样引号化
- 提权提示只在非 root 时出现，不泄漏执行者身份之外的信息
- SC-C02 只对 GitHub 发 HEAD，不下载正文、不写盘

## 测试覆盖（本提案的正事）

诚实的信号分级：

| 用例 | 状态 | 证伪情况 |
|---|---|---|
| `TestSC_M02_SudoPrefixesPackageManager` | 本机绿（`-race`） | 在 `f1be310` 上确认 `unknown flag: --sudo` |
| `TestSC_M02_PermissionFailureHintsSudo` | 本机绿 | 在 `f1be310` 上确认提示缺失 |
| `TestSC_M02_SudoOffByDefault` | 本机绿 | **兼容性守卫，不算证伪** |
| `TestSC_C02_ReleaseAssetExistsForThisArch` | 本机带 `CI=1` 绿 | 故意改错架构映射后变红 |
| `TestSC_D10_...` / `TestSC_F03_...` | 只 vet 过 | **本机跑不了（无 docker、连不上 sshd），未证伪** |
| `distros` 作业 | 未跑 | **未证伪** |

后三行是本提案最大的欠账：它们是否真的验得到东西，只有流水线能回答。
因此 matrix 的 SC-D10 / SC-F03 / SC-C02 / SC-M02 一律仍是 `pending`。

## 评审意见

产品代码只动了一处且默认行为不变，风险集中在"新增的验证到底验不验得到东西"上。
本次评审把唯一一处可能让 `pending` 冒充 `done` 的路径（`tildeKey` 的静默 skip）堵掉了。

建议合并的前提有且只有一条：**Integration 与 CI 两条流水线绿之后**，
再把 4 条 matrix 场景置 `done`；任何一条红就回到编码，不许改判据迁就环境。
SC-C01 保持 `pending` 是本提案的正确结论，不是遗漏。
