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
package cluster

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
	"gopkg.in/yaml.v2"
)

// catalogEnv 指向一份替代 configs/k8s 的目录，用来在不重新编译的前提下改安装内容。
const catalogEnv = "SOMCLI_K8S_CATALOG"

// catalogFile 是目录里每个文件的结构。
//
// 单独定义而不是复用 types.ResourceConfig：那个类型还带 offline / debug / vars 这些
// 全局设置，而安装清单里写这些不会有任何效果。UnmarshalStrict 配上这个窄类型，
// 写错位置的键会当场报错，而不是静默失效。
type catalogFile struct {
	Resources []types.Resource `yaml:"resources"`
}

var (
	builtinCatalogFS   fs.FS
	builtinCatalogRoot string
)

// SetBuiltinK8sCatalog 注册编译进二进制的安装清单，由 main 调用。
// 见 main.go 里的说明：go:embed 只能发生在与 configs/ 同级的那个包里。
func SetBuiltinK8sCatalog(fsys fs.FS, root string) {
	builtinCatalogFS = fsys
	builtinCatalogRoot = root
}

// loadK8sCatalog 读出"能装的东西"，键是资源名。
//
// 目录优先级：SOMCLI_K8S_CATALOG 指定的目录 > 编译进二进制的那份。
// 只有这两个来源，不去猜当前目录：从仓库里跑和从 /usr/local/bin 跑装出来的东西
// 必须是同一份，否则"我这儿能装"就没有任何参考价值。
func loadK8sCatalog() (map[string]types.Resource, error) {
	fsys, root, origin, err := catalogSource()
	if err != nil {
		return nil, err
	}
	utils.PrintDebug("安装清单来自: %s", origin)

	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, fmt.Errorf("读取安装清单目录 %s 失败: %w", origin, err)
	}

	catalog := make(map[string]types.Resource, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		name := path.Join(root, entry.Name())
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("读取 %s 失败: %w", name, err)
		}

		var file catalogFile
		if err := yaml.UnmarshalStrict(data, &file); err != nil {
			return nil, fmt.Errorf("解析安装清单 %s 失败: %w", name, err)
		}
		for _, res := range file.Resources {
			if strings.TrimSpace(res.Name) == "" {
				return nil, fmt.Errorf("安装清单 %s 里有资源没写 name", name)
			}
			if _, dup := catalog[res.Name]; dup {
				// 重名的两条资源共用一个幂等状态键，先装的会被后装的覆盖掉，
				// 于是"这台机器装过没有"永远查不到正确答案。宁可装不了。
				return nil, fmt.Errorf("安装清单里资源名 %q 重复（%s）", res.Name, name)
			}
			catalog[res.Name] = res
		}
	}

	if len(catalog) == 0 {
		return nil, fmt.Errorf("安装清单目录 %s 里没有可用资源", origin)
	}
	return catalog, nil
}

// catalogSource 挑出这次用哪份清单，第三个返回值是给日志与报错用的来源描述。
func catalogSource() (fsys fs.FS, root, origin string, err error) {
	if dir := strings.TrimSpace(os.Getenv(catalogEnv)); dir != "" {
		dir = utils.ExpandPath(dir)
		info, statErr := os.Stat(dir)
		if statErr != nil {
			return nil, "", "", fmt.Errorf("%s 指向的 %s 不可用: %w", catalogEnv, dir, statErr)
		}
		if !info.IsDir() {
			return nil, "", "", fmt.Errorf("%s 指向的 %s 不是目录", catalogEnv, dir)
		}
		return os.DirFS(dir), ".", dir + "（" + catalogEnv + "）", nil
	}

	if builtinCatalogFS == nil {
		// 只有"没走 main 就调到这儿"才可能发生。说清楚是构建问题，
		// 免得看起来像用户少配了什么。
		return nil, "", "", fmt.Errorf("二进制里没有内置安装清单，可用 %s 指定一份 configs/k8s 目录", catalogEnv)
	}
	return builtinCatalogFS, builtinCatalogRoot, "内置 " + builtinCatalogRoot, nil
}

// catalogNames 列出清单里所有资源名，排序后给报错用 —— 名字写错时得能看到有哪些可选。
func catalogNames(catalog map[string]types.Resource) []string {
	names := make([]string, 0, len(catalog))
	for name := range catalog {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// resourceFromCatalog 按名字取出资源，并把集群配置里的版本与目标节点填进去。
//
// 清单文件里的 version: 只是兜底：装哪个版本是集群配置说了算的，
// 两处不一致时以集群配置为准，否则用户改了 containerdVersion 却装出别的版本。
func resourceFromCatalog(catalog map[string]types.Resource, name string, version string, hosts []string) (res types.Resource, err error) {
	res, ok := catalog[name]
	if !ok {
		return res, fmt.Errorf("安装清单里没有资源 %q，可用：%s",
			name, strings.Join(catalogNames(catalog), " / "))
	}
	if v := strings.TrimSpace(version); v != "" {
		res.Version = v
	}
	res.Hosts = hosts
	return res, nil
}

// k8sVersionFor 给出某个资源该用集群配置里的哪个版本键。
// 返回空串表示这条资源不由集群配置定版本（比如 base-dependencies 没有版本概念）。
func k8sVersionFor(name string, k8s types.K8sConfig) string {
	switch name {
	case "cni-plugins":
		return k8s.CniPluginsVersion
	case "runc":
		return k8s.RuncVersion
	case "containerd":
		return k8s.ContainerdVersion
	case "docker":
		return k8s.DockerVersion
	case "kubernetes":
		return k8s.Version
	case cniFlannel, cniCalico:
		return k8s.CniVersion
	default:
		return ""
	}
}

// alwaysApply 列出每次 cluster create 都必须重跑、不看幂等状态的资源。
//
// base-dependencies：swapoff 与 modprobe 的效果重启就没了，而状态库会说"装过了"
// 直接跳过，结果是重启后再装一次集群，kubeadm 在 preflight 阶段被 swap 挡住。
// docker：沿用外置前的行为（原来也是强制执行）。看着像是当年顺手写的，
// 但改不改是另一回事，不混在这次搬家里。
var alwaysApply = map[string]bool{
	"base-dependencies": true,
	"docker":            true,
}

// defaultK8sResources 是 k8sConfig.resources 留空时装的那一套，顺序即安装顺序。
//
// 网络插件不在里面：它由 k8sConfig.cni 选定，且必须等 kubeadm init 之后、
// worker 加入之前才能装（见 deployCNI）。
func defaultK8sResources(runtime string) []string {
	if strings.EqualFold(strings.TrimSpace(runtime), "docker") {
		return []string{"base-dependencies", "docker", "kubernetes"}
	}
	return []string{"base-dependencies", "cni-plugins", "runc", "containerd", "kubernetes"}
}

// k8sResourceNames 给出这套集群要装的资源名，配置里写了就按配置来。
func k8sResourceNames(config *types.ClusterConfig) []string {
	if names := config.Cluster.K8sConfig.Resources; len(names) > 0 {
		return names
	}
	return defaultK8sResources(config.Cluster.K8sConfig.ContainerRuntime)
}

// validateK8sResourceNames 在连节点之前确认要装的东西都在清单里。
//
// 一并把网络插件也查了：cni 的取值范围另有校验，但取值对、清单里却没有对应文件
// （比如用 SOMCLI_K8S_CATALOG 换了一份不完整的目录）时，失败会推迟到 kubeadm init 之后，
// 那时节点已经被改过一遍。
func validateK8sResourceNames(config *types.ClusterConfig) error {
	catalog, err := loadK8sCatalog()
	if err != nil {
		return err
	}

	names := k8sResourceNames(config)
	if cni := strings.ToLower(strings.TrimSpace(config.Cluster.K8sConfig.Cni)); cni != "" {
		names = append(names, cni)
	}

	for _, name := range names {
		if _, ok := catalog[name]; !ok {
			return fmt.Errorf("安装清单里没有资源 %q，可用：%s",
				name, strings.Join(catalogNames(catalog), " / "))
		}
	}
	return nil
}

// setK8sTemplateVars 把集群级的值交给模板，供 configs/k8s 里的清单引用。
//
// 每个键都要给（哪怕是空串）：模板渲染开了 missingkey=error，缺键会让整条命令报错，
// 而"没配 imageRepository"是完全正常的情形，得靠 shell 判空表达。
func setK8sTemplateVars(config *types.ClusterConfig) {
	k8s := config.Cluster.K8sConfig
	utils.SetComputedVars(map[string]string{
		"k8sVersion":           k8s.Version,
		"podNetworkCidr":       k8s.PodNetworkCidr,
		"serviceCidr":          k8s.ServiceCidr,
		"containerRuntime":     k8s.ContainerRuntime,
		"cni":                  k8s.Cni,
		"cniVersion":           k8s.CniVersion,
		"imageRepository":      k8s.ImageRepository,
		"pauseImageVersion":    k8s.PauseImageVersion,
		"controlPlaneEndpoint": k8s.ControlPlaneEndpoint,
	})
}
