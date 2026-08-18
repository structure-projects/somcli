---
name: create-feature
description: |
  当用户要求"新建特性/新增子目录/新建功能模块/新建目录"时触发。
  按目录类型识别结果创建对应结构：特性目录 / 子包 / 非代码目录。
  MUST 先询问目录类型，禁止默认按"子包"处理。

triggers:
  - 新建特性
  - 新增特性
  - 新建子目录
  - 新增子目录
  - 新建功能模块
  - 新建模块
  - 创建子目录
  - 创建特性
  - 创建目录
  - add feature
  - create directory
  - 新建目录

role: developer
phase: support
delegates-to: coding


allowed-tools: Bash, Read, Write, Edit, Glob

related-rules:
  - common-project-structure
  - common-core
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/project-structure.md
  - wiki/<stack>/developer.md

produces:
  - 特性目录结构（如 features/<name>/）
  - 或子包结构
  - 或非代码目录

requires: []

human-in-the-loop:
  - 目录类型 MUST 询问用户（特性目录 / 子包 / 非代码目录）
  - 特性目录的组织方式 MUST 与用户确认

on-failure: |
  用户未明确类型 → MUST 追问
  特性目录与子包混淆 → 重新询问

mode: auto

category: coding
stack: _common
priority: high
---

# 新建特性 / 子目录

> 按目录类型创建对应结构。**MUST 先询问目录类型，禁止默认按"子包"处理**。

## 前置条件

- 用户明确要新建目录 / 特性 / 功能模块

## 执行步骤

### 第 1 步：询问目录类型（MUST）

```
您要创建的目录是哪种类型？

a) 特性目录（feature directory）
   - 独立功能模块，跨层组织代码
   - 示例：features/user-management/{controller,service,repository}/
   - 不影响现有包结构

b) 子包（subpackage）
   - 在现有 Java package 下创建子包
   - 示例：cn.structured.user.features.UserController
   - 会影响 package 语句和 import

c) 非代码目录
   - 不放 Java 源代码
   - 示例：docs/、scripts/、examples/、assets/

请回复 a / b / c：
```

### 第 2 步：按类型创建

#### 类型 a：特性目录

```
features/<feature-name>/
├── README.md                   # 特性说明
├── controller/                 # 控制器（或按语言调整）
│   └── {X}Controller.java
├── service/                    # 业务逻辑
│   └── {X}Service.java
├── repository/                 # 数据访问
│   └── {X}Repository.java
├── model/                      # 模型
│   ├── {X}Entity.java
│   ├── {X}DTO.java
│   └── {X}VO.java
└── tests/                      # 测试
    └── {X}ServiceTest.java
```

**关键**：
- 跨层组织（controller / service / repository 在同一特性目录下）
- 不影响现有包结构
- 适合独立功能模块、实验性功能

#### 类型 b：子包

在现有 package 下创建子包：

```
src/main/java/cn/structured/{X}/
├── application/        # 现有
├── domain/             # 现有
└── features/           # 新增子包
    └── {Y}Controller.java
```

**关键**：
- 影响 `package` 语句
- 影响 `import`
- 需要符合 Java 命名规范

#### 类型 c：非代码目录

```
<directory-name>/
└── README.md           # 说明文档
```

**关键**：
- 不放 Java 源代码
- 通常放 markdown / 脚本 / 资源文件

### 第 3 步：生成 README（特性目录 / 非代码目录 MUST）

特性目录的 README.md：

```markdown
# <特性名>

## 用途
<这个特性是做什么的>

## 目录结构
<文件列表>

## 使用方式
<如何使用这个特性>

## 依赖
<依赖的其他模块 / 服务>
```

### 第 4 步：验证

```bash
# 特性目录
tree features/<feature-name>/  # 或 ls -R

# 子包（编译验证）
mvn clean compile  # 或 npm run build
```

## 产出物

- 特性目录结构（含 README）
- 或子包结构
- 或非代码目录

## 完成标准

- 目录类型经用户确认
- 结构与类型匹配
- README 就位（特性 / 非代码目录）
- 编译通过（如适用）

## 关联

- 前置：无
- 后续：在特性目录下开发 → `coding` 或栈级 `new-*` 技能
- Wiki：`wiki/_common/project-structure.md`
- 规则：`common-project-structure`
