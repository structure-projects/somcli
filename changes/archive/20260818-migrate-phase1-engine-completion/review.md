# 评审报告：20260818-migrate-phase1-engine-completion

| 字段 | 值 |
|---|---|
| 评审日期 | 2026-08-20 |
| 评审人 | AI（`gin-reviewer` 子代理初筛 + 主代理逐条核实） |
| 评审范围 | `git diff e100236..HEAD`（M1.1~M1.4，5 个提交，53 文件，+6062） |
| 结论 | ✅ 通过（唯一的 MUST 项已在本轮修复并补了用例） |

> ⚠️ 本报告是 AI 自检，**不等于**人类评审。其中安全项（R1）建议由人复核一遍判据。

## 评审方式说明（为什么这份报告比子代理的原始输出短）

子代理报了 16 条，其中 8 条在同一段里自行撤回（提出后又论证不成立），剩下 8 条我逐条读代码核实：
**2 条证实、4 条驳回、2 条降级为 NIT**。而唯一的 MUST 项（R1）子代理没有发现，
是核实"临时目录"那条时顺着 `header.Name` 读出来的。

结论：子代理适合把注意力引到值得读的文件上，不适合直接抄进报告。

## MUST fix（必须修复）

- [x] **R1 `pkg/images/import.go:71` 归档条目名未校验，可写到解压目录之外（CWE-22 / zip-slip）**

  `tempFile := filepath.Join(tempDir, header.Name)` 直接采信归档内的条目名。
  `header.Name` 写成 `../../../etc/cron.d/x` 时 `filepath.Join` 会把 `..` 规约掉，
  路径就落到 `tempDir` 之外，随后 `os.Create` + `io.Copy` 把**攻击者提供的内容**写进去。

  *为什么是 MUST 而不是 SHOULD*：离线镜像包正是设计上要在机器之间传递的外部输入，
  而 somcli 的典型运行方式是 root（装 `/usr/local/bin`、调包管理器）。
  写入发生在 `docker load` **之前**，所以目标机上有没有 docker 都不影响利用。

  *实测*（修复前，`f07cd5f`）：单条目归档，名字为 40 层 `../` 加绝对路径尾巴，
  `somcli images import -i evil.tar.gz` 在 `/tmp/somcli-zipslip-*/PWNED.txt` 写出了 `PWNED`。

  *修复*：新增 `secureJoin`，判据取 `filepath.Rel` 的结果而不是"名字里有没有 `..`"
  （后者漏 `a/../../b`，又误伤名字里恰好含 `..` 的正常文件）；同时把条目类型收窄为
  `tar.TypeReg` —— 软链接如果收下，后一个条目可以顺着它写到目录外。

  *用例*：`test/local/images_test.go` 的 `TestR1_ImportRejectsPathTraversalEntries`。
  判据是"标记文件有没有被创建"而**不是**退出码：没有 docker 时导入本来就非 0 退出，
  拿退出码当判据会得到一条永远绿的用例。已在 `f07cd5f` 上确认变红（写出了标记文件）。

## SHOULD fix（建议修复）

- [x] **R2 `pkg/images/{import,export}.go` 临时目录落在当前目录，`--workdir` 被忽略**

  `filepath.Join("temp-export")` 就是相对路径 `temp-export`。后果有三：无视 `--workdir`
  （与 D10 同一类病症）、在用户的工作目录里凭空造目录、两个并行的 somcli 互相覆盖中间产物。

  *修复*：抽 `newTempDir`，落到 `<workdir>/tmp` 下并用 `os.MkdirTemp` 取独占名字。

  *不补用例的理由*：`defer os.RemoveAll(tempDir)` 在两种实现下都会清掉目录，
  所以"目录建在哪"在进程外不可观测；要观测得把 CWD 设成只读，而现有夹具
  （`runEnvIn` 只给 env 与 workdir）没有设置 CWD 的入口。为一条 SHOULD 改夹具不值得，
  改生产代码求可测性又违反项目约定，故记录在此。

- [x] **R3 `pkg/images/export.go:48-52` `Close` 错误被 `defer` 吞掉，半截归档报成功**

  真正把剩余压缩块刷进文件的是 `gzip.Writer.Close()`。用 `defer gzipWriter.Close()`
  忽略返回值，等于把"磁盘满"写成"导出成功"，而症状要等到目标机上 `import` 才暴露。

  *修复*：`tar` → `gzip` → `file` 逐层 `Close` 并检查；任何失败都连带 `os.Remove(outputPath)`
  —— 与 F14 已经立下的"有失败就不写镜像清单"是同一条判据：**半截产物比没有产物更危险**。
  连带把 changelog「已知欠账」里"`images export` 失败时残留的半截 `.tar.gz` 不会被清理"这条关掉。

  *不补用例的理由*：同 R2，要构造 `Close` 失败得模拟磁盘满或只读文件系统。

## 驳回（子代理提出，核实后不成立）

- **`pkg/installer/extrafiles.go:91` `flattenPath` 撞名导致内容互相污染** —— 不成立。
  `:` `/` `\` 三者都映射成 `_` 确实会让 `/etc/a:b` 与 `/etc/a/b` 压平成同一个暂存件名，
  但 `writeExtraFiles` 是在**同一轮循环迭代内**完成"写暂存件 → 分发 → `install` 就位"的
  （`extrafiles.go:63-84`），下一轮覆盖暂存件时上一个目标早已落盘。无交叉污染。
  遗留的只是注释过度承诺 → 降级为 NIT-1。

- **`pkg/utils/download.go:172` `io.Copy` 不响应 context 取消** —— 不成立。
  请求是 `http.NewRequestWithContext` 建的，`net/http` 的 ctx 覆盖**响应体读取**，
  超时到点 `resp.Body.Read` 会返回错误，`io.Copy` 随即中断。真实剩余缺口只是
  "Ctrl-C 时留下 `.part`"，而 `.part` 这个命名本身就是为此设计的（`download.go:135` 的注释），
  下一次运行不会把它当缓存命中。不修。

- **`pkg/utils/command.go:333` `fanOut` 每个目标一个 goroutine，只靠信号量限流** —— 事实成立，
  但不是问题：goroutine 初始栈 ~2-4KB，1000 节点约几 MB，且未拿到信号量的 goroutine
  不持有 SSH 连接。改成 worker 池只会让"结果下标与 targets 对齐"这个不变量更难读。降级为 NIT-2。

- **`cmd/docker.go:53` 含 emoji `🔧`** —— 不适用。"不用 emoji"约束的是助手的输出，
  不是代码库既有文案；且该字符早于本提案存在，改它属无关变更。

## NIT（可选）

- [ ] NIT-1 `pkg/installer/extrafiles.go:62` 注释写"避免两个 extra_files 撞名"，
      而实际只是避免了**有害的**撞名（见上）。注释比代码承诺得多，是下一个读者的陷阱。
- [ ] NIT-2 `fanOut` 预先起满 goroutine，见上。
- [ ] NIT-3 `pkg/images/` 内错误信息中英混杂：原有的是英文（`failed to create output file`），
      本轮新加的是中文。整包统一是独立的清理任务，不在本提案范围。

## 逐维度结论

| 维度 | 结论 | 依据 |
|---|---|---|
| **符合性** | ✅ | proposal「验收标准」六条逐条对照：`method` 六种取值、`extra_files`、`uninstall`、状态/幂等/并发/容错、四个子系统缺陷均已落地。**5 个 phase-1 场景仍为 `pending`**，缺的都是"另一半环境"（多发行版容器、docker-in-docker、arm64、真实远程主机），已在 proposal 与 changelog 双向记账，未以 `done` 掩盖 |
| **规范性** | ✅ | `gofmt -l` 空、`go vet ./...` 空；`test/` 无本仓库 import（ci.yml static job 守卫）；提交信息符合 Conventional Commits |
| **测试覆盖** | ✅ 按项目约定 | 黑盒功能验证，不设覆盖率门槛（项目既定约定）。真正的判据是**证伪**：M1.1~M1.4 每条新用例都在修复前的提交上跑过并变红，逐条归因写在 proposal「执行期偏差记账」。已知弱项：SC-X08 的红以"超时"而非断言失败呈现，比其他三组弱一档 |
| **安全性** | ⚠️→✅ | R1 已修。另核实：`shellQuote` 用的是标准 `'\''` 转义，构造不出闭合逃逸；`--set k=v` 的值进模板而不进 shell 拼接。**遗留已知风险**：`StrictHostKeyChecking=no`（首次连接不校验指纹）、`method: package` 不带 sudo 的取舍未决，两条都已在 changelog 声明 |
| **性能** | ✅ | 无 N+1 / 无同步阻塞热点。`--parallel` 的栅栏设计（每条脚本一道）是为等价性刻意付出的代价，非缺陷 |
| **可读性** | ✅ | 一 method 一文件；注释普遍解释"为什么"而非"做什么"（`method.go:85` 为何不用 Go 解压、`command.go:319` 为何要栅栏），符合项目风格 |

## 评审意见

建议合并。

本提案的真实价值不在新增了几个 `method`，而在把一批**零消费者字段**变成了能力 ——
`method`、`extra_files`、`remove_scripts`、`image`、`files` 此前全都写在配置里、印在文档里、
不影响任何行为。这类"配置看起来生效了其实没有"的状态，比缺功能更危险，
因为它让用户和 CI 都拿到了错误的信号；因此几处破坏性变更（`method` 开始分发、
状态参与决策、`images` 失败改非 0 退出）是必要代价，且都给了迁移写法与回滚路径。

评审中最值得记的一件事：**R1 是子代理没报、我在核实一条 SHOULD 时顺带读出来的**，
而子代理报的 16 条里有 8 条自行撤回、4 条经核实不成立。这与本次迁移的主题同构 ——
"我认为它好了"不是信号，跑出来的红与绿才是。所以本报告里的每条 MUST/SHOULD 都注明了
是实测还是读码得出，无法构造黑盒判据的（R2/R3）明确写出理由而不是含糊带过。

尚存的 5 个 `pending` 场景不构成合并阻塞：它们缺的是环境而不是实现，
且都没有被记成 `done` —— `matrix.yaml` 作为可信信号这一点保住了，这比场景数字好看更重要。
