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

// 主机配置
type HostsConfig struct {
	Nodes []RemoteNode `yaml:"nodes"`
}

type RemoteNode struct {
	Host    string `yaml:"host"`
	IP      string `yaml:"ip"`
	Role    string `yaml:"role"` // todo 抽取出来 master,harbor,work
	User    string `yaml:"user"`
	SSHKey  string `yaml:"sshKey"`
	IsLocal bool
}
