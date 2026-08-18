# Maven 发布规范

> 本文档是 structure-projects 生态 Maven 包发布的参考手册。

## 发布目标

**Maven Central**（通过 Sonatype OSSRH）

## 关键配置

### distributionManagement

```xml
<distributionManagement>
  <snapshotRepository>
    <id>oss</id>
    <url>https://central.sonatype.com/repository/maven-snapshots/</url>
  </snapshotRepository>
  <repository>
    <id>oss</id>
    <url>https://central.sonatype.com/service/local/staging/deploy/maven2/</url>
  </repository>
</distributionManagement>
```

### 必要插件

| 插件 | 用途 |
|---|---|
| `nexus-staging-maven-plugin` | 发布到 Sonatype staging |
| `maven-gpg-plugin` | GPG 签名（Maven Central 必须） |
| `maven-source-plugin` | 打包 sources.jar |
| `maven-javadoc-plugin` | 打包 javadoc.jar |

## 必要 Secrets

| Secret | 用途 |
|---|---|
| `OSSRH_USERNAME` / `OSSRH_PASSWORD` | Sonatype 凭据 |
| `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` | GPG 签名 |

## 发布命令

```bash
mvn clean deploy -P release,oss -Drevision=<version>
```

**关键**：
- ✅ **MUST** 用 `release,oss` 双 profile
- ✅ **MUST** 用 `-Drevision=` 属性化版本（Maven CI Friendly Versions）

## GPG 密钥导出

```bash
# 生成密钥
gpg --gen-key

# 导出私钥（用于 GitHub Secrets）
gpg --export-secret-keys -a <key-id> > private.key

# 上传到公钥服务器
gpg --keyserver keyserver.ubuntu.com --send-keys <key-id>
```

## 发布后验证

- https://central.sonatype.com/ 查看 staging 仓库
- https://repo1.maven.org/maven2/ 查看正式仓库（同步约 30 分钟）

## 关联

- 技能：`release-ops`
- Wiki：`wiki/_common/version-management.md`
- 参考：`structure-boot/structure-dependencies/pom.xml`
