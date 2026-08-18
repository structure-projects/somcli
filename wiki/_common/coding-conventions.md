# 通用编码约定（coding-conventions）

> 本文档是**所有技术栈**的通用编码约定，吸收各栈重复的内容。
> 各栈 Wiki 只保留**栈特有差异**，通用部分统一引用本文档。

> 📌 **栈归属说明**：本文档的**示例代码**以 **structure-boot** 栈为主（含具体类名如 `ResResultVO` / `DataScopeRedisTemplate` / `structure-security` 等）。
> **规则部分**（MUST / MUST NOT）适用于所有技术栈。
> 其他栈的使用者请参考对应栈级 Wiki（`wiki/<stack>/developer.md`）获取具体类名与组件。

## 命名约定

### 通用命名（跨语言）

| 元素 | 规则 | 示例 |
|---|---|---|
| 类名 / 接口 | `UpperCamelCase` | `UserService` / `OrderRepository` |
| 方法 / 变量 | `lowerCamelCase` | `findById` / `userName` |
| 常量 | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT` |
| 包名 | 全小写无分隔符 | ``cn.structured.admin.biz.service`` |
| 数据库表 / 字段 | `lower_snake_case` | `user_role` / `create_time` |
| REST API URL | `kebab-case` | `/api/user-roles` |
| 文件名（前端） | `kebab-case` | `user-profile.vue` |
| 组件名（前端） | `PascalCase` | `<UserProfile />` |

### 禁止

- ❌ **MUST NOT** 用拼音
- ❌ **MUST NOT** 用无意义缩写（`usr` / `mgr` / `svc`）
- ❌ **MUST NOT** 用数字开头
- ❌ **MUST NOT** 用单字母（除循环变量 `i` / `j` / `k`）

## 注释规范

### Java 类头注释（MUST）

```java
/**
 * 用户管理服务实现
 *
 * @author zhangsan
 * @version 1.2.0
 * @since JDK 17 2026-08-13
 */
```

### 方法注释（MUST）

每个 `public` / `protected` 方法 MUST 有 JavaDoc：

```java
/**
 * 根据用户 ID 查询用户信息
 *
 * @param userId 用户 ID
 * @return 用户视图对象，不存在返回 null
 */
```

### 前端 TS 注释

```typescript
/**
 * 根据用户 ID 查询用户信息
 * @param userId 用户 ID
 * @returns 用户视图对象
 */
```

## 异常处理通用约定

- ✅ **MUST** 业务异常用 `CommonException` + `{X}ExceptionEnum`
- ✅ **MUST** 错误码集中管理
- ❌ **MUST NOT** 吞异常（catch 后不处理）
- ❌ **MUST NOT** 在异常消息含敏感信息

详细见 `wiki/_common/error-handling.md`。

## 日志通用约定

- ✅ **MUST** 用 slf4j（Java）/ 对应语言标准日志库
- ✅ **MUST** 级别：DEBUG / INFO / WARN / ERROR
- ❌ **MUST NOT** 用 `System.out.println`
- ❌ **MUST NOT** 打印敏感信息（密码 / 密钥 / Token）

详细见 `wiki/_common/logging.md`。

## 统一响应通用约定

- ✅ **MUST** 统一响应包装（如 ``ResResultVO`<T>`）
- ✅ **MUST** 统一响应构造（如 `ResultUtilSimpleImpl`）
- ❌ **MUST NOT** 在 Controller 直接抛异常

详细见 `wiki/_common/api-design.md`。

## 参数校验通用约定

- ✅ **MUST** 用 `@Valid` + JSR-303 注解（Java）
- ✅ **MUST** 前端用表单校验
- ❌ **MUST NOT** 信任前端校验（后端 MUST 二次校验）

## 依赖注入通用约定

- ✅ **MUST** 优先构造器注入（推荐 Lombok `@RequiredArgsConstructor`）
- ✅ **SHOULD** 字段注入用 `@Resource`（**不用 @Autowired**）
- ❌ **MUST NOT** 在字段上直接用 `@Autowired`

## 空值处理通用约定

- ✅ **MUST** 用 `Optional` 包装可能为 null 的返回
- ✅ **MUST** 用 `@NotNull` / `@Nullable` 标注
- ❌ **MUST NOT** 直接返回 `null`（除明确说明）

## 日期时间通用约定

- ✅ **MUST** 用 `LocalDateTime` / `LocalDate` / `Instant`
- ❌ **MUST NOT** 用 `Date` / `Calendar`（已过时）
- ❌ **MUST NOT** 用字符串存日期

## 集合处理通用约定

- ✅ **MUST** 用不可变集合（`List.of` / `Set.of` / `Map.of`）
- ✅ **MUST** 用 Stream API 替代循环（简单场景）
- ❌ **MUST NOT** 返回 `null` 集合（应返回空集合）

## 关联

- Wiki：`wiki/_common/naming.md` `wiki/_common/error-handling.md` `wiki/_common/logging.md` `wiki/_common/api-design.md`
- 栈级 Wiki：`<stack>/wiki/developer.md`（栈特有差异）
