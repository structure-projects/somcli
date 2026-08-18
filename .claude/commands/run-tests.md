---
name: run-tests
description: |
  运行测试并检查覆盖率。
  当用户请求 /run-tests 或"跑测试"时触发。
category: testing
phase: testing
triggers:
  - run-tests
  - 跑测试
related-skills:
  - testing
related-rules:
  - common-testing
stack: _common
priority: high
---

# 运行测试（Run Tests）

## 目的

运行单元测试与集成测试，检查覆盖率是否达标，为提交与 CI 门禁提供依据。

## 使用场景

- 编码完成后验证功能正确性
- CI 门禁前置检查 / 提交代码前的质量保障
- 需要检查测试覆盖率

## 参数

| 参数 | 行为 |
|---|---|
| 无参数 | 单测 + 覆盖率 |
| `--integration` | 追加集成测试（`mvn verify` / `npm run test:e2e`） |
| `--coverage-only` | 只出覆盖率报告，不新增用例 |

## 委派

→ 调用 `testing`，**测试分层表、用例设计要求、各栈命令、覆盖率阈值与结果回报格式均见该技能**。

**MUST NOT** 跳过失败测试；**MUST NOT** 降低覆盖率阈值（L0，需人工批准）；**MUST NOT** 为凑覆盖率写无意义测试。

## 关联

- Skill：`_common/skills/testing/SKILL.md`
- Wiki：`_common/wiki/testing-strategies.md`
- Rule：`_common/rules/common-testing.mdc`
