---
name: ci-pipeline
description: |
  当用户要求"写流水线/加 CI/CD/GitHub Actions/workflow"、"Jenkins 流水线/Jenkinsfile"、"云效流水线/阿里云效"
  或"提 PR/创建 PR/请求评审/合并 PR/关闭 PR"时触发。
  CI 协作全流程：按平台生成流水线（GitHub Actions / Jenkins / 云效）→ 用 gh CLI 走 PR 评审与合并。
  发布 MUST 手动触发，生产部署 MUST 走人工审批节点；PR 合并方式与合并操作 MUST 用户确认。

triggers:
  - 写流水线
  - 加 CI
  - 加 CD
  - ci/cd
  - 构建流水线
  - 发布流水线
  - GitHub Actions
  - workflow
  - 加 workflow
  - Jenkins
  - Jenkinsfile
  - Jenkins 流水线
  - jenkins pipeline
  - 云效
  - 阿里云效
  - 云效流水线
  - yunxiao
  - 提 PR
  - 创建 PR
  - 请求评审
  - 合并 PR
  - 关闭 PR
  - 发起合并
  - gh pr
  - pull request
  - new pr
  - create pr

role: devops
phase: ci

allowed-tools: Bash, Read, Write, Edit

related-rules:
  - common-core
  - common-delivery
  - common-project-stack-detection

reads-before-action:
  - wiki/_common/ci-cd-pipeline.md
  - wiki/_common/github-workflow.md
  - wiki/_common/git-workflow.md
  - wiki/_common/docker.md

produces:
  - .github/workflows/*.yml（GitHub Actions）/ Jenkinsfile / .yunxiao/pipeline.yml
  - Secrets / 凭据 / 变量配置说明
  - 创建的 PR（含完整描述）+ 评审通过记录
  - 合并后的 develop、清理的远程分支

requires:
  - skill: infra-ops
    condition: 编写流水线（第一章）时 Dockerfile 已存在
    error: 无 Dockerfile，MUST 先调用 infra-ops
  - skill: ci-gate
    condition: 提 PR（第二章）时本地 CI 通过
    error: 本地 CI 未通过，MUST 先完成 ci-gate
  - skill: expert-review
    condition: 提 PR（第二章）时 review.md 存在且无 MUST fix
    error: 评审未完成，MUST 先完成 expert-review

human-in-the-loop:
  - id: select-ci-platform
    action: CI 平台（GitHub Actions / Jenkins / 云效）与项目类型 MUST 询问用户
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 平台判断错误只影响未提交的流水线文件，可重写
  - id: confirm-credentials
    action: Secrets / 凭据 / 镜像仓库 / npm scope 配置 MUST 用户确认
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-production-deploy-stage
    action: 流水线中的生产部署阶段 MUST 保留人工审批节点，MUST NOT 改为自动
    level: L0-never-skip
    autonomous:
      behavior: block
      escalate: human-queue
      alert: true
    rollback: N/A
  - id: confirm-pr-content
    action: PR 标题与描述 MUST 用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: PR 标题 / 描述可编辑，无外部副作用
  - id: confirm-merge
    action: 合并 PR 与合并方式（squash / merge / rebase）MUST 用户确认
    level: L1-auto-decidable
    autonomous:
      behavior: ai-infer
      audit: true
    rollback: 合并到 develop 可 revert；合并到 master 按 L0 处理，MUST 人工

on-failure: |
  流水线失败 → 读运行日志定位，修复后重跑
  Secrets 缺失 → 引导用户配置，MUST NOT 把凭据写进流水线文件
  yaml 语法错误 → yamllint / helm lint 类工具本地校验后再提交
  PR CI 失败 → 修复后重试，MUST NOT 强制合并
  评审有 MUST fix → 调用 review-fix-loop
  分支冲突 → 先 rebase / merge 解决冲突，MUST NOT force push 公共分支

category: ci
stack: _common
priority: high
---

# CI/CD 流水线与 PR 协作

> 按平台生成流水线 → 用 `gh` CLI 走 PR 评审与合并。
> **发布 MUST 手动触发；生产部署 MUST 保留人工审批节点（L0）**。

按任务选章：写流水线 → 一；提 / 合 PR → 二。

---

# 一、流水线编写

前置：项目已有 Dockerfile（`infra-ops`）。MUST Read：`wiki/_common/ci-cd-pipeline.md`。

## 第 0 步：选平台与项目类型（MUST 询问用户）

```
Q1: CI 平台？
    a) GitHub Actions（生态默认）
    b) Jenkins
    c) 阿里云效

Q2: 项目类型？
    a) 后端 Java（build-and-push + release-maven）
    b) 前端（build-and-push）
    c) npm 组件库（publish-npm）
    d) 全栈（三件套都要）

Q3: 镜像仓库？
    默认：registry.cn-hangzhou.aliyuncs.com/structured

Q4: 是否发布到 Maven Central / npmjs？
```

## 跨平台通用红线（MUST 遵守）

- ❌ **MUST NOT** 用 `on: release: published` 之类事件自动触发**发布**
- ✅ **MUST** 发布走手动触发（GitHub `workflow_dispatch` / Jenkins 参数化构建 / 云效手动运行）
- ✅ **MUST** 生产部署前有人工审批节点（GitHub Environments / Jenkins `input` / 云效 `ManualApproval`）
- ✅ **MUST** 凭据走平台密钥管理（`secrets` / `withCredentials` / 云效变量），**MUST NOT** 写进流水线文件
- ✅ **MUST** 镜像打 `<version>` + `latest` 双 tag
- ✅ **MUST** 用依赖缓存（Maven / npm）加速构建
- ✅ **MUST** npm 发布前校验 scope 与 `private` 字段

## 1.1 GitHub Actions（生态默认）

### 生成三件套

按 `wiki/_common/ci-cd-pipeline.md` 的三件套模板生成：

| 文件 | 适用 |
|---|---|
| `.github/workflows/build-and-push.yml` | 所有项目（构建 + 推镜像） |
| `.github/workflows/release-maven.yml` | Java 项目（发 Maven Central） |
| `.github/workflows/publish-npm.yml` | npm 组件库 |

关键替换：`structure-${{ inputs.module }}` / `structure-${{ inputs.component }}` → 实际路径；Secrets 名称保持生态约定不变。

### 配置 Secrets

```bash
gh secret set ALIYUN_ACR_USERNAME --body "..."
gh secret set ALIYUN_ACR_PASSWORD --body "..."
gh secret set OSSRH_USERNAME      --body "..."   # Java
gh secret set OSSRH_PASSWORD      --body "..."   # Java
gh secret set GPG_PRIVATE_KEY     --body "..."   # Java
gh secret set GPG_PASSPHRASE      --body "..."   # Java
gh secret set NPM_TOKEN           --body "..."   # npm
```

### 验证与监控

```bash
yamllint .github/workflows/*.yml

git add .github/workflows/ && git commit -m "ci(workflows): 新增三件套流水线" && git push

gh workflow run build-and-push.yml -f module=<module> -f version=<version>
gh run list
gh run view <run-id> --log
```

## 1.2 Jenkins（声明式）

### 后端模板（Java / Spring Boot）

```groovy
pipeline {
    agent any

    tools {
        jdk 'jdk17'
        maven 'maven3.9'
    }

    environment {
        REGISTRY  = 'registry.cn-hangzhou.aliyuncs.com'
        NAMESPACE = 'structured'
        IMAGE     = "${REGISTRY}/${NAMESPACE}/${JOB_NAME}"
        VERSION   = "${env.BRANCH_NAME}-${env.BUILD_NUMBER}"
    }

    options {
        timestamps()
        disableConcurrentBuilds()
        buildDiscarder(logRotator(numToKeepStr: '10'))
    }

    stages {
        stage('Checkout') { steps { checkout scm } }

        stage('Test') {
            steps { sh 'mvn clean test' }
            post {
                always {
                    junit '**/target/surefire-reports/*.xml'
                    jacoco()
                }
            }
        }

        stage('Package')     { steps { sh 'mvn clean package -DskipTests' } }
        stage('Build Image') { steps { sh "docker build -t ${IMAGE}:${VERSION} -t ${IMAGE}:latest ." } }

        stage('Push Image') {
            steps {
                withCredentials([usernamePassword(
                    credentialsId: 'aliyun-acr',
                    usernameVariable: 'USERNAME',
                    passwordVariable: 'PASSWORD'
                )]) {
                    sh 'echo $PASSWORD | docker login --username=$USERNAME --password-stdin $REGISTRY'
                    sh "docker push ${IMAGE}:${VERSION}"
                    sh "docker push ${IMAGE}:latest"
                }
            }
        }

        stage('Deploy to Prod') {
            // L0：生产部署 MUST 人工确认，MUST NOT 删除本 input 块
            input {
                message "Deploy to production?"
                ok "Deploy"
            }
            steps {
                sh "kubectl set image deployment/${JOB_NAME} ${JOB_NAME}=${IMAGE}:${VERSION} -n prod"
                sh "kubectl rollout status deployment/${JOB_NAME} -n prod"
            }
        }
    }

    post {
        success { echo '✓ Pipeline succeeded' }
        failure { echo '✗ Pipeline failed' }
        always  { cleanWs() }
    }
}
```

### 前端模板（Vue / React）

与后端同构，差异在 `tools { nodejs 'node20' }`，以及构建阶段：

```groovy
stage('Install')   { steps { sh 'npm ci' } }
stage('Lint + Test') {
    parallel {
        stage('Lint') { steps { sh 'npm run lint' } }
        stage('Test') { steps { sh 'npm run test' } }
    }
}
stage('Build')     { steps { sh 'npm run build' } }
```

### Jenkins 专项约定

- ✅ **MUST** 用声明式（`pipeline { ... }`）而非脚本式
- ✅ **MUST** 生产部署用 `input` 步骤确认
- ✅ **MUST** 用 `withCredentials` 管理凭据
- ✅ **MUST** 用 `post` 块做清理
- ✅ **MUST** `disableConcurrentBuilds` 防并发
- ✅ **MUST** `buildDiscarder` 限制历史保留

## 1.3 阿里云效

### 后端模板（Java + Docker + K8s）

```yaml
# .yunxiao/pipeline.yml
version: '1.0'
name: user-service-pipeline

stages:
  - name: build
    displayName: 构建
    jobs:
      - name: maven-build
        component: MavenBuild
        inputs:
          jdkVersion: '17'
          mavenVersion: '3.9'
          buildCommand: |
            mvn clean package -DskipTests
          artifactPath: target/*.jar

  - name: docker
    displayName: Docker 镜像
    jobs:
      - name: docker-build
        component: DockerBuild
        inputs:
          dockerfile: Dockerfile
          registry: registry.cn-hangzhou.aliyuncs.com
          namespace: structured
          imageName: user-service
          imageTag: ${CI_COMMIT_REF_NAME}-${CI_BUILD_NUMBER}
          username: ${DOCKER_USERNAME}
          password: ${DOCKER_PASSWORD}

  - name: deploy-test
    displayName: 部署测试环境
    jobs:
      - name: k8s-deploy-test
        component: KubernetesDeploy
        inputs:
          namespace: test
          deployment: user-service
          image: registry.cn-hangzhou.aliyuncs.com/structured/user-service:${CI_COMMIT_REF_NAME}-${CI_BUILD_NUMBER}

  # L0：生产审批节点，MUST NOT 删除
  - name: approval
    displayName: 生产审批
    jobs:
      - name: manual-approval
        component: ManualApproval
        inputs:
          approvers: ['<user1>', '<user2>']
          message: 是否部署到生产环境？

  - name: deploy-prod
    displayName: 部署生产环境
    jobs:
      - name: k8s-deploy-prod
        component: KubernetesDeploy
        inputs:
          namespace: prod
          deployment: user-service
          image: registry.cn-hangzhou.aliyuncs.com/structured/user-service:${CI_COMMIT_REF_NAME}-${CI_BUILD_NUMBER}
```

### 前端差异

`build` 阶段换成 `NodeBuild`：

```yaml
      - name: node-build
        component: NodeBuild
        inputs:
          nodeVersion: '20'
          buildCommand: |
            npm ci
            npm run build
          artifactPath: dist/
```

### 云效专项约定

- ✅ **MUST** 生产部署前用 `ManualApproval` 人工审批
- ✅ **MUST** 镜像 tag 含 `${CI_COMMIT_REF_NAME}-${CI_BUILD_NUMBER}`
- ✅ **MUST** 凭据用云效变量管理，不写死

---

# 二、PR 工作流（gh CLI）

> **MUST 命令行操作（留痕），MUST NOT 用 Web 界面点合并**。

## 前置条件（MUST 全部满足）

1. **本地 CI 通过**：编译 + 测试 + lint（`ci-gate`）
2. **评审通过**：`review.md` 存在，无未解决 MUST fix（`expert-review`）
3. **分支正确**：当前分支为 `feat-*` / `fix-*` / `hotfix-*`
4. **已推远程**：`git push -u origin feat-<name>`

## 第 1 步：与 develop 同步

```bash
git fetch origin
git rebase origin/develop  # 或 git merge origin/develop
# 解决冲突（如有）
git push origin feat-<name>
```

## 第 2 步：创建 PR（标题与描述 MUST 用户确认）

```bash
gh pr create \
  --base develop \
  --title "feat(user): 新增用户登录接口" \
  --body "$(cat <<'EOF'
## 变更说明
<一句话说明>

## 变更类型
- [x] feat 新功能

## 关联
- Proposal: changes/proposals/<id>/
- Issue: #<number>

## 测试
- [x] 单元测试通过
- [x] 集成测试通过

## Checklist
- [x] 代码符合规范
- [x] 测试覆盖率 ≥ 80%
- [x] 文档已更新
- [x] CHANGELOG 已更新
EOF
)"
```

## 第 3 步：请求评审

```bash
gh pr request-review <number> @reviewer
gh pr view <number> --json reviews
```

## 第 4 步：处理评审意见

有 MUST fix 时：调用 `review-fix-loop` → 修复推送 → 请求复评。

```bash
git add . && git commit -m "fix: 处理评审意见 - xxx" && git push
gh pr request-review <number> @reviewer
```

## 第 5 步：检查 CI 状态

```bash
gh pr checks <number>     # 预期全部通过；失败 MUST 修复后重试
```

## 第 6 步：合并 PR

合并前置（MUST 全部满足）：CI 全部通过 + 至少 1 人评审通过 + 无未解决 MUST fix。

| 方式 | 命令 | 适用 |
|---|---|---|
| **Squash** ⭐ | `gh pr merge --squash` | 默认推荐，多 commit 压缩为 1 个 |
| **Merge** | `gh pr merge --merge` | 需保留完整历史 |
| **Rebase** | `gh pr merge --rebase` | 保持线性历史 |

```bash
# MUST 用户确认合并方式后执行
gh pr merge <number> --squash

git push origin --delete feat-<name>   # 清理远程分支
git checkout develop && git pull
```

---

## 完成标准

- 流水线文件语法校验通过，手动触发一次成功运行，产物（镜像 / 包）成功推送
- 流水线生产部署阶段存在人工审批节点
- PR 创建成功（含完整描述），评审通过，CI 通过，合并成功，远程分支已删除

## 下一步

- 流水线就绪 → 第二章走 PR 合并
- PR 合并 → `archive-change`（归档变更）→ `release-ops`（发版）

## 关联

- 前置：`infra-ops`（流水线场景）/ `ci-gate` + `expert-review`（PR 场景）
- 中途：`review-fix-loop`（如有 MUST fix）
- 后续：`archive-change` / `release-ops`
- Wiki：`wiki/_common/ci-cd-pipeline.md`、`wiki/_common/github-workflow.md`、`wiki/_common/git-workflow.md`
