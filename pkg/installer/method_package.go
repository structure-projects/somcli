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
package installer

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/types"
)

// packageManagers 按优先级排列的包管理器与其"装一个包"的写法。
//
// 探测写在生成的 shell 里而不是 Go 里：装包要发生在**目标节点**上，
// 操作机上有什么包管理器与目标节点无关。在 Go 侧用 exec.LookPath 探测，
// 一到远程就是错的 —— 而这条错误路径在单机用例里永远不会暴露。
//
// env 从命令里拆出来是为了 sudo：sudo VAR=v cmd 不是合法写法（sudo 会把 VAR=v 当成
// 要执行的命令），VAR=v sudo cmd 又会被 sudo 的环境清洗丢掉。两者都要时只能写
// sudo env VAR=v cmd。
var packageManagers = []struct {
	bin  string
	env  string
	args string
}{
	{"apt-get", "DEBIAN_FRONTEND=noninteractive", "apt-get install -y %s"},
	{"dnf", "", "dnf install -y %s"},
	{"yum", "", "yum install -y %s"},
	{"zypper", "", "zypper --non-interactive install %s"},
	{"apk", "", "apk add --no-cache %s"},
	{"brew", "", "brew install %s"},
}

// permissionHint 是装包失败且执行者不是 root 时补的一句提示。
const permissionHint = "安装失败。若是权限不足，请加 --sudo 或以 root 运行"

// installPackage 实现 method: package —— 交给目标机器上的发行版包管理器。
func installPackage(res types.Resource) ([]string, error) {
	pkg := res.Package
	if pkg == "" {
		pkg = res.Name
	}
	if pkg == "" {
		return nil, fmt.Errorf("method: package 需要 package: 或 name: 指明包名")
	}

	// 默认不提权。非 root 就自动加 sudo 等于用户没要求提权却被提权了，
	// 远程节点上更难预料 —— 要不要提权永远由 --sudo 说。
	sudo := ""
	if viper.GetBool("sudo") {
		sudo = "sudo "
	}

	var b strings.Builder
	for i, pm := range packageManagers {
		if i > 0 {
			b.WriteString("el")
		}
		fmt.Fprintf(&b, "if command -v %s >/dev/null 2>&1; then %s; ",
			pm.bin, withPermissionHint(pkgCommand(pm.env, pm.args, pkg, sudo), sudo))
	}
	// 一个都没有就必须失败：默认"装上了"而实际什么都没发生是 E1 的病症
	fmt.Fprintf(&b, "else echo '未找到可用的包管理器（尝试过 %s）' >&2; exit 1; fi",
		strings.Join(managerNames(), " / "))

	return []string{b.String()}, nil
}

// pkgCommand 拼出一条装包命令：前置环境变量、可选的 sudo、包管理器本身。
func pkgCommand(env, args, pkg, sudo string) string {
	cmd := fmt.Sprintf(args, shellQuote(pkg))
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

// withPermissionHint 在装包失败时补一句提权提示，并原样传回包管理器的退出码。
//
// 身份用 id -u 在生成的 shell 里判：要看的是**目标节点上**执行者的身份，
// 操作机上跑 somcli 的是谁无关。已经带 sudo 时不提示 —— 那时失败多半不是权限问题。
func withPermissionHint(cmd, sudo string) string {
	if sudo != "" {
		return cmd
	}
	return fmt.Sprintf(`%s || { rc=$?; [ "$(id -u)" -ne 0 ] && echo %s >&2; exit $rc; }`,
		cmd, shellQuote(permissionHint))
}

func managerNames() []string {
	names := make([]string, 0, len(packageManagers))
	for _, pm := range packageManagers {
		names = append(names, pm.bin)
	}
	return names
}
