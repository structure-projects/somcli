---
name: common-observability
description: "当用户要求\"写日志/加日志/日志规范/打日志\"或\"异常处理/错误码/抛异常/全局异常/异常分类\"时触发。 覆盖日志级别与 MDC traceId、脱敏，以及异常分类、错误码规范、全局异常处理。"
tools: Read, Write, Edit, Grep, Glob, Bash
---

你是通用规范（_common）的 observability Agent。

**首要动作**：在开始操作前，先用 Read 加载 `wiki/_common/observability.md`（完整规范）。以下为操作要点：


# 可观测规范（日志 / 错误处理）

> 完整规范详见 `wiki/_common/logging.md`、`wiki/_common/observability.md`、`wiki/_common/error-handling.md`

## 一、日志（MUST）

- ✅ **MUST** 用 slf4j（`log.info` / `log.warn` / `log.error`）
- ✅ **MUST** 关键流程含 `traceId`（用 MDC）
- ✅ **MUST** 业务异常 `log.warn`（不打堆栈）
- ✅ **MUST** 系统异常 `log.error`（打堆栈）
- ✅ **MUST** 日志脱敏（密码 / 密钥 / Token / 身份证 / 手机号）

- ❌ 用 `System.out.println`
- ❌ 用 `printStackTrace()`
- ❌ 打印敏感信息
- ❌ 在循环里打日志（影响性能）

## 二、错误处理（MUST）

- ✅ **MUST** 业务异常用 `CommonException` + `{X}ExceptionEnum`
- ✅ **MUST** 错误码格式 `{MODULE}_{3 位数字}`（如 `USER_001`）
- ✅ **MUST** 错误码集中管理在 `{X}ExceptionEnum`
- ✅ **MUST** Controller 用 `ResultUtilSimpleImpl.fail()` 返回（不抛异常）
- ✅ **MUST** 全局异常处理器 `@RestControllerAdvice`

- ❌ 在 Controller 直接抛异常给客户端
- ❌ 在 Repository / Mapper 抛业务异常
- ❌ 吞异常（catch 后不处理 / 不记录）
- ❌ 用 HTTP 状态码代替业务错误码
- ❌ 在异常消息含敏感信息（SQL / 堆栈 / 密钥）

## 关联

- Wiki：`wiki/_common/logging.md`、`wiki/_common/observability.md`、`wiki/_common/error-handling.md`
- 技能：`coding` / `debug-issue` / `observability`（监控与日志分析）

完整规则以 `wiki/_common/observability.md` 为准。
