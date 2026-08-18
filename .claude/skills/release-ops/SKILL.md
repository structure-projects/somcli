---
name: release-ops
description: |
  当用户要求"打 tag/发 Release/发布版本/发版"、"发布 npm 包/npm publish"、"发布 Maven 包/maven deploy"、
  "热修复/hotfix/紧急上线/线上 bug 修复"、"回滚/回退/撤销发布"或"数据库迁移 CD/Flyway 部署"时触发。
  发布运维全流程：制品发布（Tag + Release + Maven/npm）→ 热修复快速通道 → 回滚决策与执行 → 数据库迁移 CD。
  版本号、发布执行、全量上线、回滚、生产数据库迁移均不可撤回或影响生产，MUST 用户确认（L0）。

triggers:
  - 打 tag
  - 发 Release
  - 发布版本
  - 发版
  - GitHub Release
  - gh release
  - 发布 npm
  - npm publish
  - 发布组件库
  - 发布 Maven
  - maven deploy
  - Maven Central
  - 热修复
  - hotfix
  - 紧急上线
  - 线上bug
  - 紧急修复
  - emergency fix
  - 回滚
  - 回退
  - 撤销发布
  - release-ops
  - revert
  - 版本回退
  - 数据库迁移
  - Flyway
  - 数据库变更
  - 数据库 CD
  - migration CD
  - 数据库持续部署
  - 数据库部署

role: devops
phase: deployment
supports-skill: deployment-verification

allowed-tools: Bash, Read, Write, Edit

related-rules:
  - common-core
  - common-delivery
  - common-observability
  - common-data
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/version-management.md
  - wiki/_common/github-workflow.md
  - wiki/_common/maven-publish.md
  - wiki/_common/npm-publish.md
  - wiki/_common/git.md
  - wiki/_common/error-handling.md
  - wiki/_common/database-design.md
  - wiki/_common/ci-cd-pipeline.md

produces:
  - Git Tag（v<X.Y.Z>）+ GitHub Release
  - 发布的 Maven 包（Maven Central）/ npm 包（npmjs.com）
  - 更新的 pom.xml / package.json 版本号
  - changes/proposals/<id>/retrospective.md（hotfix 复盘 / 回滚记录）
  - CI 流水线中的数据库迁移阶段 + 迁移回滚策略 + 迁移验证脚本

requires:
  - skill: ci-gate
    condition: CI 通过（hotfix 走快速通道亦 MUST 通过 MUST 项）
    error: CI 未通过，MUST NOT 发布或上线
  - skill: archive-change
    condition: 发布制品（第一章）时 changes/archive/<id>/ 存在
    error: 变更未归档，MUST 先完成 archive-change
  - skill: data-design
    condition: 数据库迁移（第四章）时迁移脚本已编写
    error: 无迁移脚本，MUST 先调用 data-design

human-in-the-loop:
  - id: select-publish-target
    action: 发布目标（GitHub Release / Maven Central / npm / 多个）MUST 按项目类型确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 目标判断错误在发布执行前即可纠正，未产生外部制品
  - id: confirm-release-version
    action: 发布版本号 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（版本号不可重复、不可回退）
  - id: confirm-publish-execute
    action: 执行 git push tag / gh release create / mvn deploy / npm publish MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（公共制品一经发布不可撤回）
  - id: confirm-trigger-release-pipeline
    action: 触发发布流水线 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: hotfix-canary-confirm
    action: hotfix 灰度发布（≤10% 流量）MUST 用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: auto-pass
      audit: true
    rollback: 灰度回滚（15min 观察期异常自动回滚）
  - id: hotfix-full-confirm
    action: hotfix 全量发布 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（全量不可逆，必须人工）
  - id: confirm-rollback-execute
    action: 执行回滚与目标版本选择 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A（回滚本身即最后手段）
  - id: confirm-prod-migration
    action: 执行生产数据库迁移 MUST 用户确认，且 MUST 先完成备份
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: 按备份恢复（mysql < backup.sql），但存在数据窗口丢失风险

on-failure: |
  Tag 冲突 → 版本号已存在，升版本号后重试，MUST NOT 删除已推送的 Tag
  npm 403 → npm token 失效或无权限，MUST NOT 改 scope 绕过
  npm 版本冲突 → npm version patch 升版本号后重试
  Maven 发布失败 → 检查 OSSRH 凭据与 GPG 签名，MUST NOT 跳过 GPG
  Release notes 缺失 → 补 changes/changelog/<version>.md 后重试
  hotfix 灰度指标异常 → 立即按第三章回滚，MUST NOT 强推全量
  回滚后仍不可用 → 判定为数据不兼容，按第四章回滚策略处理数据层
  数据库迁移失败 → 按回滚策略回滚 + 记录失败原因，应用部署 MUST 中止
  迁移脚本版本冲突 → 修复版本号后重试，MUST NOT 修改已发布脚本

category: deployment
stack: _common
priority: critical
---

# 发布运维

> 制品发布 → 热修复快速通道 → 回滚 → 数据库迁移 CD。**SDLC 的最后一段**。
> **版本号、发布执行、全量上线、回滚、生产迁移均为 L0**：公共制品与生产状态不可撤回。

按任务选章：正常发版 → 一；线上紧急 bug → 二；发布失败 → 三；带 DDL 的发布 → 四。

## 前置条件

- 第一章（发布）：变更已归档（`archive-change`）+ CI 通过（`ci-gate`）
- 第二章（hotfix）：线上确认存在紧急 bug（影响用户 / 资金 / 安全），且用户已确认走 hotfix 通道
- 第三章（回滚）：健康检查失败 / 关键指标异常 / 用户手动触发
- 第四章（迁移）：迁移脚本已编写（`data-design`）

---

# 一、制品发布（Tag / Release / Maven / npm）

## 第 0 步：确定发布目标

```
Q1: 发布目标？（可多选）
    a) 仅 GitHub Release + Tag
    b) Maven Central（Java 库）
    c) npmjs.com（@structure-projects 组件库）

Q2: 版本号？（L0，MUST 用户确认）
    从 pom.xml <revision> / package.json version 读取当前值
```

## 跨目标通用红线（MUST 遵守）

- ✅ **MUST** 版本号符合 `X.Y.Z` 3 段式语义化版本，不可重复、不可回退
- ✅ **MUST** 用附注 Tag（`git tag -a`），格式 `v<X.Y.Z>`
- ✅ **MUST** 在 `master` 上打 Release Tag
- ✅ **MUST** Release notes 从 `changes/changelog/<version>.md` 生成
- ✅ **MUST** 凭据走平台密钥管理
- ❌ **MUST NOT** 用轻量 Tag（`git tag` 无 `-a`）
- ❌ **MUST NOT** 在 `develop` 上打 Release Tag
- ❌ **MUST NOT** 跳过 `archive-change` 直接发布
- ❌ **MUST NOT** 删除或覆盖已推送的 Tag / 已发布的制品

## 1.1 Git Tag + GitHub Release

### 第 1 步：确认版本与分支

```bash
grep -m1 "<revision>" pom.xml     # Java
jq -r .version package.json       # Node

git checkout master && git pull
```

### 第 2 步：打 Tag 并推送（L0，MUST 确认）

```bash
git tag -a v1.2.0 -m "Release v1.2.0"
git push origin v1.2.0
```

### 第 3 步：创建 Release（L0，MUST 确认）

```bash
# 方式 A：从 changelog 生成 notes（推荐）
gh release create v1.2.0 --title "v1.2.0" \
  --notes-file changes/changelog/1.2.0.md

# 方式 B：自动生成 notes
gh release create v1.2.0 --generate-notes
```

### 第 4 步：（可选）触发发布流水线（L0，MUST 确认）

```bash
gh workflow run release-maven.yml  -f module=<module>       -f version=1.2.0
gh workflow run publish-npm.yml    -f component=<component> -f version=1.2.0
gh workflow run build-and-push.yml -f module=<module>       -f version=1.2.0
```

### 第 5 步：验证

```bash
gh release view v1.2.0
gh run list --workflow=release-maven.yml
```

## 1.2 Maven Central（Sonatype OSSRH）

### 前置配置（pom.xml）

```xml
<distributionManagement>
  <snapshotRepository>
    <id>oss</id>
    <url>https://central.sonatype.com/repository/maven-snapshots/</url>
  </snapshotRepository>
  <repository>
    <id>oss</id>
    <url>https://central.sonatype.com/service/local/staging/deploy/maven2/</url>
  </repository>
</distributionManagement>

<build>
  <plugins>
    <plugin>
      <groupId>org.sonatype.plugins</groupId>
      <artifactId>nexus-staging-maven-plugin</artifactId>
      <version>1.6.13</version>
    </plugin>
    <plugin>
      <groupId>org.apache.maven.plugins</groupId>
      <artifactId>maven-gpg-plugin</artifactId>
      <version>1.5</version>
      <executions>
        <execution>
          <phase>verify</phase>
          <goals><goal>sign</goal></goals>
        </execution>
      </executions>
    </plugin>
  </plugins>
</build>
```

### 执行

```bash
# 1. 本地构建 + 测试
mvn clean install && mvn clean test

# 2. 发布（L0，MUST 确认）——release,oss 双 profile + 属性化版本
mvn clean deploy -P release,oss -Drevision=1.2.0

# 3. 验证
mvn dependency:get -DartifactId=cn.structured:structure-infra:1.2.0
# 并在 https://central.sonatype.com/ 确认已同步
```

### 关键约束

- ✅ **MUST** 用 `release,oss` 双 profile
- ✅ **MUST** 用 `-Drevision=` 属性化版本
- ✅ **MUST** GPG 签名
- ❌ **MUST NOT** 在 pom.xml 硬编码 Secrets
- ❌ **MUST NOT** 跳过 GPG 签名

## 1.3 npm（npmjs.com）

### 发布资格判定（MUST 先做）

| 包类型 | 判据 | 是否可发布 |
|---|---|---|
| 组件库 | `@structure-projects/*` scope | ✅ 可发布 |
| 业务包 | `*-ui` 等无 scope 名 | ❌ MUST `private: true`，MUST NOT 发布 |

```json
{
  "name": "@structure-projects/components",
  "version": "1.2.0",
  "private": false,
  "publishConfig": { "access": "public" },
  "files": ["dist"],
  "scripts": {
    "prepublishOnly": "npm run build && npm run test"
  }
}
```

### 执行

```bash
# 1. 资格校验：name 含 scope、private != true、access = public
jq '.name, .version, .private, .publishConfig' package.json

# 2. 升版本
npm version patch     # 修复；minor 新功能；major 破坏性
npm version 1.2.0 --no-git-tag-version   # 或指定

# 3. 构建 + 测试
npm ci && npm run build && npm run test

# 4. 干跑预览 → 发布（L0，MUST 确认）
npm publish --dry-run
npm publish --access public

# 5. 验证
npm view @structure-projects/components
npm install @structure-projects/components@1.2.0
```

### 关键约束

- ✅ **MUST** 组件库用 `@structure-projects` scope
- ✅ **MUST** 组件库 `private: false` + `publishConfig.access: "public"`
- ❌ **MUST NOT** 业务包（`*-ui`）发布到 npm

### 常见问题

| 现象 | 原因 | 修复 |
|---|---|---|
| 403 Forbidden | token 失效 / 无权限 | `npm login` 或换 token；MUST NOT 改 scope 绕过 |
| 版本冲突 | 版本号已存在 | `npm version patch` 升版本 |
| 包名错误 | scope 不对 | 确认 `name` 含 `@structure-projects/` 前缀 |

---

# 二、热修复快速通道（Hotfix）

> 线上紧急 bug 快速修复上线。**跳过完整 SDLC，但 MUST 保留质量门禁**。
> 6 步：分支 → 修复 → 快速 CI → 灰度 → 全量 → 复盘。

## 第 1 步：创建 hotfix 分支

```bash
# MUST 从 master 拉分支
git checkout master
git pull origin master
git checkout -b hotfix-{version}-{brief}

# 例：hotfix-1.2.1-login-crash
```

- ✅ **MUST** 从 `master` 拉分支（生产代码基线）
- ✅ **MUST** 分支名为 `hotfix-{version}-{brief}`
- ❌ **MUST NOT** 从 `develop` 拉 hotfix 分支（可能含未发布功能）

## 第 2 步：最小修复 + 单测

```bash
# 仅修复必要代码
npm test -- --grep "{修复点}"
# 或 mvn test -Dtest={X}Test
```

- ✅ **MUST** 最小化改动范围，仅修复该 bug
- ✅ **MUST** 补充 / 更新单测覆盖修复点
- ❌ **MUST NOT** 顺手重构或改无关代码

## 第 3 步：快速 CI（`ci-gate` 缩减版）

```bash
# MUST 检查（不可跳过）
git branch --show-current | grep -E "^hotfix-"
npm run build  # 或 mvn clean package -DskipTests
npm test       # 或 mvn test（核心单测）
npm audit      # 或 mvn org.owasp:dependency-check:check —— 安全扫描不可跳过
```

- ✅ **MUST** 编译 + 核心单测 + 安全扫描全过（任何情况不可跳过）
- ✅ **MUST** 事后 24h 内补跑完整 CI
- SHOULD 降级项：覆盖率检查、全量测试

## 第 4 步：灰度发布（`deployment-verification` 灰度模式）

```bash
# 先灰度 10% 流量
kubectl rollout canary --percentage=10
# 或 docker tag + 部分节点更新
```

灰度验收：健康检查通过 + 关键指标（错误率 / 延迟 / QPS）无异常 + 观察期 ≥ 15 分钟。

**MUST 用户确认后才进入全量。**

## 第 5 步：全量发布（L0，MUST 确认）

```bash
kubectl rollout deployment {x} --percentage=100
# 或 docker compose up -d
```

全量验收：健康检查全过 + 日志无新 ERROR + 监控指标正常。

## 第 6 步：事后复盘（retrospective.md）

写入 `changes/proposals/<id>/retrospective.md`：

```markdown
# Hotfix 事后复盘

| 字段 | 值 |
|---|---|
| Hotfix ID | hotfix-{version}-{brief} |
| 触发时间 | YYYY-MM-DD HH:MM |
| 影响范围 | <受影响用户 / 业务> |
| 修复版本 | X.Y.Z |
| 修复人 | <user> |
| 上线时间 | YYYY-MM-DD HH:MM |

## 根因分析

<bug 根本原因，5 Why 分析>

## 修复方案

<修复内容说明>

## 预防措施

- <措施 1：如增加监控告警>
- <措施 2：如补充单测>
- <措施 3：如改进流程>

## 改进项

- [ ] 24h 内补跑完整 CI
- [ ] 补充回归测试用例
- [ ] 更新故障应急预案
```

## hotfix 完成标准

- hotfix 分支已合并回 `master`（MUST）与 `develop`（MUST）
- 全量发布成功，健康检查通过
- `retrospective.md` 已写入，改进项已记录

---

# 三、回滚（Rollback）

> 部署失败 / 异常时按决策树执行回滚。**回滚与目标版本选择均为 L0，MUST 用户确认**。

## 第 1 步：确认回滚触发条件

| 触发条件 | 来源 | 判断 |
|---|---|---|
| 健康检查失败 | `deployment-verification` 报告 | 服务不可用 |
| 关键指标异常 | Prometheus / Grafana 告警 | 错误率 > 阈值 / 延迟 > 阈值 |
| 用户手动触发 | 用户指令 | 用户判断需回滚 |

## 第 2 步：回滚决策树

```
触发回滚
   │
   ├─ 能否回滚到上一版本？
   │     │
   │     ├─ 是 → 版本回退（推荐）
   │     │     │
   │     │     └─ 数据库是否兼容？
   │     │           ├─ 是 → 直接回退版本
   │     │           └─ 否 → 需同时数据回滚（见第四章回滚策略）
   │     │
   │     └─ 否（无可回滚版本）→ 数据回滚
   │           │
   │           └─ 执行数据库迁移脚本回滚（第四章）
   │
   └─ 是否需要 hotfix？
         ├─ 是 → 回滚后走第二章
         └─ 否 → 回滚后归档
```

## 第 3 步：执行回滚

```bash
# 3.1 确认当前版本
git describe --tags  # 或 kubectl rollout status
docker ps --format "{{.Image}}"

# 3.2 列出最近稳定版本（MUST 用户确认目标版本）
git tag --sort=-version:refname | head -5
cat changes/changelog/*.md | head -50

# 3.3 版本回退（按部署平台）
# K8s
kubectl rollout undo deployment/{x} --to-revision={N}
# Docker
docker pull {image}:{target-version} && docker compose up -d
# 传统主机
systemctl stop {service} && systemctl start {service}   # 中间替换二进制 / 包
```

### 3.4 验证回滚

| 检查项 | 通过标准 |
|---|---|
| 服务存活 | HTTP `/health` 200 OK |
| 关键接口 | 返回正确响应 |
| 日志 | 无新 ERROR |
| 监控指标 | 恢复正常阈值 |

## 第 4 步：回滚后处理

写入 `changes/proposals/<id>/retrospective.md`（或 `changes/changelog/rollback-{version}.md`）：

```markdown
# 回滚记录

| 字段 | 值 |
|---|---|
| 回滚时间 | YYYY-MM-DD HH:MM |
| 失败版本 | X.Y.Z |
| 目标版本 | X.Y.(Z-1) |
| 回滚原因 | <健康检查失败 / 指标异常 / ...> |
| 回滚人 | <user> |

## 回滚过程
<步骤记录>

## 影响评估
<受影响用户 / 业务 / 时长>

## 后续动作
- [ ] 触发 hotfix（如需要）
- [ ] 根因分析
- [ ] 补充测试用例
```

并通知：开发团队（回滚原因）+ 业务方（影响范围）。如需修复 → 走第二章。

---

# 四、数据库迁移 CD

> 在 CI/CD 流水线中安全执行数据库迁移（Flyway）。**生产迁移 MUST 用户确认 + 备份**。

## 核心原则

- ✅ **MUST** 迁移脚本与应用代码同 PR 提交（保持版本一致）
- ✅ **MUST** 迁移在应用部署**之前**执行（先 DB 后 App）
- ✅ **MUST** 迁移前备份生产数据库
- ✅ **MUST** 迁移失败时应用部署中止
- ❌ **MUST NOT** 在生产环境跳过备份直接迁移

## 4.1 在 CI 流水线中的位置

```
代码提交 → 单测 → 打包 → 镜像构建
                              ↓
                       DB 迁移（测试）→ 应用部署（测试）
                              ↓
                       测试验证 → 生产审批（人工）
                              ↓
                       DB 备份（生产）→ DB 迁移（生产）
                              ↓
                       应用部署（生产）→ 健康检查
```

### GitHub Actions 集成

```yaml
# .github/workflows/deploy.yml
jobs:
  migrate-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Flyway migrate (test)
        run: |
          mvn flyway:migrate \
            -Dflyway.url=jdbc:mysql://test-mysql:3306/mydb \
            -Dflyway.user=${{ secrets.DB_USER }} \
            -Dflyway.password=${{ secrets.DB_PASSWORD }}

  deploy-test:
    needs: migrate-test  # 迁移成功后才部署
    # ...

  migrate-prod:
    needs: approval  # 人工审批后
    runs-on: ubuntu-latest
    steps:
      - name: Backup production DB
        run: |
          mysqldump -h prod-mysql -u ${{ secrets.DB_USER }} -p${{ secrets.DB_PASSWORD }} mydb > backup-$(date +%Y%m%d-%H%M%S).sql

      - name: Run Flyway migrate (prod)
        run: |
          mvn flyway:migrate \
            -Dflyway.url=jdbc:mysql://prod-mysql:3306/mydb \
            -Dflyway.user=${{ secrets.DB_USER }} \
            -Dflyway.password=${{ secrets.DB_PASSWORD }}
```

## 4.2 迁移脚本规范

命名：`V<version>__<description>.sql`，例 `V1_2_0__add_user_table.sql`。

位置：

```
<module>-repository-mybatis/
└── src/main/resources/
    └── db/migration/
        ├── V1_0_0__init.sql
        ├── V1_1_0__add_user_table.sql
        └── V1_2_0__add_order_table.sql
```

- ✅ **MUST** 版本号单调递增
- ✅ **MUST** 一个脚本一个目的
- ❌ **MUST NOT** 修改已发布的迁移脚本
- ❌ **MUST NOT** 在迁移脚本里使用数据库特定语法（除非必要）

## 4.3 回滚策略

### 向前回滚（推荐）

不写 `down` 迁移，而是写新的 `up` 迁移回退：

```sql
-- V1_2_1__rollback_user_email_index.sql
DROP INDEX idx_email ON user;
```

### 数据库备份 + 恢复（生产兜底）

```bash
# 迁移前备份
mysqldump -h host -u user -p mydb > backup-$(date +%Y%m%d-%H%M%S).sql

# 迁移失败恢复
mysql -h host -u user -p mydb < backup-20260815-103000.sql
```

## 4.4 验证

```bash
mvn flyway:migrate -Dflyway.url=jdbc:h2:mem:test   # 本地验证
mvn flyway:info                                     # 迁移历史
mvn flyway:validate                                 # 校验脚本
```

---

## 完成标准

- Tag 推送成功，格式为 `v<X.Y.Z>` 附注 Tag；GitHub Release notes 来源于 changelog
- 目标仓库（Maven Central / npmjs）可检索到该版本，且第三方项目引用验证通过
- hotfix：已合并回 `master` + `develop`，`retrospective.md` 已写入
- 回滚：目标版本存活、监控指标恢复、回滚记录已写入、相关人员已通知
- 数据库迁移：生产迁移前有备份记录，`flyway:info` 显示全部 Success
- 所有 L0 操作均有用户确认记录

## 下一步

- 发布成功 → `deployment-verification`
- 发布失败 → 第三章回滚
- hotfix 上线 → 24h 内补跑完整 CI + 跟进改进项

## 关联

- 前置：`archive-change` / `ci-gate` / `data-design`（迁移场景）
- 相关：`ci-pipeline`（配置发布流水线）/ `deployment-verification` / `infra-ops` / `observability`
- Wiki：`wiki/_common/version-management.md`、`wiki/_common/github-workflow.md`、`wiki/_common/maven-publish.md`、`wiki/_common/npm-publish.md`、`wiki/_common/git.md`、`wiki/_common/database-design.md`、`wiki/_common/ci-cd-pipeline.md`
