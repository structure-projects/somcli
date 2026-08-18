# Autonomous 审计日志

> 仅在 `trust-level: autonomous` 模式下存在。
> L1（auto-decidable）跳过的 HITL 项和 L0（never-skip）阻塞项 MUST 写入本文件。
> **超过 24h 未复核的 L1 项，下次 ci-gate 时升级为 L0 阻塞**（强制闭环）。
> 复核人填姓名/ID；`复核状态 = rejected` 时 MUST 回滚对应决策并恢复 proposal 到对应阶段。

| 时间 (UTC+8) | 跳过项 ID | 所属 skill | 原始 HITL Level | 原始动作摘要 | AI 决策依据 | 风险等级（低/中/高） | 人工复核截止 | 复核状态（pending/approved/rejected） | 复核人 | 备注 |
|---|---|---|---|---|---|---|---|---|---|---|
| YYYY-MM-DD HH:MM | <hitl-id> | <skill-name> | L0/L1 | <action> | <why decided this way> | 低/中/高 | YYYY-MM-DD HH:MM | pending | - | - |
