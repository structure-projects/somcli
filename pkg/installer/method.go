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

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// applyMethod 是 method 的分发入口，在 pre_install 之后、post_install 之前执行。
//
// 每种 method 都不直接动手，只把动作**编译成一串 shell 命令**，由这里统一交给
// utils.RunScriptsDetailed —— 于是"本机执行还是逐节点远程执行"这件事只有一处实现，
// 日志前缀、失败传播、--parallel 与 on_error 也全部共享。
// 各方法自己去 exec 的话，每加一种 method 就要重写一遍远程分支，
// 而那些分支没有任何用例会走到 —— 正是 E1 这类"写了但从没生效"的来源。
//
// 返回的 RunOutcome 带着逐目标的失败名单，调用方据此在 on_error: continue 下
// 把失败节点从 post_install 里剔掉（SC-F05）。
func applyMethod(res types.Resource, artifacts []types.DownloadResult) (utils.RunOutcome, error) {
	method := strings.ToLower(strings.TrimSpace(res.Method))

	var (
		cmds []string
		err  error
	)
	switch method {
	case "", "script":
		// 动作就是 pre_install / post_install 本身，这里无事可做。
		return utils.RunOutcome{}, nil
	case "binary":
		cmds, err = installBinary(res, artifacts)
	case "package":
		cmds, err = installPackage(res)
	case "container":
		cmds, err = installContainer(res)
	case "source":
		cmds, err = installSource(res, artifacts)
	case "manifest":
		cmds, err = installManifest(res, artifacts)
	default:
		// 认不出来必须报错。静默当成 script 跑正是 E1 的病症：
		// 配置里写着 method: binary，实际什么都没做，而退出码是 0。
		return utils.RunOutcome{}, fmt.Errorf("未知的 method %q，可用：script / binary / package / container / source / manifest", res.Method)
	}
	if err != nil {
		return utils.RunOutcome{}, err
	}
	return runMethodScripts(res, cmds)
}

// runMethodScripts 执行方法生成的命令，日志里标明是哪种 method 在动手。
func runMethodScripts(res types.Resource, cmds []string) (utils.RunOutcome, error) {
	if len(cmds) == 0 {
		return utils.RunOutcome{}, nil
	}
	utils.PrintStage("按 method: %s 实施 -> %s", strings.ToLower(res.Method), res.Name)
	return utils.RunScriptsDetailed(cmds, res)
}

// shellQuote 是 utils.ShellQuote 在 installer 内的快捷方式，各 method 拼 shell 时统一用它。
func shellQuote(s string) string {
	return utils.ShellQuote(s)
}

// archiveExtractCmd 返回把归档解到 dir 的命令，不是归档则返回空串。
//
// 用 shell 的 tar / unzip 而不是 Go 的 archive/tar：解压必须发生在**目标节点**上，
// 而 Go 侧解压只能解在操作机。tar 与 sh 是任何能跑 somcli 的机器都有的东西，
// 与 F5 那个 wget 不同 —— wget 是"本该由 somcli 安装的工具"，tar 不是。
func archiveExtractCmd(path, dir string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return fmt.Sprintf("unzip -o -q %s -d %s", shellQuote(path), shellQuote(dir))
	case strings.HasSuffix(lower, ".tar"),
		strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"),
		strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"),
		strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		// -xf 让 tar 自己认压缩格式，不必按后缀选 z/j/J
		return fmt.Sprintf("tar -xf %s -C %s", shellQuote(path), shellQuote(dir))
	default:
		return ""
	}
}
