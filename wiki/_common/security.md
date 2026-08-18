# 安全基线

> 通用规则，适用范围：所有技术栈和项目类型。
> 各技术栈在其 `developer.md` 中定义框架特定的安全工具和配置方式。

> 📌 **栈归属说明**：本文档的**示例代码**以 **structure-boot** 栈为主（含具体类名如 `ResResultVO` / `DataScopeRedisTemplate` / `structure-security` 等）。
> **规则部分**（MUST / MUST NOT）适用于所有技术栈。
> 其他栈的使用者请参考对应栈级 Wiki（`wiki/<stack>/developer.md`）获取具体类名与组件。

## 认证与授权

### 认证

- **MUST** 所有非公开 API 必须经过认证。
- **MUST** Token/凭证通过 Header 传递，不在 URL 参数中传递。
- **MUST** JWT 设置合理过期时间（Access Token ≤ 2 小时，Refresh Token ≤ 7 天）。
- **MUST** 敏感操作（修改密码、绑定手机、删除账号）需要二次认证。
- **SHOULD** 支持 OAuth2 / OIDC 标准协议。
- **SHOULD** 提供 Token 吊销机制（黑名单或版本号）。

### 授权

- **MUST** 采用 RBAC（基于角色）或 ABAC（基于属性）模型，禁止硬编码权限判断。
- **MUST** 默认拒绝（deny-by-default）：未明确授权的即为禁止。
- **MUST** 最小权限原则：用户/服务只拥有完成其职责所需的最小权限集。
- **MUST** 数据权限下沉到行级：用户只能访问其有权查看的数据行。
- **禁止** 仅依赖前端路由或按钮隐藏做权限控制 —— 后端 API 必须独立校验。

## 输入安全

### SQL 注入防护

- **MUST** 使用参数化查询或 ORM 框架的安全 API，**禁止**字符串拼接 SQL。
- **MUST** 动态排序/分组字段使用白名单校验，不允许用户输入直接拼入 SQL。
- **禁止** 在 MyBatis 等框架中使用 `${}` 拼接用户输入（应用 `#{}` 参数化）。

### XSS 防护

- **MUST** 所有用户输入在输出到页面时做 HTML 实体编码。
- **MUST** 富文本内容使用白名单标签过滤（如 DOMPurify）。
- **SHOULD** 设置 `Content-Security-Policy` 响应头。

### CSRF 防护

- **MUST** 状态变更操作（POST/PUT/PATCH/DELETE）具备 CSRF 防护。
- **SHOULD** SPA + Token 认证模式下，CSRF 防护由 SameSite Cookie + 自定义请求头保证。

### 输入校验

- **MUST** 所有外部输入（请求参数、Header、Cookie、文件上传）必须在服务端校验。
- **MUST** 文件上传校验：类型白名单、大小限制、病毒扫描（生产环境）。
- **禁止** 信任客户端校验（前端校验仅为 UX 优化，不可替代后端校验）。

## 敏感信息保护

### 数据脱敏

- **MUST** 敏感字段（密码、手机号、身份证、银行卡）存储时加密，传输时脱敏。
- **MUST** 日志中**禁止**打印密码、Token、密钥、完整身份证号等敏感信息。
- **SHOULD** 手机号显示为 `138****1234`，身份证显示为 `320***********1234`。

### 配置管理

- **MUST** 密码/密钥/Token 使用环境变量或配置中心管理，**禁止**硬编码在代码、配置文件、README 中。
- **MUST** 生产环境使用独立的配置源，不共享开发/测试环境的密钥。
- **SHOULD** 密钥定期轮换。

## 传输安全

- **MUST** 生产环境全链路 HTTPS（TLS 1.2+），禁用 SSLv2/v3、TLS 1.0/1.1。
- **MUST** 敏感 cookie 设置 `Secure`、`HttpOnly`、`SameSite=Strict`。
- **SHOULD** 启用 HSTS（`Strict-Transport-Security`）头部。

## API 安全

- **MUST** 所有公开 API 具备频率限制（Rate Limiting），防止暴力破解和资源滥用。
- **MUST** 关键操作（登录、注册、支付、转账）加入验证码或人机校验。
- **MUST** 请求重放防护：每个请求携带唯一 nonce 或时间戳 + 签名。
- **SHOULD** API 响应不暴露内部异常堆栈、框架版本、数据库类型。

## 依赖安全

- **MUST** CI 流程中集成依赖漏洞扫描（如 Snyk、Dependabot、OWASP Dependency-Check）。
- **MUST** 发现 CVE 高危漏洞后 48 小时内修复或评估影响。
- **禁止** 引入已停止维护或存在已知严重漏洞的依赖。

## 审计与监控

- **MUST** 记录关键操作审计日志：登录/登出、权限变更、数据删除、配置修改。
- **MUST** 审计日志包含：操作人（userId）、操作时间、IP、操作类型、操作对象、操作结果。
- **SHOULD** 配置异常行为告警（频繁登录失败、大量数据导出、越权访问尝试）。

## 审查优先级（与 `code-review.md` 一致）

1. **安全** — SQL 注入、XSS、CSRF、权限绕过、敏感信息泄露
2. **认证** — Token 有效性、二次认证、会话管理
3. **授权** — RBAC 校验、行级权限、默认拒绝
4. **输入** — 参数校验、类型白名单、文件上传校验
5. **传输** — HTTPS、HSTS、Cookie 安全属性

## 禁止事项

- **禁止** 在代码/配置中硬编码密码、密钥、Token。
- **禁止** 使用弱加密算法（MD5、SHA1 用于密码哈希；DES、RC4 用于加密）。
- **禁止** 日志中输出密码、Token、完整身份证号、银行卡号。
- **禁止** 仅依赖前端做权限控制。
- **禁止** 动态拼接 SQL 时使用用户输入（参数化查询）。
- **禁止** 生产环境开放 Swagger/OpenAPI 文档（除非在内网且需认证）。
- **禁止** 跨租户数据泄露（多租户场景参见各技术栈多租户章节）。

---

## 认证详细规范（MUST）

### JWT 实现（`structure-security`）

- **MUST** 用 ``structure-security`` 的 JWT 组件，**禁止**自己实现
- **MUST** Access Token 过期时间 ≤ 2 小时
- **MUST** Refresh Token 过期时间 ≤ 7 天
- **MUST** Token 含 `userId` / `tenantId` / `username` / `authorities`

### 密码存储

- ✅ **MUST** 用 BCrypt（强度 ≥ 10）加密存储
- ❌ **MUST NOT** 用 MD5 / SHA1 / 明文

### 多因素认证（MFA）

**MUST 触发 MFA 的场景**：
- 修改密码
- 绑定 / 修改手机或邮箱
- 删除账号
- 大额资金操作
- 敏感权限变更

## 授权详细规范

### RBAC 模型

```
User → UserRole → Role → RolePermission → Permission
```

### 权限标识规范

```
<module>:<resource>:<action>
```

示例：
- `user:list` / `user:create` / `user:update` / `user:delete`
- `order:approve` / `order:export`

### 通配符权限

- `user:*` —— user 模块所有权限
- `*` —— 超级管理员

### 数据权限

**MUST** 用 ``structure-datascope`` 的 `DataRuleEngine` 实现：
- 本部门数据
- 本部门及以下数据
- 自定义数据范围
- 全部数据

## 加密规范

### 传输加密

- ✅ **MUST** 全站 HTTPS
- ❌ **MUST NOT** 内网服务间用 HTTP（敏感数据）

### 存储加密

| 数据类型 | 加密方式 |
|---|---|
| 密码 | BCrypt |
| 手机号 / 身份证 | AES-256 |
| 银行卡号 | AES-256 + 掩码显示 |
| API Secret | SHA-256（不可逆） |
| 敏感配置 | Jasypt / KMS |

### 密钥管理

- ❌ **MUST NOT** 在代码 / 配置文件硬编码密钥
- ✅ **MUST** 用环境变量 / Vault / KMS 注入
- ✅ **MUST** 定期轮换密钥

## SQL 注入防护

- ✅ **MUST** 用 MyBatis `#{}` 参数化（**禁止** `${}` 拼接）
- ❌ **MUST NOT** 在 SQL 里做字符串拼接

## XSS 防护

- ✅ **MUST** 前端用 Vue / React 默认转义（不主动 `v-html` / `dangerouslySetInnerHTML`）
- ✅ **MUST** 后端对富文本做白名单过滤（用 OWASP Java HTML Sanitizer）
- ✅ **MUST** 设置 CSP Header

## CSRF 防护

- ✅ **MUST** 非幂等操作（POST / PUT / DELETE）用 CSRF Token 或 SameSite Cookie
- ✅ **MUST** 跨域 API 用 CORS 白名单（不用 `*`）

## 越权防护

### 水平越权（IDOR）

**禁止**：通过修改 ID 访问其他用户数据。

```java
// ❌ 错误
@GetMapping("/{id}")
public UserVO findById(@PathVariable Long id) {
    return userService.findById(id);  // 任何登录用户都能看任何人
}

// ✅ 正确：校验当前用户是否有权访问
@GetMapping("/{id}")
public UserVO findById(@PathVariable Long id) {
    Long currentUserId = `UserContext`.getLongUserId();
    if (!id.equals(currentUserId) && !hasRole("ADMIN")) {
        throw new `CommonException`(AuthExceptionEnum.NO_PERMISSION);
    }
    return userService.findById(id);
}
```

### 垂直越权

- ✅ **MUST** 敏感接口加权限注解：`@PreAuthorize("hasAuthority('user:delete')")`

## 敏感数据脱敏

### 日志脱敏

- ❌ **MUST NOT** 在日志打印：密码 / 密钥 / Token / 完整身份证号 / 完整手机号 / 银行卡号
- ✅ **MUST** 用 `***` 部分脱敏

### 响应脱敏

| 字段 | 脱敏规则 |
|---|---|
| 手机号 | `138****1234` |
| 身份证 | `1101**********1234` |
| 银行卡 | `6222 **** **** 1234` |
| 邮箱 | `u***@example.com` |

## 关联

- 技能：`coding` / `expert-review`
- Wiki：`wiki/_common/error-handling.md` `wiki/_common/api-design.md`
- 规则：`common-security`
