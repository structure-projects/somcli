---
name: api-design
description: |
  当用户要求"设计接口/定 API/写 OpenAPI/定义接口"、"详细设计/LLD/类图/时序图"
  或"生成 API 文档/Swagger 文档"时触发。
  设计层统一入口：API 契约设计 → 详细设计（类图 / 时序图 / 错误处理 / 测试策略）→ OpenAPI 文档。
  产出 changes/proposals/<id>/design.md。

triggers:
  - 设计接口
  - 定 API
  - API 设计
  - 定义接口
  - OpenAPI
  - RESTful
  - 详细设计
  - LLD
  - 类图
  - 时序图
  - 设计文档
  - Swagger 文档
  - API 文档

role: architect
phase: design
supports-skill: coding

allowed-tools: Bash, Read, Write, Edit, Glob, Grep

related-rules:
  - common-api-design
  - common-project-structure
  - common-delivery
  - common-core
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/api-design.md
  - wiki/_common/detailed-design.md
  - wiki/<stack>/swagger.md
  - wiki/<stack>/developer.md

stack-constraints:
  structure-boot:
    response-wrapper: "ResResultVO<T>"
    response-builder: "ResultUtilSimpleImpl"
    pagination-request: "ReqPage"
    pagination-response: "ResPage<T>"
    error-handling: "CommonException + {X}ExceptionEnum"
    open-api-prefix: "/api/open"  # Open{X}Controller
    inner-api-prefix: "/api"      # {X}Controller
    version-strategy: "URL 路径版本（/api/v1/...）"
    openapi-library: "springdoc-openapi"
    forbidden:
      - "直接抛异常到 Controller 层"
      - "在 Controller 注入 Mapper / PO"
      - "swagger 2.x 老注解（@Api / @ApiOperation）"

produces:
  - changes/proposals/<id>/design.md（详细设计文档）
  - API 契约（路径 / 方法 / 请求响应）+ Controller 接口骨架 + DTO/VO
  - 错误码定义（{X}ExceptionEnum）
  - 类图 / 时序图 / 数据模型 / 测试策略
  - OpenAPI / Swagger 注解与文档

requires:
  - skill: requirement-analysis
    condition: changes/proposals/<current>/proposal.md exists
    error: 无变更提案，MUST 先调用 requirement-analysis

human-in-the-loop:
  - id: select-design-depth
    action: 设计粒度（仅 API 契约 / 完整 LLD）MUST 按变更规模确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 粒度判断错误只影响设计文档篇幅，可补写，无代码副作用
  - id: confirm-api-contract
    action: API 路径 / HTTP 方法 / 请求响应结构 MUST 与用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 契约未进入编码前可改；已被下游消费方引用后视为破坏性变更，MUST 升版本
  - id: confirm-error-codes
    action: 错误码分配 MUST 与用户确认（避免与既有码冲突）
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 冲突可改码；已对外发布的错误码 MUST NOT 复用于其他语义
  - id: confirm-data-model-and-state-machine
    action: 数据模型与关键算法 / 状态机 MUST 与用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 设计阶段可改；已落库的模型变更 MUST 走 database-migration-cd

on-failure: |
  API 不符合 RESTful → 修正路径 / 方法后重试，MUST NOT 用动词路径将就
  错误码冲突 → 与用户确认新码，MUST NOT 复用既有码
  设计不符合 HLD → 回到 high-level-design 调整，MUST NOT 在 LLD 里私自改架构
  设计遗漏关键场景 → 补充后重试

category: api-design
stack: _common
priority: high
---

# API 与详细设计

> API 契约 → 详细设计（LLD）→ OpenAPI 文档。产出 `changes/proposals/<id>/design.md`。

前置：变更提案存在；已识别项目栈；新项目 HLD 已完成。

## 第 0 步：确定设计粒度

| 变更规模 | 产出 | 本文档章节 |
|---|---|---|
| 仅新增 / 调整接口 | API 契约 + OpenAPI 注解 | 一 + 三 |
| 功能级变更（含模型 / 流程） | 完整 `design.md` | 一 + 二 + 三 |
| trivial / minor（typo / 配置） | 跳过设计，直接 `coding` | — |

---

# 一、API 契约设计

## 第 1 步：确定 API 类型

| 类型 | 前缀 | 说明 |
|---|---|---|
| **内部 API** | `/api/{资源}` | 前端 / 内部服务调用，需认证 |
| **开放 API** | `/api/open/{资源}` | 第三方调用，需签名 / 开放认证 |

## 第 2 步：设计路径（MUST 遵守）

- ✅ **MUST** `kebab-case`：`/api/user-roles`（不是 `/api/userRoles`）
- ✅ **MUST** 名词复数：`/users`（不是 `/user`）
- ✅ **MUST** 含版本号：`/api/v1/users`
- ❌ **MUST NOT** 含动词：不写 `/api/getUsers`

## 第 3 步：设计 HTTP 方法

| 操作 | 方法 | 路径示例 | 幂等 |
|---|---|---|---|
| 查询单条 | GET | `/api/v1/users/{id}` | ✅ |
| 查询列表 | GET | `/api/v1/users` | ✅ |
| 分页查询 | GET | `/api/v1/users/page` | ✅ |
| 创建 | POST | `/api/v1/users` | ❌（需幂等键） |
| 全量更新 | PUT | `/api/v1/users/{id}` | ✅ |
| 部分更新 | PATCH | `/api/v1/users/{id}` | ❌ |
| 删除 | DELETE | `/api/v1/users/{id}` | ✅ |

## 第 4 步：设计请求 / 响应

请求：路径参数 `@PathVariable`、查询参数 `@RequestParam`、请求体 `@RequestBody @Valid`、分页 `page(UserQuery query, ReqPage reqPage)`。

```java
ResResultVO<UserVO>                  // 统一响应包装
ResResultVO<ResPage<UserVO>>         // 分页响应

ResultUtilSimpleImpl.success(data)
ResultUtilSimpleImpl.fail(code, message)
```

## 第 5 步：设计错误码

```java
public enum UserExceptionEnum {
    USER_NOT_FOUND("USER_001", "用户不存在"),
    USERNAME_DUPLICATED("USER_002", "用户名已存在"),
}
```

- 格式 `{MODULE}_{3 位数字}`
- ✅ **MUST** 在 `{X}ExceptionEnum` 集中管理
- ✅ **MUST** 抛 `CommonException`

## 第 6 步：幂等性设计

非幂等操作（POST / PATCH）MUST 支持幂等：客户端传 `Idempotency-Key` header，服务端用 Redis SETNX 去重。

---

# 二、详细设计（LLD）

## 第 1 步：读 HLD 与提案

```bash
cat changes/proposals/<current>/hld.md       # 新项目
cat changes/proposals/<current>/proposal.md
```

## 第 2 步：类图

```mermaid
classDiagram
    class UserEntity {
        +Long id
        +String username
        +Long tenantId
    }
    class UserRepository {
        <<interface>>
        +findById(Long) Optional~UserEntity~
        +save(UserEntity) UserEntity
    }
    class UserService {
        <<interface>>
        +findById(Long) UserVO
        +create(UserDTO) Long
    }
    class UserServiceImpl {
        -UserRepository userRepository
        +findById(Long) UserVO
    }

    UserServiceImpl ..|> UserService
    UserServiceImpl --> UserRepository
    UserRepository ..> UserEntity
```

## 第 3 步：接口清单

按第一部分的契约列出：

```
GET    /api/v1/users/{id}        → UserVO
GET    /api/v1/users/page        → ResPage<UserVO>
POST   /api/v1/users             → Long
PUT    /api/v1/users/{id}        → void
DELETE /api/v1/users/{id}        → void

GET    /api/open/v1/users/{id}   → UserVO
```

## 第 4 步：数据模型

调用 `data-design` 输出 Entity / PO / DTO / VO / Query 定义、DDL、Flyway 迁移脚本。

## 第 5 步：时序图

```mermaid
sequenceDiagram
    participant C as Client
    participant Ctrl as Controller
    participant Svc as Service
    participant Repo as Repository
    participant DB as Database

    C->>Ctrl: POST /api/v1/users
    Ctrl->>Svc: create(dto)
    Svc->>Repo: save(entity)
    Repo->>DB: INSERT
    DB-->>Repo: id
    Repo-->>Svc: entity
    Svc-->>Ctrl: id
    Ctrl-->>C: ResResultVO<Long>
```

## 第 6 步：错误处理表

| 场景 | 错误码 | HTTP 状态 | 处理 |
|---|---|---|---|
| 用户不存在 | USER_001 | 404 | CommonException |
| 用户名重复 | USER_002 | 400 | CommonException |
| 参数校验失败 | COMMON_001 | 400 | CommonException |

## 第 7 步：测试策略

标注各层覆盖范围，交由 `testing` 技能执行：单测（Service 全方法）/ 集成（Controller + Testcontainers）/ E2E（关键业务流程）。

## 第 8 步：产出 design.md

```markdown
# 详细设计：<标题>

## 类图
## 接口定义
## 数据模型
## 时序图
## 错误处理
## 测试策略
## 关键决策
```

---

# 三、OpenAPI 文档

```java
@Tag(name = "用户管理", description = "用户相关 API")
@RestController
@RequestMapping("/api/v1/users")
public class UserController {

    @Operation(summary = "根据 ID 查询用户", description = "返回用户详情")
    @GetMapping("/{id}")
    public ResResultVO<UserVO> findById(
            @Parameter(description = "用户 ID", required = true)
            @PathVariable Long id) {
        return ResultUtilSimpleImpl.success(userService.findById(id));
    }
}
```

访问：`http://localhost:8080/swagger-ui/index.html`

## 关键约束

- ✅ **MUST** 用 `springdoc-openapi`（Spring Boot 4）
- ✅ **MUST** 每个 Controller 有 `@Tag`、每个方法有 `@Operation`、每个参数有 `@Parameter`
- ❌ **MUST NOT** 用 swagger 2.x 老注解（`@Api` / `@ApiOperation`）

---

## 完成标准

- 路径符合 RESTful，响应统一 `ResResultVO<T>`
- 错误码集中管理，无冲突
- 非幂等操作支持幂等键
- 功能级变更有完整 `design.md`（类图 / 接口 / 模型 / 时序 / 错误 / 测试策略）
- OpenAPI 注解完整，Swagger UI 可访问

## 下一步

进入 `coding` 开始编码实现。

## 关联

- 前置：`high-level-design`（新项目）或 `requirement-analysis`（历史项目）
- 后续：`coding`
- 支撑：`data-design`
- 相关：`testing`（执行本技能定义的测试策略）
- Wiki：`wiki/_common/api-design.md`、`wiki/_common/detailed-design.md`、`wiki/<stack>/swagger.md`
