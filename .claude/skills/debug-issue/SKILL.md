---
name: debug-issue
description: |
  当用户报告"报错/异常/不工作/有问题/bug/为什么 X"时触发。
  系统性调试问题：收集信息 → 复现 → 定位 → 修复 → 验证。
  禁止凭直觉猜测。

triggers:
  - 报错
  - 异常
  - 不工作
  - 有问题
  - bug
  - 为什么
  - 怎么回事
  - 排查
  - 调试
  - debug
  - investigate
  - 修复

role: developer
phase: support


allowed-tools: Bash, Read, Write, Edit, Grep, Glob

related-rules:
  - common-observability
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/error-handling.md
  - wiki/_common/logging.md
  - wiki/<stack>/developer.md

produces:
  - 问题根因分析
  - 修复代码
  - 验证报告
  - （可选）changes/proposals/<id>/ 变更提案（如果修复较大）

requires: []

human-in-the-loop:
  - 修复方案 MUST 与用户确认（特别是涉及数据/逻辑变更）
  - 生产环境问题 MUST 用户确认修复

on-failure: |
  无法定位根因 → 列出已排除的可能，请用户提供更多信息
  修复引入新问题 → 回滚 + 重新分析

mode: auto

category: debug
stack: _common
priority: high
---

# 调试问题

> 系统性调试：收集 → 复现 → 定位 → 修复 → 验证。**禁止凭直觉猜测**。

## 两条通道（MUST 先判定走哪条）

| 通道 | 适用前提 | 流程 |
|---|---|---|
| **完整通道**（默认） | 原因不明 / 需要复现定位 | 下方 8 步全走 |
| **快速通道**（`/quick-fix`） | 原因已明确 + 方案清晰 + ≤ 5 文件且 ≤ 50 行 | 见"快速通道"节 |

原因不明 MUST 走完整通道，**MUST NOT** 用快速通道猜。
生产环境热修复两条通道都不适用，MUST 转 `release-ops`。

## 执行步骤

### 第 1 步：收集信息（MUST 完整）

收集以下信息：
- **错误信息**：完整的错误消息、堆栈
- **复现步骤**：如何重现问题
- **环境信息**：哪个环境（本地 / 测试 / 生产）、什么版本
- **最近变更**：最近改了什么代码 / 配置 / 依赖
- **影响范围**：影响哪些用户 / 功能

```bash
# 收集日志
tail -100 logs/application.log

# 收集最近的 git 变更
git log --oneline -10
git diff HEAD~1
```

### 第 2 步：复现问题

在本地 / 测试环境尝试复现：
- 能复现 → 继续第 3 步
- 不能复现 → 回到第 1 步收集更多信息（可能是环境差异）

### 第 3 步：定位根因

按层次排查：

```
1. 输入层：请求参数是否正确？
2. 业务层：业务逻辑是否有 bug？
3. 数据层：SQL 是否正确？数据是否异常？
4. 外部依赖：下游服务是否正常？
5. 配置层：配置是否正确？
6. 环境层：JDK / 框架版本是否兼容？
```

**工具**：
- 日志：`grep` 关键错误
- 调试：打断点（如本地）
- 测试：写单元测试复现
- 监控：看指标异常

### 第 4 步：分析根因

**禁止**：
- 只看表面错误就修
- 没复现就修
- 凭印象修

**必须**：
- 找到真正的根因（不是表象）
- 理解为什么会出现这个问题
- 评估影响范围

### 第 5 步：制定修复方案

**MUST 与用户确认**：
- 修复方式是否合适
- 是否需要数据修复
- 是否需要变更提案（大改动）

小改动（typo / 配置）→ 直接修复
大改动（逻辑变更）→ 走 `requirement-analysis` 创建变更提案

### 第 6 步：修复 + 测试

1. 写单元测试复现问题
2. 修复代码
3. 验证测试通过
4. 验证原问题场景已修复

### 第 7 步：验证

- 本地验证
- 测试环境验证
- （生产问题）生产环境验证 + 监控

### 第 8 步：总结

- 根因是什么
- 为什么没在测试阶段发现
- 如何避免类似问题（如需要，写入 `retrospective.md`）

## 快速通道（原因已明确时）

1. **确认问题**：简要确认问题描述与修复方案
2. **分析影响**：评估修复的影响范围
3. **实施修复**：代码 / 配置 / 文档，MUST 保持现有代码风格
4. **验证修复**：编译通过 + 相关单测通过 + 手动验证场景
5. **提交**：MUST 走 `ci-gate` 快速通道（跳过 archive-change，保留「编译 + 单测」两项门禁），**MUST NOT** 裸 `git commit`

超出适用前提（原因不明 / 范围过大）MUST 立即转完整通道；快速通道门禁失败同样 MUST 转完整通道。

回报格式：

```
🔧 快速修复完成

修复内容：
- src/main/java/.../UserController.java:45
  - 修复：密码加密逻辑缺失 → 调用 BCrypt.hashpw()

验证：
✅ 编译通过  ✅ 相关单测 12/12 通过  ✅ 手动验证：密码加密正常

提交：
[fix(user): 修复密码加密逻辑缺失] 2 files changed, 5 insertions(+), 1 deletion(-)
```

## 常见问题模式

### NPE / 空指针
- 检查参数校验（`@NotNull` / `@Valid`）
- 检查 Optional 使用
- 检查数据库返回 null 字段

### SQL 问题
- 打印实际执行的 SQL（`show-sql: true`）
- 检查索引是否生效（`EXPLAIN`）
- 检查 N+1 查询

### 并发问题
- 检查共享可变状态
- 检查事务边界
- 检查锁使用

### 性能问题
- 看监控（QPS / 延迟 / 内存）
- 看慢查询日志
- 看 GC 日志

## 产出物

- 问题根因分析
- 修复代码 + 测试
- 验证报告
- （可选）变更提案 + 复盘文档

诊断报告格式：

```markdown
## 问题诊断报告

| 字段 | 值 |
|---|---|
| 根因 | UserRepository.findById() 返回 null 未处理 |
| 影响范围 | 所有涉及用户查询的接口 |
| 修复方案 | 在 Service 层添加 Optional 检查 |
| 风险 | 低（小改动） |

## 修复步骤
1. UserServiceImpl.findById() 添加 null 检查
2. 抛出 UserNotFoundException（404）
3. 编写单元测试覆盖 null 场景

## 预防措施
- 统一使用 Optional 处理查询结果
- 添加全局异常处理器
```

## 关联

- Wiki：`wiki/_common/error-handling.md` `wiki/_common/logging.md`
- 后续：大改动 → `requirement-analysis`；小修复 → `coding` → `ci-gate`
