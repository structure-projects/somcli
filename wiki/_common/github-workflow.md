# GitHub 工作流（gh CLI + PR + Release）

> 本文档是 structure-projects 生态的 GitHub 协作规范。
> 所有 GitHub 操作 MUST 通过 `gh` CLI 完成，禁止直接在 Web 界面操作（保持命令行可追溯）。

## 前置要求

- 安装 `gh` CLI：`brew install gh`（macOS）或参考 https://cli.github.com/
- 登录：`gh auth login`
- 验证：`gh auth status`

## gh CLI 核心命令

### PR 操作

```bash
# 创建 PR
gh pr create --base develop --title "feat(user): 新增用户登录" --body "..."

# 查看 PR 列表
gh pr list

# 查看 PR 详情
gh pr view <number>

# 查看 PR diff
gh pr diff <number>

# 请求评审
gh pr request-review <number> @reviewer

# 检出 PR 到本地
gh pr checkout <number>

# 合并 PR
gh pr merge <number> --squash   # 压缩合并（推荐）
gh pr merge <number> --merge    # 普通合并
gh pr merge <number> --rebase   # rebase 合并

# 关闭 PR
gh pr close <number>

# 查看 PR 状态（CI）
gh pr checks <number>
```

### Issue 操作

```bash
# 创建 Issue
gh issue create --title "..." --body "..."

# 查看 Issue
gh issue list
gh issue view <number>

# 关闭 Issue
gh issue close <number>
```

### Release 操作

```bash
# 创建 Release
gh release create v1.2.0 --title "v1.2.0" --notes "..."

# 从 changelog 生成 Release notes
gh release create v1.2.0 --generate-notes

# 查看 Release
gh release list
gh release view v1.2.0

# 上传附件
gh release upload v1.2.0 ./dist/app.jar
```

### Workflow 操作

```bash
# 查看 workflow
gh workflow list

# 触发 workflow
gh workflow run <name>

# 查看运行
gh run list
gh run view <run-id>

# 查看日志
gh run view <run-id> --log
```

## PR 工作流（MUST 遵守）

### 创建 PR 前检查

- ✅ 本地 CI 通过（编译 + 测试）
- ✅ commit message 符合 Conventional Commits
- ✅ 分支已从最新 develop 同步（rebase 或 merge）
- ✅ 变更提案已存在（`changes/proposals/<id>/`）

### 创建 PR

```bash
gh pr create \
  --base develop \
  --title "feat(user): 新增用户登录接口" \
  --body "$(cat <<'EOF'
## 变更说明
新增用户登录接口，包含 JWT 签发与验证。

## 关联
- Proposal: changes/proposals/2026-08-15-add-user-login/
- Issue: #123

## 测试
- [x] 单元测试通过
- [x] 集成测试通过
- [x] 本地手动验证通过

## Checklist
- [x] 代码符合规范
- [x] 测试覆盖率 ≥ 80%
- [x] 文档已更新
- [x] CHANGELOG 已更新
EOF
)"
```

### PR 描述模板（MUST 遵循）

```markdown
## 变更说明
<一句话说明本次变更>

## 变更类型
- [ ] feat 新功能
- [ ] fix 修复
- [ ] refactor 重构
- [ ] docs 文档
- [ ] test 测试
- [ ] chore 杂务

## 关联
- Proposal: changes/proposals/<id>/
- Issue: #<number>

## 测试
- [ ] 单元测试
- [ ] 集成测试
- [ ] E2E 测试
- [ ] 手动验证

## Checklist
- [ ] 代码符合规范
- [ ] 测试覆盖率 ≥ 80%
- [ ] 文档已更新（README / Wiki）
- [ ] CHANGELOG 已更新
- [ ] 无 MUST fix 评审意见

## 截图（如涉及 UI）
<贴图>
```

### 请求评审

```bash
# 请求特定人评审
gh pr request-review <number> @reviewer1 @reviewer2

# 查看评审状态
gh pr view <number> --json reviews
```

### 处理评审意见

```bash
# 检出 PR 到本地修复
gh pr checkout <number>

# 修复后提交
git add .
git commit -m "fix: 处理评审意见 - xxx"
git push

# 请求复评
gh pr request-review <number> @reviewer
```

### 合并 PR

**前置条件**（MUST 全部满足）：
- ✅ CI 全部通过（`gh pr checks <number>`）
- ✅ 至少 1 人评审通过
- ✅ 无未解决的 MUST fix 意见
- ✅ 无冲突

**合并方式选择**：

| 方式 | 命令 | 适用 |
|---|---|---|
| **Squash** ⭐ | `gh pr merge --squash` | 功能分支（多 commit 压缩为 1 个） |
| **Merge** | `gh pr merge --merge` | 需要保留完整提交历史 |
| **Rebase** | `gh pr merge --rebase` | 保持线性历史（谨慎使用） |

**推荐**：默认用 `--squash`，保持 develop 历史干净。

```bash
# 合并
gh pr merge <number> --squash

# 合并后删除远程分支
git push origin --delete feat-<name>
```

## Release 工作流

### 创建 Release

```bash
# 1. 确保 master 最新
git checkout master
git pull

# 2. 打 Tag
git tag -a v1.2.0 -m "Release v1.2.0"
git push origin v1.2.0

# 3. 创建 Release（从 changelog 生成 notes）
gh release create v1.2.0 \
  --title "v1.2.0" \
  --notes-file changes/changelog/1.2.0.md

# 或自动生成 notes
gh release create v1.2.0 --generate-notes
```

### Release 命名约定

- **格式**：`v<X.Y.Z>`（如 `v1.2.0`）
- **Tag 类型**：附注 Tag（`git tag -a`）
- **Title**：与 Tag 一致

## GitHub Secrets 管理

### 常用 Secrets

| Secret | 用途 |
|---|---|
| `ALIYUN_ACR_USERNAME` / `ALIYUN_ACR_PASSWORD` | 阿里云镜像仓库 |
| `OSSRH_USERNAME` / `OSSRH_PASSWORD` | Maven Central (Sonatype) |
| `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` | Maven GPG 签名 |
| `NPM_TOKEN` | npm 发布 |
| `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` | Docker Hub |

### 设置 Secrets

```bash
# 通过 gh CLI 设置
gh secret set NPM_TOKEN --body "<token>"

# 查看 Secrets（只显示名称）
gh secret list
```

## 禁止事项

- ❌ **MUST NOT** 直接在 Web 界面合并 PR（应用 `gh pr merge` 保持命令行留痕）
- ❌ **MUST NOT** 跳过评审直接合并
- ❌ **MUST NOT** force push 到共享分支
- ❌ **MUST NOT** 在 PR 描述中留空"变更说明"和"测试"章节
- ❌ **MUST NOT** 在 CI 未通过时合并

## 关联

- 技能：`ci-pipeline` / `release-ops` / `ci-gate`
- 规则：`common-core`
- Wiki：`wiki/_common/git-workflow.md` / `wiki/_common/version-management.md`
