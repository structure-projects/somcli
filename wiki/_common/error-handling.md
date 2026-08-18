# 错误处理公约

> 通用规则，适用范围：所有技术栈和项目类型。
> 各技术栈在其 `developer.md` 中定义具体的异常类和错误码枚举实现方式。

> 📌 **栈归属说明**：本文档的**示例代码**以 **structure-boot** 栈为主（含具体类名如 `ResResultVO` / `DataScopeRedisTemplate` / `structure-security` 等）。
> **规则部分**（MUST / MUST NOT）适用于所有技术栈。
> 其他栈的使用者请参考对应栈级 Wiki（`wiki/<stack>/developer.md`）获取具体类名与组件。

## 异常分类

所有业务异常 MUST 分为以下类别，每类使用不同的处理策略：

| 类别 | 说明 | 示例 | HTTP 状态码 | 策略 |
|------|------|------|:--:|------|
| **业务异常** | 业务规则不满足 | 余额不足、库存不够、状态不允许 | `400` / `409` / `422` | 返回明确错误码和用户可理解的提示 |
| **校验异常** | 参数格式或值不合法 | 手机号格式错误、必填字段为空 | `400` | 返回字段级错误明细 |
| **认证异常** | 未登录或 Token 失效 | Token 过期、未携带认证信息 | `401` | 提示重新登录 |
| **授权异常** | 已认证但无操作权限 | 普通用户访问管理员接口 | `403` | 返回无权限提示，不暴露资源是否存在 |
| **资源异常** | 请求的资源不存在 | 查询的用户不存在 | `404` | 返回资源不存在提示 |
| **系统异常** | 非预期内部错误 | 数据库连接失败、OOM | `500` | 返回通用错误信息，**禁止**暴露内部详情 |

## 错误码规范

### 编码格式

```text
{服务前缀}_{序号}
```

- **MUST** `code` 使用 **字符串** 类型，按 `{服务前缀}_{序号}` 格式编码，分隔符（`-` 或 `_`）项目内统一。
- **MUST** 成功统一用 `"SUCCESS"`。
- **MUST** 错误码具有唯一性和可追溯性，每个错误码对应一个明确的业务语义。
- **MUST** 错误码在项目内统一管理（`{X}ExceptionEnum` 枚举），**禁止**散落在代码各处用字符串字面量。

### 错误码分段

按业务领域前缀划分，序号独立递增：

| 服务/领域 | 前缀 | 示例 | 用途 |
|-----------|:--:|------|------|
| 通用系统 | `SYS` | `"SYS_001"` | 未知异常、参数校验、认证授权等通用错误 |
| 用户 | `U` | `"U-001"` | 用户不存在、状态异常 |
| 组织 | `ORG` | `"ORG_001"` | 部门/岗位相关 |
| 订单 | `O` | `"O-001"` | 订单状态、库存不足 |
| 支付 | `P` | `"P-001"` | 余额不足、支付失败 |
| ... | ... | ... | 按业务领域按需扩展 |

### 消息规范

- **MUST** 错误消息面向 API 调用方（开发者/前端），描述清晰可行动。
  - ✅ `"手机号格式不正确，请输入11位手机号"`
  - ❌ `"NullPointerException at line 42"`
- **MUST** 错误消息包含足够的上下文定位问题，但**禁止**暴露内部敏感信息。
  - ✅ `"订单 ORD-2026-001 取消失败：订单状态为 '已发货'，不允许取消"`
  - ❌ `"UPDATE t_order SET status = ? WHERE id = ? failed: Deadlock found"`
- **SHOULD** 支持 i18n（国际化），至少支持中文和英文。

## 异常处理铁律

### 抛出端

- **MUST** 业务层（Service / Domain / Use Case）**可以**抛出业务异常，但异常类型必须是对应的业务异常类。
- **MUST** 异常信息包含明确的业务语义，不使用"系统错误"、"操作失败"等含糊描述。
- **禁止** 在业务层抛出通用运行时异常（`RuntimeException` / `Exception` / `Error`）。
- **禁止** 吞掉异常（`catch (Exception e) {}` 空块）—— 如确需忽略，必须在注释中说明理由。

### 捕获端

- **MUST** 全局异常处理器统一拦截异常，转换为标准错误响应格式（见 [`api-design.md`](api-design.md)）。
- **MUST** 全局异常处理器区分业务异常（返回业务错误码）和系统异常（返回 500 + 模糊提示）。
- **禁止** 在 Controller 方法内 `try-catch` 所有异常后手动构造响应 —— 应抛出由全局处理器统一处理。

### 兜底

- **MUST** 未知异常兜底返回通用服务端错误码（如 `"SYS_001"`），不暴露堆栈。
- **MUST** 生产环境记录完整堆栈到日志系统，**禁止**在 HTTP 响应中返回。
- **SHOULD** 意外异常触发告警（如 Sentry、Prometheus Alert）。

## 跨服务调用

- **MUST** 远程调用失败时区分场景处理：
  - **强一致性**（资金、库存扣减）：抛出业务异常中断当前操作。
  - **弱一致性**（日志记录、通知）：降级处理，记录异常但不中断主流程。
- **MUST** 远程调用集成重试机制（指数退避），重试仍失败才抛出异常。
- **SHOULD** 远程调用异常映射为当前服务的业务异常，避免底层异常类型泄露。

## 日志记录

- **MUST** 所有业务异常抛出时记录日志（级别 WARN），含错误码 + 关键参数 + 触发用户。
- **MUST** 所有系统异常记录日志（级别 ERROR），含完整堆栈。
- **禁止** 在日志中记录密码、Token、完整身份证号等敏感信息。

详见 [`logging.md`](logging.md)。

## 禁止事项

- **禁止** `catch (Exception e) {}` 空捕获（吞异常）。
- **禁止** 在 HTTP 响应中返回异常堆栈（`e.printStackTrace()` 或 `stack trace`）。
- **禁止** 使用字符串字面量作为错误码（如 `throw new XxxException("USER_NOT_FOUND")`）。
- **禁止** 在 Controller 中 `try-catch` + 手动构造错误响应 —— 用全局异常处理器。
- **禁止** 返回裸 `500` 状态码让前端自己猜原因——每个错误都应有对应的错误码和消息。

---

## 错误码规范（MUST）

### 错误码格式

```
{MODULE}_{3 位数字}
```

示例：`USER_001` / `ORDER_002` / `AUTH_001` / `COMMON_001`

### 段位分配

| 模块 | 段位 | 示例 |
|---|---|---|
| `COMMON` | 001-099 | 通用错误（参数校验、系统异常） |
| `AUTH` | 100-199 | 认证 / 授权 |
| `USER` | 200-299 | 用户模块 |
| `ORDER` | 300-399 | 订单模块 |
| `PAY` | 400-499 | 支付模块 |
| ... | ... | ... |

### 集中管理（MUST）

```java
public enum UserExceptionEnum {
    USER_NOT_FOUND("USER_201", "用户不存在"),
    USERNAME_DUPLICATED("USER_202", "用户名已存在"),
    USER_STATUS_INVALID("USER_203", "用户状态不允许此操作"),
    USER_PASSWORD_INCORRECT("USER_204", "密码错误"),
    ;

    private final String code;
    private final String message;

    UserExceptionEnum(String code, String message) {
        this.code = code;
        this.message = message;
    }
    // getters
}
```

### 关键约束

- ✅ **MUST** 错误码集中管理在 `{X}ExceptionEnum`
- ✅ **MUST** 抛 `CommonException` + `{X}ExceptionEnum`
- ✅ **MUST** 错误码与 HTTP 状态码区分（业务错误码 ≠ HTTP 状态码）
- ❌ **MUST NOT** 在 Controller 直接抛异常给客户端
- ❌ **MUST NOT** 用 HTTP 状态码代替业务错误码
- ❌ **MUST NOT** 在异常消息里含敏感信息（SQL / 堆栈 / 密钥）

## 异常处理链路

### 后端

```
Controller → @RestControllerAdvice → 统一异常处理 → `ResResultVO`
```

**全局异常处理器**：

```java
@RestControllerAdvice
public class GlobalExceptionHandler {

    @ExceptionHandler(`CommonException`.class)
    public `ResResultVO`<Void> handleCommonException(`CommonException` e) {
        return `ResultUtilSimpleImpl`.fail(e.getCode(), e.getMessage());
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    public `ResResultVO`<Void> handleValidException(MethodArgumentNotValidException e) {
        // 参数校验失败
        String msg = e.getBindingResult().getFieldErrors().stream()
            .map(f -> f.getField() + ": " + f.getDefaultMessage())
            .collect(Collectors.joining("; "));
        return `ResultUtilSimpleImpl`.fail("COMMON_001", msg);
    }

    @ExceptionHandler(Exception.class)
    public `ResResultVO`<Void> handleException(Exception e) {
        log.error("系统异常", e);
        return `ResultUtilSimpleImpl`.fail("COMMON_999", "系统异常，请稍后重试");
    }
}
```

### 前端

```typescript
// 统一拦截
request.interceptors.response.use(
  (res) => res,
  (err) => {
    const { code, message } = err.response?.data || {};
    if (code === 'AUTH_101') {
      // Token 过期，跳登录
      router.push('/login');
    } else {
      ElMessage.error(message || '请求失败');
    }
    return Promise.reject(err);
  }
);
```

## 异常传播约束

- ✅ **MUST** 在 Service 抛 `CommonException`
- ✅ **MUST** 在 Controller 用 ``ResultUtilSimpleImpl`.fail()` 返回，**不抛异常**
- ❌ **MUST NOT** 在 Repository / Mapper 抛业务异常
- ❌ **MUST NOT** 吞异常（catch 后不处理 / 不记录）

## 日志记录

```java
try {
    // ...
} catch (`CommonException` e) {
    log.warn("业务异常: code={}, msg={}", e.getCode(), e.getMessage());
    throw e;
} catch (Exception e) {
    log.error("系统异常", e);  // 系统异常 MUST 打完整堆栈
    throw new `CommonException`(CommonExceptionEnum.SYSTEM_ERROR);
}
```

**约束**：
- 业务异常：`log.warn`（不打堆栈）
- 系统异常：`log.error`（MUST 打堆栈）
- ❌ **MUST NOT** 在日志打印敏感信息

## 关联

- 技能：`coding` / `debug-issue`
- Wiki：`wiki/_common/api-design.md` `wiki/_common/security.md` `wiki/_common/logging.md`
- 规则：`common-observability`
