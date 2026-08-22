/*
Copyright 2023 Structure Projects

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

import (
	"fmt"
	"strings"

	"github.com/structure-projects/somcli/pkg/types"
)

// 软件源模式。留空按 official 处理。
const (
	SourceOfficial = "official"
	SourceAliyun   = "aliyun"
	SourceISO      = "iso"

	defaultISOMount = "/mnt/somcli-iso"
)

// RenderSourceSetup 生成一段在目标节点上执行的 shell：在装系统包之前把软件源配好。
//
// 返回空串表示无事可做（official 或未配置）。探测与执行都在这段 shell 里发生在
// **目标节点**上，理由同 RenderPkgCommand：操作机上有什么源与目标节点无关。
// sudo 传 "sudo " 表示提权（包管理器与 /etc 写入都需要 root），传 "" 表示不提权。
func RenderSourceSetup(cfg *types.SourceConfig, sudo string) (string, error) {
	if cfg == nil {
		return "", nil
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" || mode == SourceOfficial {
		return "", nil
	}

	switch mode {
	case SourceAliyun:
		return renderAliyunSource(sudo), nil
	case SourceISO:
		iso := strings.TrimSpace(cfg.ISO)
		if iso == "" {
			return "", fmt.Errorf("source.mode=iso 需要配置 source.iso（ISO 在目标节点上的绝对路径）")
		}
		mount := strings.TrimSpace(cfg.Mount)
		if mount == "" {
			mount = defaultISOMount
		}
		return renderISOSource(sudo, iso, mount), nil
	default:
		return "", fmt.Errorf("未知的 source.mode %q，可用：official / aliyun / iso", cfg.Mode)
	}
}

// renderAliyunSource 按包管理器把官方源替换成阿里云镜像。
//
// 每个分支先 sed 改 repo 文件再刷新元数据；sed 只改主机名、保留 URI 路径，
// 这样 debian → mirrors.aliyun.com/debian、ubuntu → mirrors.aliyun.com/ubuntu 都对。
// 用 for 循环 + [ -f ] 跳过不存在的文件，不同发行版的源文件清单本就不同。
func renderAliyunSource(sudo string) string {
	var b strings.Builder
	b.WriteString("# 切换到阿里云镜像源（somcli source: aliyun）\n")
	b.WriteString("if command -v apt-get >/dev/null 2>&1; then\n")
	b.WriteString("  for f in /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do\n")
	b.WriteString("    [ -f \"$f\" ] || continue\n")
	b.WriteString("    " + sudoSed(sudo,
		`s|https\?://deb.debian.org|https://mirrors.aliyun.com|g`,
		`s|https\?://security.debian.org|https://mirrors.aliyun.com|g`,
		`s|https\?://archive.ubuntu.com|https://mirrors.aliyun.com|g`,
		`s|https\?://security.ubuntu.com|https://mirrors.aliyun.com|g`,
		`s|https\?://ports.ubuntu.com|https://mirrors.aliyun.com|g`,
	) + " \"$f\"\n")
	b.WriteString("  done\n")
	b.WriteString("  " + sudoCmd(sudo, "apt-get", "update") + "\n")
	b.WriteString("elif command -v dnf >/dev/null 2>&1; then\n")
	b.WriteString("  for f in /etc/yum.repos.d/Rocky-*.repo /etc/yum.repos.d/rocky*.repo /etc/yum.repos.d/fedora*.repo; do\n")
	b.WriteString("    [ -f \"$f\" ] || continue\n")
	b.WriteString("    " + sudoSed(sudo,
		`s|^mirrorlist=|#mirrorlist=|g`,
		`s|^#\?baseurl=https\?://dl.rockylinux.org/$contentdir|baseurl=https://mirrors.aliyun.com/rockylinux|g`,
		`s|^metalink=|#metalink=|g`,
	) + " \"$f\"\n")
	b.WriteString("  done\n")
	b.WriteString("  " + sudoCmd(sudo, "dnf", "makecache") + "\n")
	b.WriteString("elif command -v yum >/dev/null 2>&1; then\n")
	b.WriteString("  for f in /etc/yum.repos.d/CentOS-*.repo; do\n")
	b.WriteString("    [ -f \"$f\" ] || continue\n")
	b.WriteString("    " + sudoSed(sudo,
		`s|^mirrorlist=|#mirrorlist=|g`,
		// CentOS 7 已 EOL，阿里云把它归档在 centos-vault/7.9.2009，路径里的 /centos/$releasever 一并吃掉。
		`s|^#\?baseurl=https\?://mirror.centos.org/centos/\$releasever|baseurl=https://mirrors.aliyun.com/centos-vault/7.9.2009|g`,
	) + " \"$f\"\n")
	b.WriteString("  done\n")
	b.WriteString("  " + sudoCmd(sudo, "yum", "makecache", "fast") + "\n")
	b.WriteString("elif command -v zypper >/dev/null 2>&1; then\n")
	b.WriteString("  for f in /etc/zypp/repos.d/*.repo; do\n")
	b.WriteString("    [ -f \"$f\" ] || continue\n")
	b.WriteString("    " + sudoSed(sudo,
		// download.opensuse.org 后面的路径（distribution/leap/...、update/...）原样保留。
		`s|https\?://download.opensuse.org|https://mirrors.aliyun.com/opensuse|g`,
	) + " \"$f\"\n")
	b.WriteString("  done\n")
	b.WriteString("  " + sudoCmd(sudo, "zypper", "--non-interactive", "refresh") + "\n")
	b.WriteString("elif command -v apk >/dev/null 2>&1; then\n")
	b.WriteString("  " + sudoCmd(sudo, "sed", "-i",
		`s|https\?://dl-cdn.alpinelinux.org|https://mirrors.aliyun.com|g`,
		"/etc/apk/repositories") + "\n")
	b.WriteString("  " + sudoCmd(sudo, "apk", "update") + "\n")
	b.WriteString("else\n")
	b.WriteString("  echo '未找到可用的包管理器（尝试过 apt-get / dnf / yum / zypper / apk），无法换源' >&2\n")
	b.WriteString("  exit 1\n")
	b.WriteString("fi\n")
	return b.String()
}

// renderISOSource 挂载 ISO 并把它配成本地软件源。
//
// 离线场景：ISO 已在目标节点上，somcli 只挂载并写 repo 文件。yum/dnf 的 DVD 目录结构
// 在 CentOS 7（repodata 在根）与 RHEL/Rocky 8+（BaseOS/AppStream 子目录）间不同，
// 三个 baseurl 都配上、用 skip_if_unavailable=1 跳过不存在的。
// apt 走 file:// deb 源，发行版代号从 /etc/os-release 取。
func renderISOSource(sudo, iso, mount string) string {
	var b strings.Builder
	b.WriteString("# 挂载 ISO 作为本地软件源（somcli source: iso）\n")
	b.WriteString(fmt.Sprintf("ISO=%s\n", ShellQuote(iso)))
	b.WriteString(fmt.Sprintf("MOUNT=%s\n", ShellQuote(mount)))
	b.WriteString(sudoCmd(sudo, "mkdir", "-p", "\"$MOUNT\"") + "\n")
	b.WriteString("if ! mountpoint -q \"$MOUNT\" 2>/dev/null; then\n")
	b.WriteString("  " + sudoCmd(sudo, "mount", "-o", "loop,ro", "\"$ISO\"", "\"$MOUNT\"") + "\n")
	b.WriteString("fi\n")

	b.WriteString("if command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then\n")
	repo := strings.Join([]string{
		"[somcli-iso-root]",
		"name=somcli local iso (root)",
		"baseurl=file://" + mount,
		"enabled=1",
		"gpgcheck=0",
		"skip_if_unavailable=1",
		"",
		"[somcli-iso-baseos]",
		"name=somcli local iso (BaseOS)",
		"baseurl=file://" + mount + "/BaseOS",
		"enabled=1",
		"gpgcheck=0",
		"skip_if_unavailable=1",
		"",
		"[somcli-iso-appstream]",
		"name=somcli local iso (AppStream)",
		"baseurl=file://" + mount + "/AppStream",
		"enabled=1",
		"gpgcheck=0",
		"skip_if_unavailable=1",
		"",
	}, "\n")
	b.WriteString("  " + writeFile(sudo, "/etc/yum.repos.d/somcli-iso.repo", repo) + "\n")
	b.WriteString("  if command -v dnf >/dev/null 2>&1; then " + sudoCmd(sudo, "dnf", "makecache") + "; else " + sudoCmd(sudo, "yum", "makecache", "fast") + "; fi\n")
	b.WriteString("elif command -v apt-get >/dev/null 2>&1; then\n")
	b.WriteString("  . /etc/os-release\n")
	// Ubuntu 服务器 ISO 带 main/restricted；Debian 带 main。按 ID 区分，缺的组件 apt 会报错，
	// 所以只写该发行版确定有的。
	b.WriteString("  case \"$ID\" in\n")
	b.WriteString("    ubuntu) COMPONENTS=\"main restricted\" ;;\n")
	b.WriteString("    *)        COMPONENTS=\"main\" ;;\n")
	b.WriteString("  esac\n")
	// 用 printf 把 $MOUNT/$VERSION_CODENAME/$COMPONENTS 作为参数传进去，而不是把整行
	// 单引号包住：后者会把这些变量原样写进文件，apt 读到的是字面量 $VERSION_CODENAME。
	b.WriteString("  " + printfFile(sudo, "/etc/apt/sources.list.d/somcli-iso.list",
		"deb [trusted=yes] file://%s %s %s\n",
		`"$MOUNT"`, `"$VERSION_CODENAME"`, `"$COMPONENTS"`) + "\n")
	b.WriteString("  " + sudoCmd(sudo, "apt-get", "update") + "\n")
	b.WriteString("else\n")
	b.WriteString("  echo 'ISO 源目前仅支持 yum/dnf 与 apt（zypper/apk 暂不支持）' >&2\n")
	b.WriteString("  exit 1\n")
	b.WriteString("fi\n")
	return b.String()
}

// sudoCmd 拼一条可选提权的命令，参数逐个转义。
func sudoCmd(sudo, name string, args ...string) string {
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		// 调用方传进来的 "$f" / "$MOUNT" 是要在远端 shell 里展开的变量，不能被单引号包住。
		if strings.HasPrefix(a, "\"$") && strings.HasSuffix(a, "\"") {
			quoted = append(quoted, a)
			continue
		}
		quoted = append(quoted, ShellQuote(a))
	}
	return sudo + name + " " + strings.Join(quoted, " ")
}

// sudoSed 拼一条 sed -i -e '...' 命令，多个 -e 逐个给出。
func sudoSed(sudo string, expressions ...string) string {
	parts := []string{sudo + "sed", "-i"}
	for _, e := range expressions {
		parts = append(parts, "-e", ShellQuote(e))
	}
	return strings.Join(parts, " ")
}

// writeFile 用 root 权限把多行内容写入 path（/etc 下需要）。
// 走 sudo tee 而非 sudo sh -c '... >'：重定向由当前 shell 完成时不带 root，
// 只有 tee 以 root 打开文件才能写进 root 所有的目录。
func writeFile(sudo, path, content string) string {
	return fmt.Sprintf("printf '%%s\n' %s | %stee %s >/dev/null",
		ShellQuote(content), sudo, ShellQuote(path))
}

// printfFile 像 writeFile 那样用 sudo tee 写文件，但内容由 printf 按 format 渲染，
// args 是要在**远端 shell** 里展开的变量（形如 "$MOUNT"），不能被单引号包住。
// 用于 apt 源这类行里带运行时变量（$VERSION_CODENAME 等）的场景。
func printfFile(sudo, path, format string, args ...string) string {
	quoted := append([]string{ShellQuote(format)}, args...)
	return fmt.Sprintf("printf %s | %stee %s >/dev/null",
		strings.Join(quoted, " "), sudo, ShellQuote(path))
}
