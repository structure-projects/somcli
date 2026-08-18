# 需求分析规范

> 本规范定义了 structure-projects 生态下需求分析的分类标准、澄清清单、复杂度分级与验收标准。规则使用 RFC 2119 风格标注强制级别（MUST / SHOULD / MAY）。

## 需求分类

| 类型 | 说明 | 变更级别 | 流程 |
|---|---|---|---|
| 新功能 | 全新业务能力 | major | 完整 SDLC |
| 增强 | 现有功能改进 | minor | 完整 SDLC |
| Bug 修复 | 缺陷修复 | patch/hotfix | 简化流程 |
| 重构 | 技术债务 | minor/major | 完整 SDLC |
| 迁移 | 框架/版本升级 | major | 完整 SDLC + migration |

需求分析阶段 MUST 明确分类，并据此选择对应的变更模板与流程。

## 澄清问题标准清单

MUST 在需求分析阶段回答以下问题，任一未明确则禁止进入设计阶段：

1. 业务目标是什么？解决什么痛点？
2. 涉及哪些模块/服务？
3. 输入输出是什么？
4. 有哪些边界条件/异常场景？
5. 是否涉及数据库变更？（schema / 索引 / 数据迁移）
6. 是否涉及 API 变更？（新增 / 修改 / 废弃）
7. 是否涉及权限变更？（角色 / 接口授权）
8. 是否涉及多租户/数据权限？
9. 性能要求是什么？（QPS / 延迟 / 数据量）
10. 是否有兼容性要求？（老版本 API / 前端）
11. 是否涉及第三方集成？
12. 是否有合规/审计要求？

SHOULD 将澄清结果记录到变更提案的"需求背景"章节。

## 变更复杂度分级标准

| 级别 | 判定条件 | 模板 | 审批 |
|---|---|---|---|
| 简单 | 单文件改动，无 API/DB 变更 | proposal-simple | 自动 |
| 标准 | 多文件改动，有 API 或 DB 变更 | proposal-full | 需评审 |
| 重大 | 跨服务 / 架构变更 / 数据迁移 | proposal-full + design | 需评审 + 审批 |
| 迁移 | 框架升级 / 技术栈切换 | proposal-migration | 需评审 + 审批 + 回滚计划 |
| Hotfix | 线上紧急修复 | proposal-hotfix | 事后补审 |

### 分级流程

- 简单变更：跳过详细设计，直接进入实现
- 标准/重大变更：MUST 输出概要设计 + 详细设计
- 迁移变更：MUST 输出迁移计划 + 回滚预案 + 数据校验方案
- Hotfix：MUST 在修复后 24 小时内补齐提案文档

## 拒绝条件

MUST 拒绝以下需求，并在提案中明确记录拒绝原因：

- 无明确业务目标
- 涉及未授权的破坏性变更
- 无回滚方案的不可逆数据迁移
- 与现有架构严重冲突且无迁移计划
- 安全扫描未通过
- 缺少必要的澄清信息且无法补充

SHOULD 对边界模糊的需求先发起澄清会议，再决定是否拒绝。

## 验收标准模板

每个需求 MUST 在分析阶段定义可验证的验收标准，建议使用以下模板：

```
## 验收标准

### 功能验收
- [ ] <具体可验证的功能条件>
- [ ] <边界场景验证>

### 性能验收
- [ ] QPS: <指标>
- [ ] P99 延迟: <指标>
- [ ] 数据量: <预期规模>

### 安全验收
- [ ] 安全扫描通过（OWASP / SAST / DAST）
- [ ] 权限校验符合预期

### 兼容性验收
- [ ] 老版本 API 兼容
- [ ] 前端版本兼容

### 文档验收
- [ ] API 文档更新
- [ ] 变更日志更新
- [ ] 必要的用户手册更新
```

验收标准 SHOULD 在实现前由需求方与开发共同确认。

## 关联

- 通用规则：`common-project-stack-detection` / `common-delivery`
- 技能：`requirement-analysis` / `api-design` / `high-level-design`
- 变更管理：`changes/templates/` proposal-full / proposal-simple / proposal-hotfix / proposal-migration
- 相关文档：`_common/wiki/documentation.md` / `_common/wiki/api-design.md` / `_common/wiki/architecture.md`
