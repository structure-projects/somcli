# 通用命名规范

> 通用规则，适用范围：所有技术栈和项目类型。

## 目录与文件

- **MUST** 目录名使用小写 + 连字符（kebab-case）：`user-management/`、`order-service/`。
- **MUST** 文件名反映其用途与内容，见名知意。
- **MUST** 配置文件按环境分离：`application.yml`（公共） + `application-{profile}.yml`（环境专用）。
- **SHOULD** 避免过深的目录嵌套，一般不超过 4 层。

## 代码命名

### 通用原则

- **MUST** 使用描述性命名，反映变量、函数、类的用途与含义。
- **MUST** 遵循项目既有的命名风格，**禁止**在同一项目中混用多种风格。
- **MUST** 命名长度与作用域成正比：全局符号详细描述，局部变量可简写。
- **SHOULD** 布尔变量以 `is` / `has` / `can` / `should` 开头。
- **SHOULD** 集合类型变量使用复数或集合后缀（`users`、`userList`、`userMap`）。

### 语言特定

| 语言 | 类/接口 | 函数/方法 | 变量 | 常量 |
|------|---------|-----------|------|------|
| **Java** | PascalCase (`UserService`) | camelCase (`findById`) | camelCase (`userId`) | UPPER_SNAKE (`MAX_RETRY`) |
| **TypeScript/JavaScript** | PascalCase (`UserCard`) | camelCase (`fetchUser`) | camelCase (`userName`) | UPPER_SNAKE (`API_BASE_URL`) |
| **Python** | PascalCase (`UserModel`) | snake_case (`get_user`) | snake_case (`user_id`) | UPPER_SNAKE (`MAX_CONNECTIONS`) |
| **Go** | PascalCase (导出) / camelCase (私有) | PascalCase/camelCase | camelCase | PascalCase (导出常量) |
| **Rust** | PascalCase (类型) / snake_case (变量) | snake_case | snake_case | UPPER_SNAKE (`MAX_SIZE`) |

## API 与接口命名

- **MUST** RESTful 资源 URL 使用名词复数 + kebab-case：`/api/users`、`/api/order-items`。
- **MUST** 版本控制使用路径前缀（`/api/v1/users`）；Header 版本与查询参数版本 MUST NOT 使用（见 [`api-design.md`](api-design.md) 版本管理）。
- **MUST** 同一操作在不同 Controller / 模块内命名**必须统一**，不允许一处 `list`、一处 `page`、一处 `queryPage`。
- **SHOULD** 标准 CRUD 操作使用固定动词（RESTful 映射见 [`api-design.md`](api-design.md)）。

## 数据库命名

- **MUST** 表名小写 + 下划线（snake_case）：`user_info`、`order_detail`。
- **MUST** 主键统一命名为 `id`。
- **MUST** 外键命名：`{关联表名}_id`（如 `user_id`、`order_id`）。
- **SHOULD** 索引命名：`idx_{表名}_{字段}`（如 `idx_user_email`）；唯一索引 `uk_{表名}_{字段}`。
- **SHOULD** 时间字段：创建时间 `create_time`，更新时间 `update_time`。

## Java 注释规范

> 适用于所有 Java 技术栈项目。

### 类头注释（MUST）

每个类 MUST 包含 `@author`（作者）、`@version`（模块版本号）、`@since`（创建时的 JDK 版本与日期）：

```java
/**
 * 用户管理服务实现
 *
 * @author zhangsan
 * @version 1.2.0
 * @since JDK 17 2025-07-31
 */
@Service
public class UserServiceImpl implements IUserService {
    // ...
}
```

- **MUST** `@version` 与项目版本号同步，版本发布时更新。
- **SHOULD** `@since` 记录类首次创建时的 JDK 版本和创建日期，后续不修改。

### 方法注释（MUST）

每个 public/protected 方法 MUST 包含 JavaDoc：

```java
/**
 * 根据用户 ID 查询用户信息
 *
 * @param userId 用户 ID
 * @return 用户视图对象，不存在返回 null
 */
public UserVO findById(Long userId) {
    // ...
}
```

- **MUST** `@param` 描述每个参数的含义与约束。
- **MUST** `@return` 描述返回值的含义与可能为 null 的情况。
- **SHOULD** 复杂业务逻辑在方法注释中补充简要说明（1-2 句）。

### 注释语言

- **SHOULD** 注释使用中文，与项目文档语言保持一致。
- **MAY** 底层公共库（common/infra）使用英文注释。

## 禁止事项

- **禁止** 拼音命名或中英混用。
- **禁止** 单字母变量（循环变量 `i`/`j`/`k` 除外）。
- **禁止** 缩写歧义命名（如 `acc` 可能是 account / access / accumulate）。
- **禁止** 否定式布尔变量（如 `isNotEnabled`）—— 一律用肯定式（`isEnabled`）。
