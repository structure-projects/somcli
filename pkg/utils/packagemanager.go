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
)

// PkgOp 标识对目标节点包管理器执行的操作种类。
type PkgOp int

const (
	// PkgInstall 安装包。
	PkgInstall PkgOp = iota
	// PkgRemove 卸载包。
	PkgRemove
	// PkgUpdate 刷新软件源元数据（apt-get update / dnf makecache 之类），不带包名。
	PkgUpdate
	// PkgQuery 查询包是否已安装，已安装退 0。
	PkgQuery
)

// pkgManager 描述一个包管理器的命令写法。install/remove/query 里的 %s 留给包名
// （会被 ShellQuote 转义），update 不带包名。
//
// 探测写在生成的 shell 里而不是 Go 里：装包发生在**目标节点**上，操作机上
// 有什么包管理器与目标节点无关。在 Go 侧用 exec.LookPath 探测，一到远程就是错的
// —— 而这条错误路径在单机用例里永远不会暴露。
//
// env 从命令里拆出来是为了 sudo：sudo VAR=v cmd 不是合法写法（sudo 会把 VAR=v
// 当成要执行的命令），VAR=v sudo cmd 又会被 sudo 的环境清洗丢掉。两者都要时只能
// 写 sudo env VAR=v cmd。
type pkgManager struct {
	bin     string
	env     string
	install string
	remove  string
	update  string
	query   string
}

// packageManagers 按优先级排列：同一台机器上可能同时存在多个（apt-get 与 brew 并存），
// 排在前面的先被选中。yum 排在 dnf 之后是因为新装的 RHEL 系两者都有，dnf 才是现役。
var packageManagers = []pkgManager{
	{"apt-get", "DEBIAN_FRONTEND=noninteractive",
		"apt-get install -y %s", "apt-get remove -y %s", "apt-get update", "dpkg-query -W %s"},
	{"dnf", "",
		"dnf install -y %s", "dnf remove -y %s", "dnf makecache", "rpm -q %s"},
	{"yum", "",
		"yum install -y %s", "yum remove -y %s", "yum makecache", "rpm -q %s"},
	{"zypper", "",
		"zypper --non-interactive install %s", "zypper --non-interactive remove %s",
		"zypper refresh", "rpm -q %s"},
	{"apk", "",
		"apk add --no-cache %s", "apk del --no-cache %s", "apk update", "apk info -e %s"},
	{"brew", "",
		"brew install %s", "brew uninstall %s", "brew update", "brew list --versions %s"},
}

// permissionHint 是改包操作失败且执行者不是 root 时补的一句提示。
const permissionHint = "命令失败。若是权限不足，请加 --sudo 或以 root 运行"

// RenderPkgCommand 生成一段在目标节点上执行的 shell：按优先级逐个 command -v 探测，
// 命中哪个包管理器就用它执行指定操作；一个都没命中则报错退出。
//
// sudo 传 "sudo " 表示提权，传 "" 表示不提权。包名会被单引号转义。
// 探测、身份判断都发生在目标节点上（见 pkgManager 注释），所以这段 shell 可以
// 直接经 ssh 下发，不需要操作机与目标节点同发行版。
func RenderPkgCommand(op PkgOp, pkg, sudo string) string {
	var b strings.Builder
	for i, pm := range packageManagers {
		if i > 0 {
			b.WriteString("el")
		}
		cmd := pm.command(op, pkg)
		// query 是只读探测，失败是正常结果（没装），不该附权限提示。
		if op != PkgQuery {
			cmd = withPermissionHint(pkgCommand(pm.env, cmd, sudo), sudo)
		} else {
			cmd = pkgCommand(pm.env, cmd, sudo)
		}
		fmt.Fprintf(&b, "if command -v %s >/dev/null 2>&1; then %s; ", pm.bin, cmd)
	}
	// 一个都没有就必须失败：默认"装上了"而实际什么都没发生是 E1 的病症。
	fmt.Fprintf(&b, "else echo '未找到可用的包管理器（尝试过 %s）' >&2; exit 1; fi",
		strings.Join(pkgManagerNames(), " / "))
	return b.String()
}

func (pm pkgManager) command(op PkgOp, pkg string) string {
	switch op {
	case PkgInstall:
		return fmt.Sprintf(pm.install, ShellQuote(pkg))
	case PkgRemove:
		return fmt.Sprintf(pm.remove, ShellQuote(pkg))
	case PkgUpdate:
		return pm.update
	case PkgQuery:
		return fmt.Sprintf(pm.query, ShellQuote(pkg))
	default:
		return ""
	}
}

// pkgCommand 拼出一条包管理器命令：前置环境变量、可选的 sudo、命令本身。
func pkgCommand(env, cmd, sudo string) string {
	switch {
	case sudo == "" && env == "":
		return cmd
	case sudo == "":
		return env + " " + cmd
	case env == "":
		return sudo + cmd
	default:
		return sudo + "env " + env + " " + cmd
	}
}

// withPermissionHint 在命令失败且执行者不是 root 时补一句提权提示，并原样传回退出码。
//
// 身份用 id -u 在生成的 shell 里判：要看的是**目标节点上**执行者的身份，操作机上
// 跑 somcli 的是谁无关。已经带 sudo 时不提示 —— 那时失败多半不是权限问题。
func withPermissionHint(cmd, sudo string) string {
	if sudo != "" {
		return cmd
	}
	return fmt.Sprintf(`%s || { rc=$?; [ "$(id -u)" -ne 0 ] && echo %s >&2; exit $rc; }`,
		cmd, ShellQuote(permissionHint))
}

func pkgManagerNames() []string {
	names := make([]string, 0, len(packageManagers))
	for _, pm := range packageManagers {
		names = append(names, pm.bin)
	}
	return names
}
