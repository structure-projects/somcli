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
package offline

// DownloadConfig 下载配置文件结构
type DownloadConfig struct {
	Proxy     string             `yaml:"proxy"` // 可选代理
	Resources []DownloadResource `yaml:"download,omitempty"`
}

// DownloadResource 单个资源定义
type DownloadResource struct {
	Name     string   `yaml:"name"`
	Version  string   `yaml:"version"`
	URLs     []string `yaml:"urls"`
	Target   string   `yaml:"target"`   // 相对缓存目录的路径
	Checksum string   `yaml:"checksum"` // 可选校验和
}

// DownloadResult 下载结果
type DownloadResult struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	LocalPath string `json:"local_path"` // 相对路径
	Error     error  `json:"error,omitempty"`
}
