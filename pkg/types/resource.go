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
package types

// ResourceConfig 是 somcli 的统一配置：一份文件描述全部场景，各命令只取自己那一段 ——
// install/download 取 resources，cluster 取 cluster，images 取 images，nodes 谁都可能用。
// 别人的段落原样放着不影响解析，缺自己那段才报错。
type ResourceConfig struct {
	// 全局设置。同名命令行标志优先于此处。
	Offline     bool              `yaml:"offline"`
	Debug       bool              `yaml:"debug"`
	GithubProxy string            `yaml:"github_proxy"`
	WorkDir     string            `yaml:"workdir"`
	Vars        map[string]string `yaml:"vars,omitempty"` // 自定义模板变量，模板里以 {{.Vars.xxx}} 访问
	Source      *SourceConfig     `yaml:"source,omitempty"`

	Resources []Resource    `yaml:"resources,omitempty"`
	Nodes     []RemoteNode  `yaml:"nodes"`
	Clusters  []ClusterSpec `yaml:"cluster,omitempty"`
	Images    []Image       `yaml:"images,omitempty"`
}

// SourceConfig 描述"装系统包前先把软件源配好"（换源）。
//
// 国内环境下不配源，apt/yum 装基础工具就可能超时或失败；离线环境则要挂 ISO 当本地源。
// 它是节点级前置动作：method: package 在目标节点上装包前会先按这里的配置换好源。
// 探测与执行都发生在**目标节点**上（与 pkg/utils/packagemanager.go 同一纪律），
// 所以操作机是什么发行版无关。
type SourceConfig struct {
	// Mode 三选一，留空按 official 处理：
	//   official —— 不改源，用系统自带默认源；
	//   aliyun   —— 换成阿里云镜像（apt/dnf/yum/zypper/apk）；
	//   iso      —— 挂载目标节点上的 ISO 作为本地源（yum/dnf 与 apt）。
	Mode string `yaml:"mode"`

	// ISO 是 mode=iso 时 ISO 在**目标节点**上的绝对路径。
	// ISO 体积大，应由文件资源或离线介质预先分发到目标，somcli 只负责挂载与配源。
	ISO string `yaml:"iso"`

	// Mount 是 ISO 的挂载点，留空取 /mnt/somcli-iso。
	Mount string `yaml:"mount"`
}

// Resource 单个资源定义
type Resource struct {
	Name          string            `yaml:"name"`
	Version       string            `yaml:"version"`
	ResType       string            `yaml:"res_type"` //资源类型 file 文件、tar 归档文件、rpm 、ded、exe 可执行文件、sh 脚本文件
	URLs          []string          `yaml:"urls"`
	Target        string            `yaml:"target"`   // 相对缓存目录的路径
	Checksum      string            `yaml:"checksum"` // 可选校验和
	Image         string            `yaml:"image"`
	Hosts         []string          `yaml:"hosts"`          //安装节点
	PreInstall    []string          `yaml:"pre_install"`    //检测脚本
	PostInstall   []string          `yaml:"post_install"`   // 安装脚本
	RemoveScripts []string          `yaml:"remove_scripts"` //卸载脚本
	Method        string            `yaml:"method"`         // 安装方法：script（默认）/ binary / package / container / source
	Package       string            `yaml:"package"`        // method: package 的包名，留空取 name
	InstallDir    string            `yaml:"install_dir"`    // method: binary / container 的落地目录，留空取 /usr/local/bin
	Build         []string          `yaml:"build"`          // method: source 的构建命令，在解压出的源码目录里执行
	ExtraFiles    map[string]string `yaml:"extra_files"`    // 附加文件：目标路径 -> 内容，两者都过模板
	Files         []string          `yaml:"files"`          // method: binary 时指定归档内要安装的文件，留空则安装归档里所有可执行文件
	Check         string            `yaml:"check"`          // 幂等探针：在目标上执行，退出 0 视为已安装并跳过整个资源
	OnError       string            `yaml:"on_error"`       // 失败策略：abort（默认，立即中止）/ continue（跳过失败目标与本资源，继续后续资源）
}

// DownloadResult 下载结果
type DownloadResult struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	LocalPath string `json:"local_path"` // 相对路径
	Error     error  `json:"error,omitempty"`
}
