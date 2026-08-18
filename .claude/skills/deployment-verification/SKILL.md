---
name: deployment-verification
description: |
  当用户要求"部署/上线/发版/deploy/release/验证"时触发。
  MUST 执行部署前检查 + 健康检查 + 回滚预案。
  生产操作 MUST 用户确认。

triggers:
  - 部署
  - 上线
  - deploy
  - release
  - 验证
  - 发布

role: devops
phase: deployment


allowed-tools: Bash, Read

related-rules:
  - common-core
  - common-delivery
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/deployment.md
  - wiki/_common/version-management.md
  # 栈级规范（MUST 根据识别的栈动态替换 <stack>）
  - wiki/<stack>/developer.md
  - wiki/<stack>/components.md

produces:
  - 部署验证报告
  - 更新 changes/proposals/<current>/deployment.md
  - 或触发热修复流程

requires:
  - skill: ci-gate
    condition: CI 全部通过
    error: CI 未通过，MUST 先通过 ci-gate

human-in-the-loop:
  - id: confirm-prod-deploy
    action: 生产部署 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（不可逆）
  - id: confirm-rollback
    action: 回滚操作 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（回滚影响生产流量）
  - id: assist-command-only
    action: AI 只生成命令，用户执行（assist 行为）
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: strict/standard MUST 用户执行；autonomous 下 staging/dev 可自动执行，生产仍 L0

  # L2 附加：显式明确非生产部署
  - id: deploy-staging-dev
    action: 用户显式要求 /deploy 测试环境（staging/dev）不问
    level: L2-always-auto
    autonomous:
      behavior: auto-pass
      audit: false
    rollback: 部署撤销

on-failure: |
  健康检查失败 → MUST 回滚 + 记录原因
  关键指标异常 → MUST 回滚 + 触发 hotfix 流程
  部署脚本失败 → 修复后重试；strict/standard 3 次失败问用户；autonomous 阻塞转人工+告警

mode: assist  # deprecated: 已被 HITL 结构化 level + trust-level 矩阵替代

# 栈级硬约束（MUST 遵守）
stack-constraints:
  structure-boot:
    spring-boot-version: "4.0.6"
    jdk: "17+"
    parent: "cn.structured:structure-dependencies:1.4.4"
    required-components:
      - structure-security
      - structure-infra
      - structure-restful-web-starter
    forbidden:
      - "Jackson / Gson"
      - "RestTemplate / WebClient"
      - "Spring Boot 3.x"
  vue3:
    required-components:
      - "@structure-projects/components"
      - "@structure-projects/wujie-subapp"
      - "@structure-projects/gateway-client"
    forbidden:
      - "Vue 2"
  react:
    forbidden:
      - "class 组件（必须函数式 + Hooks）"

category: deployment
stack: _common
priority: high
---

# 部署验证

> 部署到目标环境并验证。生产操作 MUST 用户确认。

## 前置条件（MUST 全部满足）

1. **CI 通过**：ci-gate 全部检查通过
2. **变更提案存在**：`changes/proposals/<current>/proposal.md` 存在
3. **changelog 已更新**：`changelog/<version>.md` 含本次变更条目
4. **版本号已升级**：pom.xml / package.json 版本号符合语义化版本

## 执行步骤

### 第 1 步：部署前检查

```bash
# 确认 changelog
cat changes/changelog/<version>.md

# 确认版本号
grep version pom.xml  # 或 package.json

# 确认数据库迁移脚本（如有）
ls db/migration/

# 确认配置变更（如有）
git diff master...HEAD -- "**/application*.yaml" "**/*.env*"
```

### 第 2 步：执行部署

按部署 Wiki 执行（平台差异）：
- K8s：`kubectl apply -f ...`
- Docker：`docker compose up -d`
- 传统主机：`systemctl restart ...`

**MUST 用户确认生产部署命令后再执行**。

### 第 3 步：健康检查（MUST 全部通过）

| 检查项 | 方式 | 通过标准 |
|---|---|---|
| 服务存活 | HTTP `/health` / TCP 端口 | 200 OK / 端口可达 |
| 关键接口冒烟 | 调用核心 API | 返回正确响应 |
| 日志无 ERROR | `kubectl logs` / `tail -f` | 无新 ERROR |
| 监控指标正常 | Prometheus / Grafana | QPS / 延迟 / 错误率在阈值内 |

具体命令（Spring Boot Actuator 为例，其他栈按等价端点替换）：

```bash
# 服务存活 + 依赖健康
curl http://{host}/actuator/health
curl http://{host}/actuator/health/db
curl http://{host}/actuator/health/diskSpace

# 响应时间
curl -s -o /dev/null -w "%{http_code} %{time_total}s\n" http://{host}/api/health

# 内存
curl http://{host}/actuator/metrics/jvm.memory.used

# 日志
tail -200 logs/application.log | grep -i "error\|exception"
head -100 logs/application.log | grep -i "started\|exception"
```

核心功能逐项验证（按业务场景，MUST 全部勾选）：

- [ ] 登录 → 获取 Token
- [ ] 核心 API 正常响应
- [ ] 数据库读写正常
- [ ] 缓存读写正常
- [ ] 消息队列收发正常

**MUST 与基线对比**：响应时间与基线持平、错误率低于基线、资源使用正常。任一项劣化 MUST 判为不通过。

### 第 4 步：写入部署验证报告

写入 `changes/proposals/<current>/deployment.md`：

```markdown
# 部署验证报告

| 字段 | 值 |
|---|---|
| 部署时间 | YYYY-MM-DD HH:MM |
| 部署环境 | staging / production |
| 版本 | X.Y.Z |
| 部署人 | <user> |

## 健康检查
| 检查项 | 状态 | 响应时间 |
|---|---|---|
| Health | ✅ UP | 12ms |
| Database | ✅ UP | 15ms |
| Disk Space | ✅ UP | — |

## 核心功能
| 场景 | 状态 |
|---|---|
| 用户登录 | ✅ 正常 |
| 创建订单 | ✅ 正常 |

## 性能对比（MUST 与基线逐项对比）
| 指标 | 基线 | 当前 | 状态 |
|---|---|---|---|
| P99 响应 | 200ms | 180ms | ✅ 正常 |
| 错误率 | 0.01% | 0.005% | ✅ 改善 |
| 内存使用 | 60% | 58% | ✅ 正常 |

## 结论
✅ 部署成功 / ❌ 部署失败（原因 + 回滚操作）
```

### 第 5 步：失败时回滚

回滚触发条件：
- 健康检查任一失败
- 关键指标异常
- 用户手动触发

回滚步骤：
1. 回滚到上一个稳定版本
2. 验证回滚后服务正常
3. 在 proposal 中记录失败原因
4. 触发 hotfix 流程（如需要）

## 完成标准

- 部署成功
- 健康检查全部通过
- 部署验证报告写入 proposal
- changelog 已更新

## 下一步

- 成功 → 归档变更提案到 `changes/archive/`
- 失败 → 回滚 + 触发 hotfix 流程

## 关联

- 前置：`ci-gate`
- 后续：归档 / hotfix
- Wiki：`wiki/_common/deployment.md`
- 规则：`common-delivery`
