---
name: testing
description: |
  当用户要求"写测试/补测试/跑测试/单测/集成测试/E2E/Playwright/Testcontainers"时触发。
  覆盖单元测试、集成测试、E2E 三层；MUST 先按被测范围选层，再按该层的工具与约束执行。
  与 coding 技能并行执行。

triggers:
  - 写测试
  - 补测试
  - 跑测试
  - 单测
  - 单元测试
  - 测试用例
  - write test
  - UT
  - 集成测试
  - IT
  - 跨模块测试
  - Testcontainers
  - integration test
  - E2E 测试
  - 端到端测试
  - E2E
  - e2e test
  - Playwright
  - Cypress

role: tester
phase: testing

allowed-tools: Bash, Read, Write, Edit, Glob, Grep

related-rules:
  - common-core
  - common-testing
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/testing-strategies.md
  # 栈级规范（MUST 根据识别的栈动态替换 <stack>）
  - wiki/<stack>/tester.md

produces:
  - 单元测试代码
  - 集成测试代码 + Testcontainers 配置
  - E2E 测试代码（Playwright / Cypress）
  - 覆盖率报告

requires:
  - skill: coding
    condition: 至少有一项编码任务完成
    error: 无代码可测，MUST 先完成部分编码

human-in-the-loop:
  - id: select-test-layer
    action: 测试分层选择（单元 / 集成 / E2E）MUST 与被测代码作者确认（如非同一人）
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 分层判断错误只影响测试代码，可重写，不影响生产代码
  - id: lower-coverage-threshold
    action: 降低覆盖率阈值 MUST 人工批准
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A

on-failure: |
  测试失败 → 分析是代码 bug 还是测试 bug；代码 bug 回到 coding
  覆盖率不达标 → 补充测试用例；MUST NOT 降低覆盖率阈值（L0）
  Testcontainers 拉不到镜像 → 明说"未执行，原因 X"，MUST NOT 降级为 H2 / 内存实现

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

category: testing
stack: _common
priority: high
---

# 测试

> 单元 / 集成 / E2E 三层统一入口。与 `coding` 技能并行执行。

## 前置条件

- 至少有一项编码任务完成（`tasks.md` 中有 `- [x]` 项）

## 第 0 步：选层（MUST 先做）⭐

| 层级 | 范围 | 工具 | 覆盖要求 | 本文档章节 |
|---|---|---|---|---|
| **单元测试** | 函数级、类级 | Jest / JUnit / pytest | 行覆盖 ≥ 80%，分支 ≥ 70% | 一 |
| **集成测试** | 跨模块、跨服务、DB / MQ / Redis | Testcontainers / Supertest | 关键路径 100% | 二 |
| **E2E 测试** | 端到端用户场景（UI + 后端 + DB） | Playwright / Cypress | 核心业务流程 100% | 三 |

选层判据：**被测边界内是否包含进程外依赖**——不含则单元；含中间件则集成；含浏览器/真实用户路径则 E2E。

三层 MUST 按顺序推进：单元通过 → 集成 → E2E。

MUST Read：`wiki/_common/testing-strategies.md`；按需 Read：`wiki/<stack>/tester.md`。

---

# 一、单元测试

## 测试替身决策树

```
被测代码依赖什么？
   ├─ 纯函数 / 纯计算 → 无替身
   ├─ 外部 HTTP 服务 → mock（如 MSW / WireMock）
   ├─ 数据库 → 归入集成测试（用 Testcontainers，不用 H2）
   ├─ 消息队列 → 归入集成测试
   ├─ 文件系统 → 临时目录
   └─ 时间 / 随机数 → 注入 Clock / Seed
```

## 执行步骤

1. **读被测代码**：理解业务逻辑、输入输出、边界条件、异常路径
2. **设计用例**：每个函数 MUST 覆盖正常路径 + 边界条件（空值 / 最大值 / 最小值 / 边界字符）+ 异常路径（非法输入 / 依赖失败 / 并发冲突）
3. **编写测试**：
   - 命名 `should<Expected>When<Condition>` 或 `test<What>_<Condition>`
   - 结构 Arrange / Act / Assert（Given / When / Then）
   - 每个测试 MUST 独立运行，无顺序依赖
4. **跑测试 + 覆盖率**：

```bash
mvn clean test jacoco:report      # Java
npm test -- --coverage            # Node
pytest --cov --cov-report=html    # Python
go test ./... -cover              # Go
```

5. **分析覆盖率**：行 ≥ 80%、分支 ≥ 70%、关键路径 100%。不达标 → 回第 2 步补用例，**MUST NOT 降低阈值**（L0）

6. **回报结果**：MUST 同时给出通过数、覆盖率三档与跳过 / flaky 用例

```
🧪 测试结果

✅ 单元测试：45 passed, 0 failed
✅ 集成测试：3 passed, 0 failed
📊 覆盖率：行 85.2% ✅ ｜ 分支 78.3% ⚠️ ｜ 方法 92.1% ✅
⏱️ 耗时：2m 34s
```

MUST 显式报告跳过的测试与 flaky test，**MUST NOT** 静默忽略。

---

# 二、集成测试

## 核心原则

- ✅ **MUST** 用 Testcontainers 起真实 DB / MQ / Redis
- ✅ **MUST** 用 `@Testcontainers` + `@Container` + `@ServiceConnection`（Spring Boot 3.1+）
- ✅ **MUST** 测试后清理数据
- ❌ **MUST NOT** 用 H2 替代 MySQL（SQL 方言与行为不一致）
- ❌ **MUST NOT** 用内存 MQ 替代 RocketMQ / Kafka
- ❌ **MUST NOT** 用 `@MockBean` 替代真实中间件

## Testcontainers 示例

```java
@SpringBootTest
@Testcontainers
class UserServiceIT {

    @Container
    @ServiceConnection
    static MySQLContainer<?> mysql = new MySQLContainer<>("mysql:8.0")
        .withDatabaseName("test")
        .withUsername("test")
        .withPassword("test");

    @Autowired
    private IUserService userService;

    @Test
    void shouldCreateUser() {
        // 真实 MySQL 环境测试
    }
}
```

```java
@Container
@ServiceConnection
static GenericContainer<?> redis = new GenericContainer<>("redis:7-alpine")
    .withExposedPorts(6379);

@Container
static KafkaContainer kafka = new KafkaContainer(
    DockerImageName.parse("confluentinc/cp-kafka:latest")
);
```

---

# 三、E2E 测试

## 工具选择

| 工具 | 推荐度 | 说明 |
|---|---|---|
| **Playwright** ⭐ | 推荐 | 多浏览器 / 快 / 内置等待 |
| Cypress | 备选 | 易上手 / 社区大 |
| Selenium | 不推荐 | 老旧 |

## 关键约束

- ✅ **MUST** 覆盖核心业务流程（登录 / 下单 / 支付）
- ✅ **MUST** 用 `data-testid` 选择器（不用 CSS class）
- ✅ **MUST** 测试独立（无顺序依赖）
- ❌ **MUST NOT** 用 `page.waitForTimeout`（应用 `waitForSelector`）

## Playwright 示例

```typescript
import { test, expect } from '@playwright/test'

test.describe('用户登录', () => {
  test('正常登录', async ({ page }) => {
    await page.goto('/login')
    await page.fill('input[name="username"]', 'admin')
    await page.fill('input[name="password"]', 'password')
    await page.click('button[type="submit"]')
    await expect(page).toHaveURL('/dashboard')
    await expect(page.locator('[data-testid="user-name"]')).toHaveText('admin')
  })

  test('密码错误', async ({ page }) => {
    await page.goto('/login')
    await page.fill('input[name="username"]', 'admin')
    await page.fill('input[name="password"]', 'wrong')
    await page.click('button[type="submit"]')
    await expect(page.locator('[data-testid="error-message"]')).toBeVisible()
  })
})
```

---

## 完成标准

- 所有新增代码有对应单元测试，覆盖率达标
- 跨进程依赖有集成测试，关键路径 100%
- 核心业务流程有 E2E 测试
- 全部测试通过，且本地 / CI 表现一致

## 下一步

测试全部通过后 MUST 进入 `expert-review`（专家评审）。

- 评审产出 `review.md`，无未解决 MUST fix 后方可进入 `archive-change`
- 主线顺序：`coding` → **`testing`** → `expert-review` → `archive-change` → `ci-gate` → `deployment-verification`

## 关联

- 并行：`coding`
- 后续：`expert-review`
- Wiki：`wiki/_common/testing-strategies.md`
