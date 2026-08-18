# CI/CD 流水线规范

> 本文档是 structure-projects 生态 CI/CD 流水线的单一来源。
> 所有项目 MUST 使用 GitHub Actions；流水线模板 MUST 从本文档派生。

## 流水线三件套

生态标准流水线由 **3 个 workflow** 组成，按职责分离：

| Workflow | 触发 | 用途 |
|---|---|---|
| **`build-and-push.yml`** | `workflow_dispatch`（手动） | 构建 Docker 镜像并推送到镜像仓库 |
| **`release-maven.yml`** | `workflow_dispatch` | 发布 Java 包到 Maven Central |
| **`publish-npm.yml`** | `workflow_dispatch` | 发布 npm 包到 npmjs.com |

**关键约定**：
- ❌ **MUST NOT** 使用 `on: release: types: [published]` 自动触发（避免误发布）
- ✅ **MUST** 所有发布使用 `workflow_dispatch` 手动触发（可控）
- ✅ **MUST** Tag 命名遵循约定：`backend-<module>-<version>` / `maven-<module>-<version>` / `npm-<component>-<version>` / `v<semver>`

## 通用 Secrets

| Secret | 用途 |
|---|---|
| `ALIYUN_ACR_USERNAME` / `ALIYUN_ACR_PASSWORD` | 阿里云镜像仓库 |
| `OSSRH_USERNAME` / `OSSRH_PASSWORD` | Maven Central (Sonatype OSSRH) |
| `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` | Maven GPG 签名 |
| `NPM_TOKEN` | npm 发布 |
| `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` | Docker Hub（可选） |

## 通用环境变量

```yaml
env:
  REGISTRY: registry.cn-hangzhou.aliyuncs.com
  REGISTRY_NAMESPACE: structured
```

## 模板 1：build-and-push.yml

> 用于构建前后端 Docker 镜像并推送到 ACR。

```yaml
name: build-and-push

on:
  workflow_dispatch:
    inputs:
      module:
        description: '模块名（如 user / org / iam）'
        required: true
      version:
        description: '版本号（如 1.2.0）'
        required: true

env:
  REGISTRY: registry.cn-hangzhou.aliyuncs.com
  REGISTRY_NAMESPACE: structured

jobs:
  # 后端构建
  build-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-java@v4
        with:
          distribution: temurin
          java-version: 17

      # Maven 缓存
      - uses: actions/cache@v4
        with:
          path: ~/.m2/repository
          key: ${{ runner.os }}-m2-${{ inputs.module }}-${{ hashFiles('**/pom.xml') }}
          restore-keys: |
            ${{ runner.os }}-m2-${{ inputs.module }}-
            ${{ runner.os }}-m2-

      # 多模块先 install dependencies
      - name: Install dependencies (multi-module)
        run: |
          cd structure-${{ inputs.module }}-center/structure-${{ inputs.module }}-dependencies
          mvn clean install -DskipTests

      # 打包 boot 模块
      - name: Package boot module
        run: |
          cd structure-${{ inputs.module }}-center/structure-${{ inputs.module }}/structure-${{ inputs.module }}-boot
          mvn clean package -DskipTests

      # 登录 ACR
      - name: Login to ACR
        run: |
          echo "${{ secrets.ALIYUN_ACR_PASSWORD }}" | docker login \
            --username="${{ secrets.ALIYUN_ACR_USERNAME }}" \
            --password-stdin ${{ env.REGISTRY }}

      # 构建并推送镜像
      - name: Build and push
        run: |
          IMAGE=${{ env.REGISTRY }}/${{ env.REGISTRY_NAMESPACE }}/${{ inputs.module }}-service
          cd structure-${{ inputs.module }}-center/structure-${{ inputs.module }}/structure-${{ inputs.module }}-boot
          docker build -t $IMAGE:${{ inputs.version }} -t $IMAGE:latest .
          docker push $IMAGE:${{ inputs.version }}
          docker push $IMAGE:latest

  # 前端构建
  build-frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 20

      # npm 缓存
      - uses: actions/cache@v4
        with:
          path: ~/.npm
          key: ${{ runner.os }}-npm-${{ inputs.module }}-${{ hashFiles('**/package-lock.json') }}

      - name: Build frontend
        run: |
          cd structure-${{ inputs.module }}-center/structure-${{ inputs.module }}-ui
          npm ci
          npm run build

      - name: Login to ACR
        run: |
          echo "${{ secrets.ALIYUN_ACR_PASSWORD }}" | docker login \
            --username="${{ secrets.ALIYUN_ACR_USERNAME }}" \
            --password-stdin ${{ env.REGISTRY }}

      - name: Build and push
        run: |
          IMAGE=${{ env.REGISTRY }}/${{ env.REGISTRY_NAMESPACE }}/${{ inputs.module }}-ui
          cd structure-${{ inputs.module }}-center/structure-${{ inputs.module }}-ui
          docker build -t $IMAGE:${{ inputs.version }} -t $IMAGE:latest .
          docker push $IMAGE:${{ inputs.version }}
          docker push $IMAGE:latest
```

**关键点**：
- ✅ Maven / npm 缓存（加速构建）
- ✅ 镜像打两个 tag：`version` + `latest`
- ✅ 多模块项目先 `mvn install dependencies`，再 `mvn package boot`

## 模板 2：release-maven.yml

> 用于发布 Java 包到 Maven Central。

```yaml
name: release-maven

on:
  workflow_dispatch:
    inputs:
      module:
        description: '模块名（如 boot / infra / security）'
        required: true
      version:
        description: '版本号（如 1.4.4）'
        required: true

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-java@v4
        with:
          distribution: temurin
          java-version: 17
          server-id: oss
          server-username: OSSRH_USERNAME
          server-password: OSSRH_PASSWORD
          gpg-private-key: ${{ secrets.GPG_PRIVATE_KEY }}
          gpg-passphrase: GPG_PASSPHRASE

      - name: Release to Maven Central
        run: |
          cd structure-${{ inputs.module }}/structure-${{ inputs.module }}-dependencies
          mvn clean deploy -P release,oss -Drevision=${{ inputs.version }}
        env:
          OSSRH_USERNAME: ${{ secrets.OSSRH_USERNAME }}
          OSSRH_PASSWORD: ${{ secrets.OSSRH_PASSWORD }}
          GPG_PASSPHRASE: ${{ secrets.GPG_PASSPHRASE }}
```

**关键点**：
- ✅ 使用 `release,oss` 双 profile
- ✅ 使用 `-Drevision=` 属性化版本
- ✅ 需要 GPG 签名

## 模板 3：publish-npm.yml

> 用于发布 npm 包到 npmjs.com。

```yaml
name: publish-npm

on:
  workflow_dispatch:
    inputs:
      component:
        description: '组件名（如 components / gateway-client）'
        required: true
      version:
        description: '版本号（如 1.0.0）'
        required: true

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 20
          registry-url: https://registry.npmjs.org

      - name: Verify package
        run: |
          cd structure-${{ inputs.component }}
          # 禁止发布 private 包
          if [ "$(jq -r '.private' package.json)" = "true" ]; then
            echo "❌ 业务包 private:true 不允许发布"
            exit 1
          fi
          # 校验 scope
          if [[ "$(jq -r '.name' package.json)" != @structure-projects/* ]]; then
            echo "❌ 包名 MUST 使用 @structure-projects scope"
            exit 1
          fi

      - name: Build
        run: |
          cd structure-${{ inputs.component }}
          npm ci
          npm run build

      - name: Publish
        run: |
          cd structure-${{ inputs.component }}
          npm version ${{ inputs.version }} --no-git-tag-version
          npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

**关键点**：
- ✅ 校验 `private != true`（业务包不可发布）
- ✅ 校验 scope MUST 为 `@structure-projects`
- ✅ 使用 `--access public`

## 缓存策略

### Maven 缓存

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.m2/repository
    key: ${{ runner.os }}-m2-${{ env.MODULE }}-${{ hashFiles('**/pom.xml') }}
    restore-keys: |
      ${{ runner.os }}-m2-${{ env.MODULE }}-
      ${{ runner.os }}-m2-
```

### npm 缓存

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.npm
    key: ${{ runner.os }}-npm-${{ hashFiles('**/package-lock.json') }}
```

## 轻量版：release.yml（tag 触发）

> 仅用于小型项目，tag 推送自动构建。

```yaml
name: release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # 构建 + 推送 Docker

      # 创建 GitHub Release
      - uses: softprops/action-gh-release@v1
        with:
          generate_release_notes: true
```

**适用**：`structure-mono-template` 等轻量项目。

## 禁止事项

- ❌ **MUST NOT** 使用 `on: release: published` 自动触发发布
- ❌ **MUST NOT** 在 CI 中硬编码 Secrets
- ❌ **MUST NOT** 跳过测试阶段直接发布
- ❌ **MUST NOT** 发布业务包到 npm（`private: true` 拦截）

## 关联

- 技能：`ci-pipeline` / `ci-gate`
- Wiki：`wiki/_common/github-workflow.md` / `wiki/_common/docker.md`
- 参考实现：`structure-iam/.github/workflows/`
