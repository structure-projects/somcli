# 部署规范

> 本规范定义了 structure-projects 生态下所有技术栈的部署流程、健康检查标准、回滚决策与权限边界。规则使用 RFC 2119 风格标注强制级别（MUST / SHOULD / MAY）。

## 部署前检查

部署前 MUST 完成以下检查项，任一未通过则禁止继续部署：

- [ ] 所有测试通过（单元测试 + 集成测试 + E2E）
- [ ] CI 流水线为 green 状态
- [ ] 变更日志（changelog）已更新
- [ ] 数据库迁移脚本已准备并验证（含回滚脚本）
- [ ] 健康检查端点已验证可达
- [ ] 回滚计划已准备（明确回滚版本号 + 步骤）
- [ ] 干系人通知已发送（staging/production 环境）
- [ ] 配置项已核对（环境变量 / 密钥 / feature flag）
- [ ] 依赖服务版本兼容性已确认

SHOULD 在部署前进行一次 dry-run 预演，验证流程无阻塞。

## 部署流程

### 标准部署（minor/major）

1. **创建 release branch**：从 `develop` 创建 `release/vX.Y.Z`，冻结新功能合入
2. **编译 + 打包 + 镜像构建**：通过 CI 产出可追溯的镜像（带 git sha 标签）
3. **灰度部署**（10% → 50% → 100%）：
   - 10% 灰度：观察 5 分钟，关注错误率与延迟
   - 50% 灰度：观察 10 分钟，关注业务指标
   - 100% 全量：完成发布
4. **每阶段健康检查**：通过健康端点 + 关键业务指标验证
5. **全量发布后监控 30 分钟**：关注告警面板与日志异常

每阶段 MUST 通过健康检查后才可进入下一阶段；任一阶段异常 SHOULD 触发回滚评估。

### Hotfix 部署

1. **从 main/master 创建 hotfix branch**：`hotfix/vX.Y.Z-patch-N`
2. **最小修复 + 快速 CI**：仅包含修复必要代码，禁止夹带其他变更
3. **直接灰度部署**：可压缩灰度阶段为 50% → 100%
4. **事后补回 develop**：hotfix 合入 main 后 MUST 同步回流 develop 分支

Hotfix 部署 SHOULD 在修复完成后补写变更日志与 retrospective。

## 健康检查模板

### 端点定义

- Spring Boot 项目：`GET /actuator/health`
- 通用项目：`GET /api/health`
- 返回内容示例：
  ```json
  {"status": "UP", "components": {"db": {"status": "UP"}, "cache": {"status": "UP"}}}
  ```

### 检查策略

- 检查间隔：5s
- 超时时间：3s
- 连续成功次数：3 次视为健康
- 连续失败次数：3 次视为不健康，触发告警

### 关键指标阈值

| 指标 | 阈值 | 告警级别 |
|---|---|---|
| CPU 使用率 | < 80% | warning |
| 内存使用率 | < 85% | warning |
| 错误率（5xx） | < 1% | critical |
| P99 延迟 | < 业务基线 | warning |
| 重启次数 | < 3 次/小时 | critical |

## 回滚决策树

### 触发条件

- 健康检查连续失败
- 关键指标异常（错误率超阈值 / 延迟翻倍）
- 业务侧报告严重故障
- 用户手动触发回滚

### 决策流程

```
触发回滚评估
   ├── 能否版本回退（无 DB 不可逆变更）?
   │   ├── 是 → 回退到上一版本镜像 + 验证
   │   └── 否 → 数据回滚 + 修复部署
   └── 回滚后 MUST 创建 retrospective.md
```

### 强制要求

- 回滚 MUST 经用户确认（production 环境需两人确认）
- 回滚操作 MUST 记录操作日志（操作人 / 时间 / 原因 / 目标版本）
- 回滚后 MUST 创建 `retrospective.md` 分析根因
- 不可逆数据迁移 MUST 在部署前提供回滚脚本

## 部署权限边界

| 角色 | dev/test | staging | production |
|---|---|---|---|
| 开发人员 | 可部署 | 禁止 | 禁止 |
| DevOps | 可部署 | 可部署 | 可部署（需审批） |
| Release Manager | 可部署 | 可部署 | 可部署（需审批） |

### 强制约束

- Production 部署 MUST 审批 + 两人确认
- 禁止绕过 CI 直接部署到任何环境
- 禁止使用 `latest` 标签部署到 production
- 部署镜像 MUST 使用不可变标签（git sha 或语义化版本）
- Production 部署 SHOULD 避开业务高峰期

## 关联

- 通用规则：`common-core` / `common-project-stack-detection`
- 技能：`deployment-verification` / `release-ops`
- 变更管理：`changes/templates/` proposal-hotfix
- 相关文档：`_common/wiki/version-management.md` / `_common/wiki/error-handling.md`
