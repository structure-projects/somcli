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
	"os"
	"path/filepath"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// extraFilesMode 是附加文件的权限。
// 0644 覆盖了这个字段的全部现有用法（daemon.json、containerd.service 这类配置文件）；
// 需要可执行的场合写 method: binary，或在 post_install 里 chmod。
const extraFilesMode = 0644

// writeExtraFiles 把 extra_files 渲染落盘（D3）。
//
// 路径与内容都过模板：路径里写 {{.Version}}、内容里写 {{.Vars.registry}} 都是真实需求。
// 先在缓存目录里落一份"暂存件"，再用 install 放到最终位置 —— 这样声明了 hosts 的资源
// 走的是与下载产物完全相同的分发路径（拷到节点上的同一个绝对路径，再 install 就位），
// 不需要为远程另写一套写文件的逻辑。
//
// 内容不经 shell：暂存件是 Go 直接写的。用 heredoc 把内容塞进命令的话，
// JSON 里的引号、$、反引号都会变成雷。
func writeExtraFiles(res types.Resource, hosts []types.RemoteNode) error {
	if len(res.ExtraFiles) == 0 {
		return nil
	}
	utils.PrintStage("生成附加文件 -> %s", res.Name)

	stageDir := filepath.Join(utils.GetDownloadDir(), res.Name, res.Version, "extra")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return fmt.Errorf("创建附加文件暂存目录 %s 失败: %w", stageDir, err)
	}

	for rawPath, rawContent := range res.ExtraFiles {
		dest, err := utils.ParseStr(rawPath, res)
		if err != nil {
			return fmt.Errorf("附加文件路径模板解析失败 (%s): %w", rawPath, err)
		}
		content, err := utils.ParseStr(rawContent, res)
		if err != nil {
			return fmt.Errorf("附加文件 %s 的内容模板解析失败: %w", dest, err)
		}

		// 暂存件名按目标路径压平，避免两个 extra_files 撞名
		staged := filepath.Join(stageDir, flattenPath(dest))
		if err := os.WriteFile(staged, []byte(content), extraFilesMode); err != nil {
			return fmt.Errorf("写附加文件暂存件 %s 失败: %w", staged, err)
		}

		for _, node := range hosts {
			if node.IP == utils.LocalNodeIP {
				continue
			}
			if err := utils.CopyToRemote(node.User, node.IP, node.SSHKey, staged, staged); err != nil {
				return fmt.Errorf("分发附加文件 %s 到节点 %s 失败: %w", dest, node.IP, err)
			}
		}

		// install -D 是 GNU 扩展，BSD 没有，所以父目录单独 mkdir
		cmds := []string{
			fmt.Sprintf("mkdir -p %s", shellQuote(filepath.Dir(dest))),
			fmt.Sprintf("install -m %04o %s %s", extraFilesMode, shellQuote(staged), shellQuote(dest)),
		}
		if err := utils.RunScripts(cmds, res); err != nil {
			return fmt.Errorf("放置附加文件 %s 失败: %w", dest, err)
		}
		utils.PrintInfo("附加文件就位 -> %s", dest)
	}
	return nil
}

// flattenPath 把 /etc/docker/daemon.json 变成 etc_docker_daemon.json。
func flattenPath(p string) string {
	flat := []rune(filepath.ToSlash(p))
	for i, r := range flat {
		if r == '/' || r == ':' || r == '\\' {
			flat[i] = '_'
		}
	}
	return string(flat)
}
