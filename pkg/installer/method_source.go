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
	"path/filepath"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// installSource 实现 method: source —— 把源码归档解到 {{.SrcDir}}，在那里执行 build:。
//
// 不猜构建方式：不去试 ./configure / make / cargo / go build。
// 猜错的代价是"报告装好了、其实什么都没编出来"，而 build: 写两行就说清了。
// 归档里通常有一层顶层目录，需要的话在 build: 里自己 cd —— 显式好过 --strip-components 的隐式假设。
func installSource(res types.Resource, artifacts []types.DownloadResult) error {
	if len(artifacts) == 0 {
		return fmt.Errorf("method: source 需要 urls: 指明源码归档")
	}
	if len(res.Build) == 0 {
		return fmt.Errorf("method: source 需要 build: 指明构建命令（somcli 不猜构建方式）")
	}

	srcDir := filepath.Join(utils.GetDownloadDir(), res.Name, res.Version, "src")
	cmds := []string{
		fmt.Sprintf("rm -rf %s && mkdir -p %s", shellQuote(srcDir), shellQuote(srcDir)),
	}

	for _, art := range artifacts {
		extract := archiveExtractCmd(art.LocalPath, srcDir)
		if extract == "" {
			return fmt.Errorf("method: source 的 %s 不是可识别的归档（支持 tar / tar.gz / tgz / tar.bz2 / tar.xz / zip）",
				art.LocalPath)
		}
		cmds = append(cmds, extract)
	}

	for _, step := range res.Build {
		// 不在这里渲染模板：RunScripts 会对每条命令做一次 ParseStr，
		// 提前渲染等于渲染两遍，命令里出现 {{ 的场合就会出错。
		// 每条 build 命令都自带 cd：RunScripts 每条命令起一个新 sh，上一条的 cd 不留给下一条。
		cmds = append(cmds, fmt.Sprintf("cd %s && %s", shellQuote(srcDir), step))
	}

	return runMethodScripts(res, cmds)
}
