# npm 发布规范

> 本文档是 structure-projects 生态 npm 包发布的参考手册。

## 包类型分级

| 类型 | 命名 | 发布 |
|---|---|---|
| **L1 组件库** | `@structure-projects/components` | ✅ 发布 npm |
| **L2 业务组件库** | `@structure-projects/<X>-ui-components` | ✅ 发布 npm |
| **业务应用** | `<X>-ui` | ❌ MUST `private: true` |

## package.json 标准配置

```json
{
  "name": "@structure-projects/components",
  "version": "1.2.0",
  "description": "...",
  "private": false,
  "publishConfig": {
    "access": "public"
  },
  "files": ["dist"],
  "main": "./dist/index.js",
  "module": "./dist/index.mjs",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "import": "./dist/index.mjs",
      "require": "./dist/index.js",
      "types": "./dist/index.d.ts"
    }
  },
  "scripts": {
    "build": "...",
    "test": "...",
    "prepublishOnly": "npm run build && npm run test"
  }
}
```

## 发布流程

```
本地构建 → npm version → 测试 → npm publish → 打 Tag → GitHub Release
```

## 关键约束

- ✅ **MUST** scope 为 `@structure-projects`
- ✅ **MUST** `publishConfig.access: "public"`
- ✅ **MUST** `files: ["dist"]`（只发布产物）
- ✅ **MUST** `prepublishOnly` 跑 build + test
- ❌ **MUST NOT** 业务包发布到 npm
- ❌ **MUST NOT** 不带版本号发布

## 关联

- 技能：`release-ops`
- Wiki：`wiki/_common/version-management.md`
