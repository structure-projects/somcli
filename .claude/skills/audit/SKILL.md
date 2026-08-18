---
name: audit
description: |
  当用户要求"扫描现状/代码审计/项目评估/现状分析"或"安全审计/安全扫描/安全检查"时触发。
  两类审计统一入口：现状审计（老项目接入第一步，产出 audit-report.md）+ 安全审计（依赖 / 代码 / 配置 / 传输 / 认证 / 注入）。
  生产环境扫描 MUST 用户确认。

triggers:
  - 扫描现状
  - 代码审计
  - 项目评估
  - 现状分析
  - audit
  - 安全审计
  - 安全扫描
  - 安全检查
  - security audit

role: reviewer
phase: review
supports-skill: project-intake

allowed-tools: Bash, Read, Grep, Glob

related-rules:
  - common-security
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/legacy-onboarding.md
  - wiki/_common/security.md

produces:
  - changes/proposals/0000-legacy-onboarding/audit-report.md（现状审计）
  - 安全审计报告 + 漏洞清单（按严重度分级）+ 修复建议

requires: []

human-in-the-loop:
  - id: select-audit-scope
    action: 审计类型（现状审计 / 安全审计 / 两者）MUST 按来意确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 范围判断错误只影响报告内容，可重跑，无副作用
  - id: confirm-audit-conclusion
    action: 审计报告结论与不合规点定级 MUST 用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 结论有误可修订报告，不产生代码变更
  - id: confirm-production-scan
    action: 对生产环境执行扫描 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A

on-failure: |
  扫描工具缺失 → 明说"未执行，原因 X"，MUST NOT 用主观判断填充扫描结论
  覆盖率 / 依赖扫描跑不起来 → 记为"未评估"，MUST NOT 记为"通过"
  发现高危漏洞 → MUST 列入报告并定级，MUST NOT 因"暂不影响"而省略

category: review
stack: _common
priority: high
---

# 审计

> 现状审计（老项目接入第一步）+ 安全审计。**生产扫描 MUST 用户确认（L0）**。

## 第 0 步：确定审计类型

| 来意 | 类型 | 本文档章节 |
|---|---|---|
| 老项目要接入本规范 | 现状审计 | 一 |
| 上线前 / 定期安全检查 | 安全审计 | 二 |
| 全面评估 | 两者都做 | 一 + 二 |

---

# 一、现状审计（老项目接入）

## 第 1 步：扫描项目结构

```bash
ls pom.xml package.json go.mod Cargo.toml 2>/dev/null   # 项目类型
tree -L 2 -d                                            # 目录结构

find . -name "*.java" | wc -l
find . -name "*.ts" -o -name "*.vue" | wc -l

git log --oneline | wc -l
git log --since="6 months ago" --oneline | wc -l
```

## 第 2 步：规范符合性检查

| 维度 | 检查项 | 工具 |
|---|---|---|
| **命名** | 类名 / 方法名 / 常量 / 包名 | grep |
| **分支策略** | 当前分支 / 分支列表 | `git branch -a` |
| **Commit 规范** | 最近 20 条 commit message | `git log --oneline -20` |
| **架构分层** | 模块划分 / 依赖方向 | 目录结构 |
| **异常处理** | 是否用 CommonException | grep "throw new" |
| **日志规范** | 是否用 slf4j / 是否含敏感信息 | grep "log\." |
| **API 设计** | 是否 RESTful / 统一响应 | grep "@RestController" |
| **安全** | 见第二部分 | 见第二部分 |

## 第 3 步：测试评估

```bash
mvn test jacoco:report    # Java
npm test -- --coverage    # Node
pytest --cov              # Python

find . -name "*Test.java" | wc -l
find . -name "*.test.ts" | wc -l
```

## 第 4 步：CI/CD 与文档评估

```bash
ls .github/workflows/ .gitlab-ci.yml Jenkinsfile 2>/dev/null
```

文档维度：README 完整度 / 架构文档 / API 文档 / 变更日志。

## 第 5 步：产出 audit-report.md

按 `changes/templates/audit-report.md` 填充：

```markdown
# 现状审计报告：<项目名>

## 项目概览
- 技术栈 / 模块数 / 代码规模 / 提交历史

## 规范符合性评估
| 维度 | 符合度 | 说明 |

## 测试评估
## CI/CD 评估
## 文档评估
## 主要不合规点
## 改造建议（P0 / P1 / P2）
## 迁移建议
```

## 完成标准

- 所有维度都检查过，未评估项明确标注"未评估 + 原因"
- 主要不合规点列出并定级
- 改造建议分优先级

## 下一步

调用 `project-intake` 制定迁移计划。

---

# 二、安全审计

## 1. 依赖漏洞

```bash
mvn dependency-check:check   # Java
npm audit                    # Node
npm audit fix
```

## 2. 代码扫描

```bash
mvn sonar:sonar
semgrep --config=auto .
```

## 3. 配置检查

```bash
# Secrets 是否硬编码
grep -r "password\|secret\|token" --include="*.yaml" --include="*.properties" .

# 是否暴露敏感端点
grep -r "management.endpoints.web.exposure.include" .
```

## 4. 传输安全

- ✅ **MUST** HTTPS
- ✅ **MUST** TLS 1.2+
- ❌ **MUST NOT** 启用 TLS 1.0 / 1.1

## 5. 认证授权

- ✅ **MUST** 密码 BCrypt 存储
- ✅ **MUST** JWT 设过期时间
- ✅ **MUST** 权限注解完整

## 6. 注入防护

- ✅ **MUST** SQL 参数化
- ✅ **MUST** XSS 转义
- ✅ **MUST** CSRF Token

## 完成标准

- 6 个维度全部检查过
- 漏洞清单按严重度分级，每项有修复建议
- 高危项 MUST 全部列出，MUST NOT 省略

## 关联

- 调用方：`project-intake`
- 后续：`project-intake`（现状审计）/ `expert-review`（安全审计）
- 相关：`debug-issue` / `ci-gate`
- Wiki：`wiki/_common/legacy-onboarding.md`、`wiki/_common/security.md`
