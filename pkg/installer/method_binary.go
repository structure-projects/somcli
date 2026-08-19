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
	"path"
	"path/filepath"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// installBinary 实现 method: binary —— 下载物是归档就解开，然后把可执行文件装到 InstallDir。
//
// 归档内要装哪些文件由 files: 指定；留空则装归档里所有带可执行位的文件，
// 这正好覆盖 containerd / docker 这类"一个 tar 包里若干个 bin"的形态。
// 下载物不是归档时它自己就是那个可执行文件。
func installBinary(res types.Resource, artifacts []types.DownloadResult) error {
	if len(artifacts) == 0 {
		return fmt.Errorf("method: binary 需要 urls: 指明要安装的二进制或归档")
	}

	dir := utils.InstallDir(res)
	cmds := []string{fmt.Sprintf("mkdir -p %s", shellQuote(dir))}

	for _, art := range artifacts {
		extract := archiveExtractCmd(art.LocalPath, "")
		if extract == "" {
			// 裸二进制：install 一步到位建目录之外的事（拷贝 + 权限 + 原子替换）
			dest := filepath.Join(dir, filepath.Base(art.LocalPath))
			cmds = append(cmds, fmt.Sprintf("install -m 0755 %s %s",
				shellQuote(art.LocalPath), shellQuote(dest)))
			continue
		}

		stage := filepath.Join(filepath.Dir(art.LocalPath), "unpack")
		cmds = append(cmds,
			fmt.Sprintf("rm -rf %s && mkdir -p %s", shellQuote(stage), shellQuote(stage)),
			archiveExtractCmd(art.LocalPath, stage),
		)

		if len(res.Files) == 0 {
			// -perm -u+x 在 GNU find 与 BSD find 上语义一致
			cmds = append(cmds, fmt.Sprintf("find %s -type f -perm -u+x -exec install -m 0755 {} %s/ \\;",
				shellQuote(stage), shellQuote(dir)))
			continue
		}
		for _, src := range res.Files {
			// 不预渲染模板：RunScripts 会对整条命令做一次 ParseStr
			cmds = append(cmds, fmt.Sprintf("install -m 0755 %s %s",
				shellQuote(filepath.Join(stage, src)),
				shellQuote(filepath.Join(dir, path.Base(src)))))
		}
	}

	return runMethodScripts(res, cmds)
}
