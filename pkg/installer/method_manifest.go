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
	"sort"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// installManifest 实现 method: manifest —— 把清单交给 kubectl apply。
//
// 清单来自两处，都用同一条路径落到目标节点上：
//   - urls: 下载下来的文件（已由 prepareFiles 拷到节点的同一个绝对路径）；
//   - extra_files: 内容直接写在配置里的清单（已由 writeExtraFiles 放到目标路径）。
//
// 与其他 method 一样，这里只生成命令，不自己去 exec：谁在哪台机器上跑、
// 失败怎么传播，统一由 runMethodScripts 处理。
// hosts 要指向一台有 kubeconfig 的 master —— apply 是对集群做的，不是对每个节点做的，
// 写多台就是把同一份清单 apply 好几遍。
func installManifest(res types.Resource, artifacts []types.DownloadResult) ([]string, error) {
	paths := make([]string, 0, len(artifacts)+len(res.ExtraFiles))
	for _, art := range artifacts {
		paths = append(paths, art.LocalPath)
	}
	for rawPath := range res.ExtraFiles {
		dest, err := utils.ParseStr(rawPath, res)
		if err != nil {
			return nil, fmt.Errorf("附加文件路径模板解析失败 (%s): %w", rawPath, err)
		}
		paths = append(paths, dest)
	}
	// map 的遍历顺序是随机的，不排一下的话同一份配置两次执行的 apply 顺序不同；
	// 清单之间有依赖（先 CRD 再 CR）时表现为偶发失败。
	sort.Strings(paths)

	if len(paths) == 0 {
		return nil, fmt.Errorf("method: manifest 需要 urls: 或 extra_files: 指明要 apply 的清单")
	}

	cmds := make([]string, 0, len(paths))
	for _, p := range paths {
		cmds = append(cmds, kubectlApplyCmd(p))
	}
	return cmds, nil
}

// kubectlApplyCmd 生成一条 apply 命令。
//
// kubeconfig 的选择放在生成的 shell 里判断，而不是 Go 侧：apply 发生在目标节点上，
// 操作机上有没有 /etc/kubernetes/admin.conf 与那台机器无关。
// 刚 kubeadm init 出来的 master 上只有 admin.conf，$HOME/.kube/config 是之后才拷的，
// 两个都认才能在"init 完立刻装 CNI"这个时点上工作。
func kubectlApplyCmd(path string) string {
	return fmt.Sprintf(`if ! command -v kubectl >/dev/null 2>&1; then
  echo "节点上没有 kubectl，无法 apply 清单 %s" >&2
  exit 1
fi
if [ -z "$KUBECONFIG" ]; then
  if [ -f /etc/kubernetes/admin.conf ]; then
    export KUBECONFIG=/etc/kubernetes/admin.conf
  elif [ -f "$HOME/.kube/config" ]; then
    export KUBECONFIG="$HOME/.kube/config"
  else
    echo "找不到 kubeconfig（/etc/kubernetes/admin.conf 或 \$HOME/.kube/config），无法 apply 清单 %s" >&2
    exit 1
  fi
fi
kubectl apply -f %s`, path, path, shellQuote(path))
}
