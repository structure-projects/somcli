# Trust-Level 与 HITL 三级模型

> 完整规范：trust-level 三级 × HITL 三级的行为矩阵、autonomous 阻塞转人工机制、audit 审计闭环。

## 1. 术语定义

| 术语 | 定义 |
|---|---|
| **trust-level** | 项目级信任级别，分 strict/standard/autonomous，由 `changes/config.yaml` 配置 |
| **HITL** | Human-in-the-Loop，技能 frontmatter 中声明的人工确认节点 |
| **L0 / L1 / L2** | HITL 三级分类，定义节点本身的"可逆/不可逆"本质属性 |
| **audit.md** | autonomous 模式下人工跳过 L1 项的审计与复核日志 |
| **blocked-pending-human** | proposal status 字段枚举值之一，L0 阻塞时写入 |
| **人工队列** | 由所有 proposal.status==blocked-pending-human 组成的待人工介入列表 |

## 2. HITL 三级判定准则

### 2.1 核心判据

给技能 HITL 项定级时，以下两个问题中任何一个答案为 "No" 即 **L0**；都为 "Yes" 且能在 1 小时内恢复原状即 **L1**；无需恢复路径（本地读/写）即 **L2**。

| 判据 | L0 | L1 | L2 |
|---|---|---|---|
| 操作**结果**是否可逆？ | ❌ 不可逆 | ✅ 可逆 | ✅ 无状态变化 |
| 是否影响**生产环境/主分支/资金安全/合规**？ | ✅ 影响 | ❌ 不影响 | ❌ 不影响 |
| 恢复原状需要多久？ | N/A | ≤ 1 小时 | N/A |

### 2.2 典型场景定级对照

| 场景 | Level | 原因 |
|---|---|---|
| 生产部署（k8s apply / helm upgrade 到 production） | L0 | 不可逆 + 生产流量 |
| 生产回滚（helm rollback / kubectl rollout undo） | L0 | 影响生产流量 |
| `git push origin master` | L0 | 生产主干，不可逆 |
| `git push --force origin public-branch` | L0 | 已发布 commit 不可变 |
| hotfix 全量（100% 流量） | L0 | 全量不可逆 |
| `gh release create` 打 tag 发布 | L0 | 发布不可逆 |
| expert-review MUST fix 未解决就提交 | L0 | 质量红线 |
| 违反 stack-constraints.forbidden 的决策 | L0 | 架构红线 |
| terraform apply / destroy 生产环境 | L0 | 基础设施不可逆 |
| 资金/支付/安全关键字段变更 | L0 | 合规 |
| production DB DDL（ALTER/DROP TABLE） | L0 | 不可逆 |
| --- | --- | --- |
| 项目形态选择（DDD/单体） | L1 | 可重做 proposal |
| `git push origin develop` | L1 | 集成分支，可 revert / 重置 |
| 需求澄清补充 | L1 | 可补 proposal.assumptions |
| proposal 确认进入编码 | L1 | coding 不符可回退 |
| 关键设计决策（非红线） | L1 | 可回退方案 |
| 关键项目 AI 自评审 | L1 | 24h 人工复核 |
| commit 措辞澄清（保存 vs commit） | L1 | git reset |
| 检查失败降级走 hotfix 通道 | L1 | 可切回标准通道 |
| deploy staging/dev（assist→自动） | L1 | 非生产 |
| hotfix 灰度 ≤10% 流量 | L1 | 15min 观察+回滚 |
| 分支命名 infer | L2 | 无状态影响 |
| commit message infer | L2 | git amend |
| AI 评审打 "AI 自检" 标 | L2 | 本地打标 |
| 回 requirement-analysis 补 proposal | L2 | 本地操作 |
| commit/push 到 feat-* | L2 | feat-* 无保护 |
| 本地跑单测 / shellcheck | L2 | 本地 |

## 3. trust-level × HITL 行为矩阵

| | L0 never-skip | L1 auto-decidable | L2 always-auto |
|---|---|---|---|
| **strict** | MUST 问用户 | MUST 问用户 | 自动执行 |
| **standard** | MUST 问用户 | MUST 问用户 | 自动执行 |
| **autonomous** | **阻塞 + 转人工队列 + 告警** | **自动执行 + audit.md + 24h 复核** | 自动执行 |

### 3.1 旧字符串 HITL 兼容性

| trust-level | 旧字符串 HITL 行为 |
|---|---|
| strict/standard | 视作 L1，MUST 问用户（行为不退化） |
| autonomous | **阻塞 + 提示技能升级为结构化格式**，不得自行推断（保障 autonomous 不越权） |

## 4. autonomous 运行机制

### 4.1 L0 阻塞流程（blocked-pending-human）

```
AI 进入 skill
   ↓
识别 HITL 项为 L0 + trust-level=autonomous
   ↓
1. proposal.status = blocked-pending-human
2. audit.md 追加条目（id: l0-blocked / 阻塞原因 / 升级时间）
3. 触发外部告警（实现由 config.yaml 的 alert-channels 配置，预留字段）
4. 作业暂停但不退出，等待：
     a. 人工介入解除阻塞（status 改回对应阶段）
     b. 超时升级（默认 24h 未处理 → 告警升级 + 终止作业）
```

### 4.2 L1 自动执行流程

```
AI 进入 skill
   ↓
识别 HITL 项为 L1 + trust-level=autonomous
   ↓
读取 autonomous.behavior：
  ├─ auto-pass：无动作，直接进入下一步
  └─ ai-infer：AI 推断决策依据 → 写入 proposal.assumptions 或 design.decisions
   ↓
audit.md 追加条目：
  时间 / 跳过项 ID / 所属 skill / 原始 HITL / AI 决策依据 / 风险等级 / 人工复核截止(+24h) / 复核状态=pending
   ↓
进入下一步
```

### 4.3 audit 复核闭环

**触发点**：ci-gate 执行前（任何 commit/push 前）

**检查逻辑**：
1. 读取当前 proposal 的 audit.md
2. 过滤 `复核状态 == pending` 且 `复核截止时间 < now` 条目
3. 无 → 正常放行
4. 有 → **升级为 L0 阻塞**（proposal.status = blocked-pending-human，审计日志写"超时未复核升级"）

**复核操作**（人工）：
- 将 audit.md 对应行的 `复核状态` 改为 `approved` 或 `rejected`，`复核人` 填姓名/ID
- 若 rejected：相应回滚 L1 决策产生的变更（如回退 proposal / design 决策），status 重置到对应阶段

## 5. 仲裁与优先级

```
最高优先级
   ↑
1. 技能结构化 HITL.level（AI 机器决策唯一来源）
2. trust-level × HITL 矩阵（决定 L1 行为）
3. 规则文档（L0/L1/L2 示例约束）
4. 技能旧 mode 字段（deprecated，strict/standard 可读取；autonomous 下忽略）
5. AI 自行推断（❌ 禁止）
   ↓
最低优先级
```

**不得**：AI 根据技能的 `Description`/`Details` 文本自行推断 HITL 等级；必须读结构化 `level` 字段。

## 6. 配置扩展（changes/config.yaml）

```yaml
# 核心字段
trust-level: standard        # strict | standard | autonomous
current-proposal: ""         # 当前活跃提案 ID（供编排器定位）
default-project-form: ""     # 项目形态 fallback：DDD-7-plus-1 | single-4-modules | single-module

# 可选扩展（本提案不强制实现，保留字段名供后续）
alert-channels:              # L0 告警通道（预留）
  - type: webhook
    url: https://im.example.com/xxx
  - type: email
    to: sre@example.com
blocked-timeout-hours: 24    # L0 阻塞超时升级阈值（默认 24h）
l1-review-window-hours: 24   # L1 复核窗口期（默认 24h）
extra-l0-items:              # 项目级额外 L0 项（L1 升级到 L0）
  - confirm-prod-deploy
extra-l2-items:              # 项目级额外 L2 项（L1 降级到 L2）
  - commit-msg
```

## 7. 自检清单（给 skill 作者）

为每个 HITL 项打标前回答：

1. [ ] 该操作是否影响主分支/生产/资金/合规？→ 是则 L0
2. [ ] 执行错误后 1 小时内能否恢复原状？→ 不能则 L0
3. [ ] 能否给出明确回滚步骤？→ 不能则 L0
4. [ ] autonomous 自动执行是否可接受？→ 不能则 L0
5. [ ] AI 推断决策能否给出明确依据？→ 不能则 L0（不要依赖黑盒推断）
6. [ ] 旧字符串 HITL 是否已迁移为结构化？→ 未迁移则 autonomous 阻塞提示

## 8. 异常处理

| 异常 | strict/standard | autonomous |
|---|---|---|
| skill HITL 为空 | 正常（无确认项） | 正常（无确认项） |
| 字符串型旧 HITL | 视作 L1 问用户 | 阻塞 + 提示升级 |
| 级别的枚举拼写错误（如 L0-never-skip 写成 l0） | 阻塞报 schema 错 | 阻塞报 schema 错 |
| L0 的 autonomous.escalate 缺失 | schema 校验阶段拦截 | schema 校验阶段拦截 |
| audit.md 不存在 | 不创建（无 L1 跳过） | 首次 L1 跳过时自动创建 |
| 多个 proposal 的 current-proposal 指针冲突 | 报错问用户 | 阻塞 + 转人工（并发控制留后续提案） |

## 9. 完成证据

任何"完成/勾选"声明 MUST 附带可核验的命令输出，禁止仅凭对话推断完成。这条约束在三个 trust-level 下**一律生效**——autonomous 不豁免证据要求，因为无人值守时证据是唯一的事后审计依据。

| 场景 | 必须附的证据 |
|---|---|
| 编码完成 | `mvn test` / `npm test` / `pytest` 输出（退出码 + 通过数） |
| 提交完成 | `git log --oneline -1` + `git status --short` 输出 |
| 测试通过 | 测试摘要（`N passed, 0 failed`） |
| 部署完成 | 健康检查返回（状态码 + 响应） |
| 归档完成 | `ls changes/archive/<name>/` + changelog 条目 |

- ❌ MUST NOT 仅凭"已勾选 tasks.md"或对话宣称完成
- ❌ MUST NOT 在命令未执行时写"完成"（含"已完成""执行完成""验证通过"等措辞）
- ✅ 无法执行（无环境 / 无权限 / 无网络）→ MUST 明确说明"未执行，原因 X"，不得谎报完成
- ✅ 勾选 `tasks.md` 前 MUST 先有对应命令输出作为证据

## 10. 宿主工具权限门禁

AI 的 HITL 分级（L0/L1/L2）只能**叠加**在宿主工具权限层之上，MUST NOT 用来**绕过**宿主权限层。

| 层 | 归属 | 作用 |
|---|---|---|
| 宿主权限层 | Claude / Cursor / Trae / CodeBuddy / Qoder 的 `settings.json` 等 | 最终门禁，决定命令能否执行 |
| HITL 分级 | 本仓库规则与技能 | 在宿主允许的前提下，决定要不要先问用户 |

- 宿主工具对危险命令的"是否执行"提示是**最终门禁**，AI MUST NOT 在提示前抢先执行
- 规则里写"自动/不问"（L2）**≠** 能绕过宿主权限；宿主仍会询问时，AI MUST 停下来等确认
- 任何 push / deploy / merge 等对外操作，MUST NOT 在确认前抢先执行
- 反之亦然：宿主 allow 了某命令**不代表** HITL 允许自动执行——两层是 AND 关系，取更严的一方

具体到各工具的配置文件与语法差异，见 `wiki/_common/permissions-mapping.md`。

