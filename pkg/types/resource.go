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

// 资源配置下载配置文件结构
type ResourceConfig struct {
	Proxy     string       `yaml:"proxy"` // 可选代理
	Resources []Resource   `yaml:"resources,omitempty"`
	Nodes     []RemoteNode `yaml:"nodes"`
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
	Method        string            `yaml:"method"`         // 安装方法
	ExtraFiles    map[string]string `yaml:"ExtraFiles"`     // 扩展文件
	Files         []string          `yaml:"files"`          //文件路径
}

// DownloadResult 下载结果
type DownloadResult struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	LocalPath string `json:"local_path"` // 相对路径
	Error     error  `json:"error,omitempty"`
}
